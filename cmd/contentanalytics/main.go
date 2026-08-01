// Command contentanalytics is the Content Analytics & Performance CLI
// (Milestone 17). It collects performance metrics for published content across
// platforms (Dev.to, Medium, Hashnode, YouTube, …), stores immutable
// daily/weekly/monthly snapshots, and produces reports, dashboard datasets, and
// the structured OptimizationSignals bundle for Milestone 18.
//
// Providers are interchangeable: real API adapters read credentials from the
// environment (never logged), while --offline swaps in a deterministic synthetic
// provider so the whole pipeline runs without network or secrets.
//
// Usage:
//
//	# Offline demo: seed 30 days of synthetic history, print the report.
//	go run ./cmd/contentanalytics report --offline --days 30
//
//	# Dashboard dataset (JSON) and optimization signals for M18.
//	go run ./cmd/contentanalytics dashboard --offline
//	go run ./cmd/contentanalytics signals --offline --format json
//
//	# Real collection for today (requires env credentials + a publications file).
//	go run ./cmd/contentanalytics collect --publications pubs.json
//
// Environment (real mode, never logged): DEVTO_API_KEY, HASHNODE_TOKEN,
// MEDIUM_STATS_URL, YOUTUBE_API_KEY, YOUTUBE_ACCESS_TOKEN.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	ca "github.com/teddynted/ai-github-repository-blog-generator/internal/contentanalytics"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	mode, rest := args[0], args[1:]

	fs := flag.NewFlagSet("contentanalytics", flag.ContinueOnError)
	date := fs.String("date", "", "target date YYYY-MM-DD (default: today)")
	days := fs.Int("days", 30, "offline: days of synthetic history to seed")
	format := fs.String("format", "md", "output format: md | json")
	out := fs.String("out", "", "output file (default: stdout)")
	offline := fs.Bool("offline", false, "use the deterministic synthetic provider (no network/secrets)")
	pubsPath := fs.String("publications", "", "path to a publications JSON file (real mode)")
	timeout := fs.Duration("timeout", 2*time.Minute, "operation timeout")
	if err := fs.Parse(rest); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := ca.DefaultConfig()
	repo := ca.NewMemoryRepository()
	now := time.Now
	target := *date
	if target == "" {
		target = time.Now().UTC().Format("2006-01-02")
	}
	start := addDays(target, -(*days - 1))

	col := ca.NewCollector(cfg, repo, buildProviders(*offline, repo, start), now)
	col.Metrics = ca.LogMetricsPublisher{}

	// Load publications (real mode) or seed synthetic ones (offline).
	if *offline {
		seedSynthetic(repo, start)
	} else if *pubsPath != "" {
		if err := loadPublications(repo, *pubsPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: load publications: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(os.Stderr, "error: real mode needs --publications FILE (or use --offline)")
		return 2
	}

	// Collect history: offline backfills the range; real mode collects target day.
	if *offline {
		for d := start; d <= target; d = addDays(d, 1) {
			col.Collect(ctx, d)
		}
		ca.NewSnapshotter(repo, now).RollUpRange(start, target)
	} else {
		col.Collect(ctx, target)
		ca.NewSnapshotter(repo, now).RollUp(target)
	}

	switch mode {
	case "collect":
		res := col.Collect(ctx, target)
		return emit(*out, "json", res, "")

	case "rollup":
		n := ca.NewSnapshotter(repo, now).RollUpRange(start, target)
		return emit(*out, *format, map[string]int{"rollupsWritten": n}, fmt.Sprintf("rolled up %d weekly/monthly snapshots\n", n))

	case "report":
		r := ca.NewReportEngine(repo, cfg, now).Generate(target)
		return emit(*out, *format, r, r.Markdown())

	case "dashboard":
		ds := ca.NewDashboardEngine(repo, cfg, ca.LogMetricsPublisher{}, now).Dataset(target)
		return emit(*out, "json", ds, "")

	case "signals":
		sig := ca.NewOptimizationEngine(repo, cfg, now).Signals(target)
		return emit(*out, "json", sig, "")

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", mode)
		usage()
		return 2
	}
}

// buildProviders returns synthetic providers offline, else real env-credentialed
// adapters over a shared HTTP client.
func buildProviders(offline bool, repo ca.Repository, start ca.Date) []ca.AnalyticsProvider {
	if offline {
		return []ca.AnalyticsProvider{
			ca.NewSyntheticProvider(ca.PlatformDevTo, start, 1200, 90, false),
			ca.NewSyntheticProvider(ca.PlatformHashnode, start, 800, 60, false),
			ca.NewSyntheticProvider(ca.PlatformMedium, start, 600, 40, false),
			ca.NewSyntheticProvider(ca.PlatformYouTube, start, 5000, 300, true),
		}
	}
	hc := &http.Client{Timeout: 20 * time.Second}
	creds := ca.EnvCredentials()
	return []ca.AnalyticsProvider{
		ca.NewDevToProvider(hc, creds),
		ca.NewHashnodeProvider(hc, creds),
		ca.NewMediumProvider(hc, creds),
		ca.NewYouTubeProvider(hc, creds),
	}
}

// seedSynthetic registers a handful of demo publications per platform.
func seedSynthetic(repo ca.Repository, start ca.Date) {
	published := parseDateT(start)
	demos := []struct {
		id, cid, title string
		platform       ca.Platform
		ctype          ca.ContentType
		tags, kw       []string
	}{
		{"pub-dev-1", "c1", "Serverless on a Budget", ca.PlatformDevTo, ca.TypeBlog, []string{"aws", "serverless"}, []string{"lambda", "cost"}},
		{"pub-dev-2", "c2", "Clean Architecture in Go", ca.PlatformDevTo, ca.TypeBlog, []string{"golang", "architecture"}, []string{"solid", "ports"}},
		{"pub-hash-1", "c3", "EventBridge Patterns", ca.PlatformHashnode, ca.TypeBlog, []string{"aws", "eventbridge"}, []string{"events", "sqs"}},
		{"pub-med-1", "c4", "Shipping AI Content Pipelines", ca.PlatformMedium, ca.TypeArticle, []string{"ai", "devops"}, []string{"pipeline", "bedrock"}},
		{"pub-yt-1", "c5", "Build a Release Bot", ca.PlatformYouTube, ca.TypeYouTubeVideo, []string{"golang", "aws"}, []string{"automation", "ci"}},
		{"pub-yt-2", "c6", "60s: Local LLM Inference", ca.PlatformYouTube, ca.TypeYouTubeShorts, []string{"ai", "llm"}, []string{"local", "llm"}},
	}
	for i, d := range demos {
		_ = repo.SavePublication(ca.Publication{
			ID: d.id, ContentID: d.cid, Platform: d.platform, ContentType: d.ctype,
			Title: d.title, URL: "https://example.com/" + d.id, PlatformID: d.id,
			Tags: d.tags, Keywords: d.kw, Status: "published",
			PublishedAt: published.Add(time.Duration(i*6) * time.Hour),
		})
	}
}

func loadPublications(repo ca.Repository, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var pubs []ca.Publication
	if err := json.Unmarshal(raw, &pubs); err != nil {
		return err
	}
	for _, p := range pubs {
		if err := repo.SavePublication(p); err != nil {
			return err
		}
	}
	return nil
}

func emit(outPath, format string, v any, text string) int {
	var body string
	if format == "json" || text == "" {
		b, _ := json.MarshalIndent(v, "", "  ")
		body = string(b) + "\n"
	} else {
		body = text
	}
	if outPath == "" {
		fmt.Print(body)
		return 0
	}
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	return 0
}

func addDays(date string, n int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

func parseDateT(date string) time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Now()
	}
	return t
}

func usage() {
	fmt.Fprint(os.Stderr, `contentanalytics — Content Analytics & Performance (Milestone 17)

Usage:
  contentanalytics <command> [flags]

Commands:
  collect     Collect a day's performance metrics for tracked publications
  rollup      Derive weekly/monthly historical snapshots
  report      Generate the structured analytics report (md|json)
  dashboard   Emit the dashboard-ready dataset (json)
  signals     Emit the OptimizationSignals bundle for Milestone 18 (json)

Common flags:
  --offline            Use the deterministic synthetic provider (no network/secrets)
  --days N             Offline: synthetic history depth (default 30)
  --date D             Target date YYYY-MM-DD (default: today)
  --publications FILE  Publications JSON (real mode)
  --format md|json     Output format
  --out FILE           Write to a file instead of stdout
`)
}

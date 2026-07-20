// Command contentoptimizer is the AI Content Optimization CLI (Milestone 18). It
// consumes the analytics produced by Milestone 17, detects winning/losing content
// patterns, generates grounded recommendations, analyzes and versions prompt
// templates, detects trends, and produces a structured optimization report.
//
// It performs no optimization automatically: prompt changes are proposals that
// require human approval. The reasoning provider is deterministic by default and
// can be upgraded to Bedrock/Ollama with --ai.
//
// Usage:
//
//	# Offline demo: run the full M17→M18 pipeline on synthetic analytics.
//	go run ./cmd/contentoptimizer report --offline --days 30
//
//	# Emit the full report as JSON.
//	go run ./cmd/contentoptimizer report --offline --format json
//
//	# Grounded AI reasoning via Ollama (falls back to deterministic on error).
//	go run ./cmd/contentoptimizer report --offline --ai
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	ca "github.com/teddynted/ai-github-repository-blog-generator/internal/contentanalytics"
	co "github.com/teddynted/ai-github-repository-blog-generator/internal/contentoptimizer"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	mode, rest := args[0], args[1:]

	fs := flag.NewFlagSet("contentoptimizer", flag.ContinueOnError)
	date := fs.String("date", "", "as-of date YYYY-MM-DD (default: today)")
	days := fs.Int("days", 30, "offline: days of synthetic analytics history to seed")
	format := fs.String("format", "md", "output format: md | json")
	out := fs.String("out", "", "output file (default: stdout)")
	offline := fs.Bool("offline", false, "run on synthetic analytics (no network/secrets)")
	analyticsPath := fs.String("analytics", "", "path to an AnalyticsInput JSON file (real mode)")
	useAI := fs.Bool("ai", false, "use Ollama for grounded reasoning (falls back to deterministic)")
	ollamaURL := fs.String("ollama", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	model := fs.String("model", envOr("OLLAMA_MODEL", "qwen2.5:7b"), "Ollama model")
	timeout := fs.Duration("timeout", 2*time.Minute, "operation timeout")
	if err := fs.Parse(rest); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	target := *date
	if target == "" {
		target = time.Now().UTC().Format("2006-01-02")
	}
	now := time.Now

	input, err := buildInput(ctx, *offline, target, *days, *analyticsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	cfg := co.DefaultConfig()
	repo := co.NewMemoryRepository()

	// Register a baseline prompt template so prompt analysis has history.
	ps := co.NewPromptService(repo, now)
	_, _ = ps.Register("blog-generator", "Write a technical blog about {{release}} ...", "seed baseline")

	opt := co.NewOptimizer(cfg, repo, now)
	opt.Metrics = co.LogMetricsPublisher{}
	if *useAI {
		opt.WithReasoner(co.NewModelReasoner(ollama.New(*model, ollama.WithBaseURL(*ollamaURL)), "ollama"))
	}

	report, err := opt.Optimize(ctx, input, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: optimize: %v\n", err)
		return 1
	}

	switch mode {
	case "report":
		return emit(*out, *format, report, report.Markdown())
	case "recommendations":
		return emit(*out, "json", report.Recommendations, "")
	case "patterns":
		return emit(*out, "json", map[string]any{"winning": report.WinningPatterns, "losing": report.LosingPatterns}, "")
	case "trends":
		return emit(*out, "json", report.Trends, "")
	case "prompts":
		return emit(*out, "json", report.PromptAnalyses, "")
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", mode)
		usage()
		return 2
	}
}

// buildInput produces the grounded AnalyticsInput: offline runs the real M17
// analytics pipeline on synthetic data; real mode loads a serialized input.
func buildInput(ctx context.Context, offline bool, target string, days int, path string) (co.AnalyticsInput, error) {
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return co.AnalyticsInput{}, fmt.Errorf("read analytics: %w", err)
		}
		var in co.AnalyticsInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return co.AnalyticsInput{}, fmt.Errorf("parse analytics: %w", err)
		}
		return in, nil
	}
	if !offline {
		return co.AnalyticsInput{}, fmt.Errorf("real mode needs --analytics FILE (or use --offline)")
	}
	return syntheticInput(ctx, target, days), nil
}

// syntheticInput runs Milestone 17 end-to-end on synthetic providers, then adapts
// its output into the optimizer's AnalyticsInput — a real M17→M18 pipeline.
func syntheticInput(ctx context.Context, target string, days int) co.AnalyticsInput {
	start := addDays(target, -(days - 1))
	repo := ca.NewMemoryRepository()
	tagsByID := map[string][]string{}
	kwByID := map[string][]string{}
	demos := []struct {
		id, title string
		platform  ca.Platform
		ctype     ca.ContentType
		tags, kw  []string
		video     bool
	}{
		{"pub-dev-1", "Serverless on a Budget", ca.PlatformDevTo, ca.TypeBlog, []string{"aws", "serverless"}, []string{"lambda"}, false},
		{"pub-dev-2", "Clean Architecture in Go", ca.PlatformDevTo, ca.TypeBlog, []string{"golang", "architecture"}, []string{"solid"}, false},
		{"pub-hash-1", "EventBridge Patterns", ca.PlatformHashnode, ca.TypeBlog, []string{"aws", "eventbridge"}, []string{"events"}, false},
		{"pub-yt-1", "Build a Release Bot", ca.PlatformYouTube, ca.TypeYouTubeVideo, []string{"golang", "aws"}, []string{"automation"}, true},
		{"pub-yt-2", "60s: Ollama Local LLM", ca.PlatformYouTube, ca.TypeYouTubeShorts, []string{"ai", "ollama"}, []string{"llm"}, true},
	}
	var provs []ca.AnalyticsProvider
	seen := map[ca.Platform]bool{}
	published := parseDateT(start)
	for i, d := range demos {
		_ = repo.SavePublication(ca.Publication{
			ID: d.id, ContentID: d.id, Platform: d.platform, ContentType: d.ctype,
			Title: d.title, PlatformID: d.id, Tags: d.tags, Keywords: d.kw, Status: "published",
			PublishedAt: published.Add(time.Duration(i*7) * time.Hour),
		})
		tagsByID[d.id] = d.tags
		kwByID[d.id] = d.kw
		if !seen[d.platform] {
			seen[d.platform] = true
			base := int64(1500 + i*400)
			provs = append(provs, ca.NewSyntheticProvider(d.platform, start, base, int64(60+i*30), d.video))
		}
	}
	cfg := ca.DefaultConfig()
	cfg.PublishToCloud = false
	col := ca.NewCollector(cfg, repo, provs, func() time.Time { return parseDateT(start) })
	for dd := start; dd <= target; dd = addDays(dd, 1) {
		col.Collect(ctx, dd)
	}
	report := ca.NewReportEngine(repo, cfg, time.Now).Generate(target)
	signals := ca.NewOptimizationEngine(repo, cfg, time.Now).Signals(target)

	// Build a views series for trend analysis from the dashboard dataset.
	ds := ca.NewDashboardEngine(repo, cfg, nil, time.Now).Dataset(target)
	series := map[string][]co.SeriesPoint{}
	for _, s := range ds.Series {
		if s.Name == "TotalViews" {
			for _, p := range s.Points {
				series["views"] = append(series["views"], co.SeriesPoint{Date: p.Date, Value: p.Value})
			}
		}
	}

	in := co.FromContentAnalytics(report, signals, series)
	// Ground per-record topics from real publication metadata so topic patterns
	// and topic trends work end-to-end.
	enrichPublishedAt(&in, published)
	in.EnrichTags(tagsByID, kwByID)
	return in
}

// enrichPublishedAt spreads publish timestamps across the records for trend
// bucketing in the demo (the M17 report omits per-record timestamps).
func enrichPublishedAt(in *co.AnalyticsInput, base time.Time) {
	for i := range in.Records {
		in.Records[i].PublishedAt = base.Add(time.Duration(i*36) * time.Hour)
	}
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Fprint(os.Stderr, `contentoptimizer — AI Content Optimization (Milestone 18)

Usage:
  contentoptimizer <command> [flags]

Commands:
  report           Full optimization report (md|json)
  recommendations  Grounded recommendations (json)
  patterns         Winning/losing patterns (json)
  trends           Multi-horizon trend analysis (json)
  prompts          Prompt version analysis + proposals (json)

Common flags:
  --offline          Run the real M17 analytics pipeline on synthetic data
  --days N           Offline: synthetic analytics history depth (default 30)
  --date D           As-of date YYYY-MM-DD (default: today)
  --analytics FILE   AnalyticsInput JSON (real mode)
  --ai               Use Ollama for grounded reasoning (falls back to deterministic)
  --format md|json   Output format
  --out FILE         Write to a file instead of stdout
`)
}

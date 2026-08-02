// Command socialintel is the Social Media Intelligence platform CLI (Milestone
// 16). It collects daily performance data across YouTube, Instagram, X, and
// TikTok, stores immutable historical snapshots, and generates grounded
// analytics — a morning briefing and a Business Advisory Council package.
//
// Providers are interchangeable: real API adapters read credentials from the
// environment (never logged), while --offline swaps in a deterministic
// synthetic provider so the whole pipeline runs without network or secrets.
//
// Usage:
//
//	# Fully offline demo: seed 30 days of synthetic history, print the briefing.
//	go run ./cmd/socialintel briefing --offline --days 30
//
//	# Collect today from the real platform APIs (needs env credentials).
//	go run ./cmd/socialintel collect
//
//	# Backfill a date range, then emit the advisory package as JSON.
//	go run ./cmd/socialintel advisory --offline --from 2026-06-01 --to 2026-06-30 --format json
//
// Environment (real mode, never logged): YOUTUBE_ACCESS_TOKEN,
// INSTAGRAM_ACCESS_TOKEN, INSTAGRAM_USER_ID, X_BEARER_TOKEN, X_USER_ID,
// TIKTOK_ACCESS_TOKEN. Optional AI enrichment via the Anthropic API (ANTHROPIC_API_KEY).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/localgen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	si "github.com/teddynted/ai-github-repository-blog-generator/internal/socialintel"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	mode, rest := args[0], args[1:]

	fs := flag.NewFlagSet("socialintel", flag.ContinueOnError)
	date := fs.String("date", "", "target date YYYY-MM-DD (default: today)")
	from := fs.String("from", "", "backfill start date YYYY-MM-DD")
	to := fs.String("to", "", "backfill end date YYYY-MM-DD")
	days := fs.Int("days", 30, "offline: days of synthetic history to seed")
	format := fs.String("format", "md", "output format: md | json")
	out := fs.String("out", "", "output file (default: stdout)")
	offline := fs.Bool("offline", false, "use the deterministic synthetic provider (no network/secrets)")
	useAI := fs.Bool("ai", false, "enrich insights via the Anthropic API (grounded; falls back to deterministic)")
	model := fs.String("model", "", "model id (Anthropic; default when empty)")
	timeout := fs.Duration("timeout", 2*time.Minute, "operation timeout")
	if err := fs.Parse(rest); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg := si.DefaultConfig()
	repo := si.NewMemoryRepository()
	now := func() time.Time { return time.Now() }
	target := *date
	if target == "" {
		target = time.Now().UTC().Format("2006-01-02")
	}

	col := si.NewCollector(cfg, repo, buildProviders(*offline, target, *days), now)
	col.Monitor = si.LogMonitor{}

	// Seed history: offline backfills a synthetic range so analytics has depth;
	// real mode collects the requested day(s).
	seedStart, seedEnd := *from, *to
	if *offline && seedStart == "" {
		seedStart = addDaysStr(target, -(*days - 1))
		seedEnd = target
	}

	switch mode {
	case "collect":
		res := col.Collect(ctx, target)
		return emit(*out, *format, res, collectText(res))

	case "backfill":
		if seedStart == "" || seedEnd == "" {
			fmt.Fprintln(os.Stderr, "error: backfill needs --from and --to (or --offline)")
			return 2
		}
		res := col.Backfill(ctx, seedStart, seedEnd)
		return emit(*out, *format, res, fmt.Sprintf("backfilled %s..%s (%d days)\n", seedStart, seedEnd, len(res)))

	case "briefing", "advisory":
		// Both reporting modes need history — seed it first.
		if seedStart != "" && seedEnd != "" {
			col.Backfill(ctx, seedStart, seedEnd)
		} else {
			col.Collect(ctx, target)
		}
		var ai si.AIInsighter
		if *useAI {
			ai = si.NewModelInsighter(newModel(*model), "anthropic")
		}
		eng := si.NewBriefingEngine(cfg, repo, ai, now)
		if mode == "advisory" {
			pkg := eng.Advisory(ctx, target)
			return emit(*out, "json", pkg, "") // advisory is JSON by contract
		}
		b := eng.Generate(ctx, target)
		return emit(*out, *format, b, b.Markdown())

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", mode)
		usage()
		return 2
	}
}

// buildProviders returns the synthetic provider offline, else the real
// env-credentialed adapters over a shared HTTP client.
func buildProviders(offline bool, date si.Date, days int) []si.Provider {
	if offline {
		start := addDaysStr(date, -(days - 1))
		return []si.Provider{
			si.NewSyntheticProvider(si.PlatformYouTube, start, 42000, 120, 4),
			si.NewSyntheticProvider(si.PlatformInstagram, start, 18000, 60, 3),
			si.NewSyntheticProvider(si.PlatformX, start, 9500, 25, 5),
			si.NewSyntheticProvider(si.PlatformTikTok, start, 31000, 200, 4),
		}
	}
	hc := &http.Client{Timeout: 20 * time.Second}
	creds := si.EnvCredentials()
	return []si.Provider{
		si.NewYouTubeProvider(hc, creds, nil),
		si.NewInstagramProvider(hc, creds, nil),
		si.NewXProvider(hc, creds, nil),
		si.NewTikTokProvider(hc, creds),
	}
}

func newModel(model string) releasegen.Model {
	return localgen.Default(model)
}

func collectText(res si.CollectResult) string {
	var s strings.Builder
	fmt.Fprintf(&s, "collected for %s:\n", res.Date)
	for p, r := range res.Platforms {
		switch {
		case r.Skipped:
			fmt.Fprintf(&s, "  %-10s skipped (already stored)\n", p)
		case r.Error != "":
			fmt.Fprintf(&s, "  %-10s error: %s (retries %d)\n", p, r.Error, r.Retries)
		default:
			fmt.Fprintf(&s, "  %-10s ok (%d content, retries %d)\n", p, r.Content, r.Retries)
		}
	}
	return s.String()
}

// emit writes JSON or the provided text form, to a file or stdout.
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

func addDaysStr(date si.Date, n int) si.Date {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

func usage() {
	fmt.Fprint(os.Stderr, `socialintel — Social Media Intelligence (Milestone 16)

Usage:
  socialintel <command> [flags]

Commands:
  collect    Collect a single day's snapshots across all platforms
  backfill   Collect an inclusive date range (--from/--to)
  briefing   Generate the daily morning briefing (md|json)
  advisory   Generate the Business Advisory Council package (json)

Common flags:
  --offline        Use the deterministic synthetic provider (no network/secrets)
  --days N         Offline: synthetic history depth (default 30)
  --date D         Target date YYYY-MM-DD (default: today)
  --from D --to D  Backfill range
  --ai             Enrich insights via the Anthropic API (grounded)
  --format md|json Output format
  --out FILE       Write to a file instead of stdout
`)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

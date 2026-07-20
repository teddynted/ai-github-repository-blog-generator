// Command distribute publishes APPROVED content to multiple platforms
// (Milestone 15). By default it runs in --dry-run mode (no network, no
// credentials) so the multi-platform flow can be demonstrated safely; with
// --dry-run=false it uses the real publisher adapters with credentials from the
// environment. It never publishes content that is not marked approved.
//
// Usage:
//
//	# Dry-run distribution of an approved blog to Dev.to + Medium + GitHub:
//	go run ./cmd/distribute --content post.md --type "Technical Blog" --approval gov:blog-1 --platforms devto,medium,github
//
//	# Real distribution (needs DEVTO_API_KEY etc. in the environment):
//	go run ./cmd/distribute --content post.md --platforms devto --dry-run=false
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

	"github.com/teddynted/ai-github-repository-blog-generator/internal/publishing"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("distribute", flag.ContinueOnError)
	contentPath := fs.String("content", "", "path to the approved content file (required)")
	typ := fs.String("type", "Technical Blog", "content type")
	title := fs.String("title", "", "content title (default: derived from the file)")
	tagsCSV := fs.String("tags", "", "comma-separated tags")
	canonical := fs.String("canonical", "", "canonical URL")
	cover := fs.String("cover", "", "cover image URL")
	approval := fs.String("approval", "", "approval reference (required — content must be approved)")
	platformsCSV := fs.String("platforms", "devto,medium,hashnode,github", "comma-separated target platforms")
	ghOwner := fs.String("gh-owner", "", "GitHub owner (for the github publisher)")
	ghRepo := fs.String("gh-repo", "", "GitHub repository (for the github publisher)")
	dryRun := fs.Bool("dry-run", true, "do not call real APIs (safe demo)")
	delay := fs.Duration("delay", 0, "delay publication by this duration (scheduled)")
	format := fs.String("format", "md", "output format: md | json")
	outPath := fs.String("out", "", "output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *contentPath == "" {
		fmt.Fprintln(os.Stderr, "error: --content is required")
		return 2
	}
	if *approval == "" {
		fmt.Fprintln(os.Stderr, "error: --approval is required (content must have passed review & approval)")
		return 2
	}

	body, err := os.ReadFile(*contentPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read content: %v\n", err)
		return 1
	}

	content := publishing.Content{
		ID:           contentID(*contentPath),
		Type:         publishing.ContentType(*typ),
		Title:        firstNonEmpty(*title, deriveTitle(string(body))),
		Body:         string(body),
		Tags:         splitCSV(*tagsCSV),
		CanonicalURL: *canonical,
		CoverImage:   *cover,
		Approved:     true, // the caller asserts approval via --approval
		ApprovalRef:  *approval,
		Metadata:     map[string]string{},
	}

	platforms := parsePlatforms(*platformsCSV)
	engine := buildEngine(*dryRun, platforms, *ghOwner, *ghRepo)

	sched := publishing.Schedule{Mode: publishing.ScheduleImmediate}
	if *delay > 0 {
		sched = publishing.Schedule{Mode: publishing.ScheduleDelay, Delay: *delay}
	}

	pubs, err := engine.Distribute(context.Background(), content, publishing.DistributeRequest{Targets: platforms, Schedule: sched})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: distribute: %v\n", err)
		return 1
	}

	var out string
	if *format == "json" {
		b, _ := json.MarshalIndent(pubs, "", "  ")
		out = string(b)
	} else {
		out = publishing.Report(pubs)
	}
	if err := writeOut(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}

	failed := 0
	for _, p := range pubs {
		if p.Status == publishing.StatusFailed {
			failed++
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", p.Platform, p.Status)
	}
	if failed > 0 {
		return 3
	}
	return 0
}

// buildEngine wires the engine with either dry-run or real publisher adapters.
func buildEngine(dryRun bool, platforms []publishing.Platform, ghOwner, ghRepo string) *publishing.Engine {
	cfg := publishing.DefaultConfig()
	var pubs []publishing.Publisher
	if dryRun {
		for _, p := range platforms {
			pubs = append(pubs, publishing.NewDryRunPublisher(p))
		}
	} else {
		httpClient := &http.Client{Timeout: 30 * time.Second}
		creds := publishing.EnvCredentials()
		for _, p := range platforms {
			switch p {
			case publishing.PlatformDevTo:
				pubs = append(pubs, publishing.NewDevToPublisher(httpClient, creds))
			case publishing.PlatformMedium:
				pubs = append(pubs, publishing.NewMediumPublisher(httpClient, creds))
			case publishing.PlatformHashnode:
				pubs = append(pubs, publishing.NewHashnodePublisher(httpClient, creds))
			case publishing.PlatformYouTube:
				pubs = append(pubs, publishing.NewYouTubePublisher(httpClient, creds))
			case publishing.PlatformGitHub:
				pubs = append(pubs, publishing.NewGitHubPublisher(httpClient, creds, ghOwner, ghRepo))
			}
		}
	}
	e := publishing.NewEngine(cfg, publishing.NewMemoryRepository(), pubs, time.Now)
	e.Notifier = publishing.LogNotifier{}
	return e
}

func parsePlatforms(csv string) []publishing.Platform {
	var out []publishing.Platform
	for _, s := range splitCSV(csv) {
		switch strings.ToLower(s) {
		case "devto", "dev.to":
			out = append(out, publishing.PlatformDevTo)
		case "medium":
			out = append(out, publishing.PlatformMedium)
		case "hashnode":
			out = append(out, publishing.PlatformHashnode)
		case "youtube":
			out = append(out, publishing.PlatformYouTube)
		case "github":
			out = append(out, publishing.PlatformGitHub)
		}
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func deriveTitle(body string) string {
	for _, ln := range strings.Split(body, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "title:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "title:")), "\"'")
		}
		if strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(t, "# "))
		}
	}
	return ""
}

func contentID(path string) string {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".md")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func writeOut(path, content string) error {
	if path == "" {
		_, err := os.Stdout.WriteString(content)
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

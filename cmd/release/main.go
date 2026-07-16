// Command release is the semantic-versioning release CLI. It determines the
// next version from Conventional Commits, generates release notes and a
// CHANGELOG section, creates and pushes an annotated Git tag, and publishes a
// GitHub Release.
//
// Usage:
//
//	release [major|minor|patch]   cut a release (bump auto-derived if omitted)
//	release validate              run all pre-release checks (no changes)
//	release version               print the current and next version
//	release notes                 print the release notes for the next version
//	release changelog             print the CHANGELOG with the next release added
//
// Flags: --dry-run, --pre <id>, --config <path>, --repo <owner/name>,
//
//	--changelog <path>, --no-verify.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/changelog"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/release"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "compute everything but make no changes")
	pre := fs.String("pre", "", "pre-release identifier (e.g. rc.1); uses config default if just --pre")
	usePre := fs.Bool("prerelease", false, "make a pre-release using the configured identifier")
	cfgPath := fs.String("config", ".release.json", "path to the release config file")
	repoFlag := fs.String("repo", "", "GitHub repo as owner/name (auto-detected from origin if unset)")
	changelogPath := fs.String("changelog", "CHANGELOG.md", "path to the CHANGELOG file")
	noVerify := fs.Bool("no-verify", false, "skip pre-release validation (not recommended)")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return 2
	}

	sub := "release"
	rest := fs.Args()
	if len(rest) > 0 {
		sub = rest[0]
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	cfg, err := release.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return 1
	}

	owner, repo := detectRepo(*repoFlag)
	token := firstNonEmpty(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))

	svc := &release.Service{
		Git:    release.NewExecGit(""),
		GitHub: &release.GitHub{API: github.New(), Owner: owner, Repo: repo, Token: token},
		Config: cfg,
		Log:    logger,
	}

	// Determine the bump / pre-release identifier from the subcommand + flags.
	var forceBump *conventional.Bump
	switch sub {
	case "major":
		b := conventional.BumpMajor
		forceBump = &b
	case "minor":
		b := conventional.BumpMinor
		forceBump = &b
	case "patch":
		b := conventional.BumpPatch
		forceBump = &b
	case "release", "validate", "version", "notes", "changelog":
		// no forced bump
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n%s", sub, usage)
		return 2
	}
	preID := *pre
	if *usePre && preID == "" {
		preID = cfg.PreReleaseID
	}

	plan, err := svc.DeterminePlan(ctx, forceBump, preID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	switch sub {
	case "version":
		fmt.Printf("current: %s\nnext:    %s (%s)\n", tagOrNone(plan.PrevTag), plan.Tag, plan.Bump)
		return 0
	case "notes":
		fmt.Print(plan.Notes)
		return 0
	case "changelog":
		old, _ := os.ReadFile(*changelogPath)
		fmt.Print(changelog.Update(string(old), plan.NextVersion.String(), plan.ChangelogSection))
		return 0
	case "validate":
		return reportValidation(svc, ctx, plan, *changelogPath)
	}

	// A release (major/minor/patch/auto).
	fmt.Printf("→ %s release: %s → %s\n", plan.Bump, tagOrNone(plan.PrevTag), plan.Tag)
	if len(plan.Commits) == 0 && *dryRun {
		fmt.Println("  (no conventional commits since the last tag)")
	}

	if !*noVerify {
		old, _ := os.ReadFile(*changelogPath)
		if errs := svc.Validate(ctx, plan, string(old)); len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "✗ validation failed:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  - %v\n", e)
			}
			return 1
		}
		fmt.Println("✓ validation passed")
	}

	old, _ := os.ReadFile(*changelogPath)
	sum, err := svc.Apply(ctx, plan, string(old), *dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ release failed: %v\n", err)
		return 1
	}

	if *dryRun {
		fmt.Printf("✓ dry run complete — would release %s\n\n--- CHANGELOG (preview) ---\n%s\n--- release notes (preview) ---\n%s",
			sum.Tag, plan.ChangelogSection, plan.Notes)
		return 0
	}

	if err := os.WriteFile(*changelogPath, []byte(sum.Changelog), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: release published but writing %s failed: %v\n", *changelogPath, err)
	}
	fmt.Printf("✓ released %s\n", sum.Tag)
	if sum.ReleaseURL != "" {
		fmt.Printf("  %s\n", sum.ReleaseURL)
	}
	fmt.Printf("  remember to commit the updated %s\n", *changelogPath)
	return 0
}

func reportValidation(svc *release.Service, ctx context.Context, plan release.Plan, changelogPath string) int {
	old, _ := os.ReadFile(changelogPath)
	errs := svc.Validate(ctx, plan, string(old))
	if len(errs) == 0 {
		fmt.Printf("✓ ready to release %s\n", plan.Tag)
		return 0
	}
	fmt.Fprintf(os.Stderr, "✗ %d validation error(s):\n", len(errs))
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  - %v\n", e)
	}
	return 1
}

var originRE = regexp.MustCompile(`github\.com[:/]([^/]+)/([^/.]+)`)

// detectRepo resolves owner/name from the flag or the git origin remote.
func detectRepo(flagVal string) (owner, repo string) {
	if flagVal != "" {
		if o, r, ok := splitRepo(flagVal); ok {
			return o, r
		}
	}
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err == nil {
		if m := originRE.FindStringSubmatch(strings.TrimSpace(string(out))); m != nil {
			return m[1], m[2]
		}
	}
	return "", ""
}

func splitRepo(s string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSuffix(s, ".git"), "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func tagOrNone(tag string) string {
	if tag == "" {
		return "(none)"
	}
	return tag
}

const usage = `release — semantic-versioning release management

Commands:
  release [major|minor|patch]  cut a release (bump auto-derived if omitted)
  release validate             run all pre-release checks; no changes
  release version              print the current and next version
  release notes                print release notes for the next version
  release changelog            print the CHANGELOG with the next release added

Flags:
  --dry-run          compute everything, make no changes
  --pre <id>         pre-release identifier (e.g. rc.1)
  --prerelease       pre-release using the configured identifier
  --config <path>    release config file (default .release.json)
  --repo <owner/nm>  GitHub repo (auto-detected from origin if unset)
  --changelog <path> CHANGELOG path (default CHANGELOG.md)
  --no-verify        skip validation (not recommended)

Auth: set GITHUB_TOKEN or GH_TOKEN for GitHub Release creation.
`

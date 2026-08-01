package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// playground is an interactive loop for rapid prompt experimentation: pick a
// provider, an artifact, and a Release Context fixture, then generate — no code
// changes, no flags to remember. It reuses the same execute() path as the flag
// interface, so behaviour matches the non-interactive CLI exactly.
func playground(args []string) int {
	fixturesDir := "fixtures"
	if len(args) > 0 {
		fixturesDir = args[0]
	}
	in := bufio.NewReader(os.Stdin)

	fmt.Println("Content Prompt Playground — Ctrl-C to exit")
	fmt.Println()

	provider := choose(in, "Provider", []string{"claude-code", "ollama", "anthropic", "bedrock"})
	art := choose(in, "Artifact", append([]string{"all"}, supportedArtifacts...))
	fixture := choose(in, "Release Context fixture", listFixtures(fixturesDir))
	if provider == "" || art == "" || fixture == "" {
		fmt.Fprintln(os.Stderr, "playground: nothing selected")
		return 2
	}

	o := options{
		artifact: art, provider: provider, ctxPath: filepath.Join(fixturesDir, fixture),
		outDir: "output", cacheDir: ".cache",
		region: envOr("AWS_REGION", "us-east-1"), verbose: true, timeout: 15 * time.Minute,
	}

	rctx, err := loadContext(o.ctxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("\nGenerating %s with %s from %s …\n\n", art, provider, fixture)
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	return execute(ctx, o, rctx)
}

// choose prints a numbered menu and returns the selected value (or "" on EOF).
func choose(in *bufio.Reader, label string, opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	fmt.Printf("%s:\n", label)
	for i, o := range opts {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	for {
		fmt.Print("> ")
		line, err := in.ReadString('\n')
		if err != nil && line == "" {
			return ""
		}
		line = strings.TrimSpace(line)
		// Accept either the number or the literal value.
		for i, o := range opts {
			if line == o || line == fmt.Sprintf("%d", i+1) {
				fmt.Println()
				return o
			}
		}
		fmt.Println("  (invalid choice, try again)")
	}
}

// listFixtures returns the *.json fixtures in dir, sorted.
func listFixtures(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

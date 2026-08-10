// Command buildcontext builds a Release Context fixture for a repo release from
// GitHub (local dev tool for the local-generate→S3→render workflow).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasesource"
)

func main() {
	owner := flag.String("owner", "teddynted", "repo owner")
	repo := flag.String("repo", "designing-an-ai-agent-platform-on-aws", "repo name")
	tag := flag.String("tag", "", "release tag (required)")
	out := flag.String("out", "", "output fixture path (required)")
	flag.Parse()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" || *tag == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "need GITHUB_TOKEN env and -tag -out")
		os.Exit(2)
	}
	b := &rc.Builder{Sources: releasesource.New(github.New(), token)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	built, err := b.Build(ctx, rc.Request{Owner: *owner, Repository: *repo, ReleaseTag: *tag})
	if err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(built, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, len(data))
}

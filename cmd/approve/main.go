// Command approve is the approvals dashboard for content held for human review
// (when REQUIRE_HUMAN_APPROVAL is set, the worker stashes generated content
// under PENDING_DIR instead of publishing). With no flags it lists the pending
// queue; -all approves (publishes) everything; -reject-all discards everything.
//
//	approve            # list pending content
//	approve -all       # auto-approve: publish all pending content
//	approve -reject-all
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/approval"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
)

func main() {
	approveAll := flag.Bool("all", false, "approve (publish) all pending content")
	rejectAll := flag.Bool("reject-all", false, "discard all pending content")
	flag.Parse()

	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	ctx := context.Background()
	store := &approval.PendingStore{Dir: a.Config.PendingDir}

	switch {
	case *approveAll:
		n, err := store.ApproveAll(ctx, destination(ctx, a))
		if err != nil {
			log.Fatalf("approve: %v", err)
		}
		fmt.Printf("approved %d pending package(s)\n", n)
	case *rejectAll:
		items, err := store.List(ctx)
		if err != nil {
			log.Fatalf("list: %v", err)
		}
		for _, p := range items {
			if err := store.Reject(ctx, p); err != nil {
				log.Fatalf("reject %s: %v", p.Repo, err)
			}
		}
		fmt.Printf("rejected %d pending package(s)\n", len(items))
	default:
		items, err := store.List(ctx)
		if err != nil {
			log.Fatalf("list: %v", err)
		}
		if len(items) == 0 {
			fmt.Println("no pending content awaiting approval")
			return
		}
		for _, p := range items {
			fmt.Printf("%-30s %s  (%d assets: %s)\n", p.Repo, p.Date, len(p.Assets), kindList(p))
		}
	}
}

// destination builds the same publisher the worker uses.
func destination(ctx context.Context, a *app.App) approval.Publisher {
	if a.Config.OutputS3Bucket != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
		if err != nil {
			log.Fatalf("aws config: %v", err)
		}
		return publish.NewS3(s3.NewFromConfig(awsCfg), a.Config.OutputS3Bucket, a.Config.OutputS3Prefix, a.Logger)
	}
	return &publish.FilePublisher{Dir: a.Config.OutputDir, Logger: a.Logger}
}

func kindList(p approval.Pending) string {
	kinds := make([]string, 0, len(p.Assets))
	for _, asset := range p.Assets {
		kinds = append(kinds, string(asset.Kind))
	}
	return strings.Join(kinds, ", ")
}

// Command content-admin is the operator tool for the versioned generated-content
// bucket. It lists an artifact's version history, compares two versions, rolls a
// single artifact back to a prior version, and (re-)promotes a release into
// latest/. It reads and copies objects, so run it with an operator identity
// (see the operator role in infrastructure/bootstrap.yaml) — the worker stays
// write-only.
//
// Usage:
//
//	content-admin history        --repo owner/name --release v0.3.0 --artifact blog.md
//	content-admin compare        --repo owner/name --release v0.3.0 --artifact blog.md --a <ver> --b <ver>
//	content-admin rollback       --repo owner/name --release v0.3.0 --artifact blog.md --to <ver>
//	content-admin promote-latest --repo owner/name --release v0.3.0
//
// The bucket comes from --bucket or $CONTENT_BUCKET; the prefix defaults to
// "generated-content".
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentadmin"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "history":
		return cmdHistory(rest)
	case "compare":
		return cmdCompare(rest)
	case "rollback":
		return cmdRollback(rest)
	case "promote-latest":
		return cmdPromoteLatest(rest)
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", sub)
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `content-admin — operator tool for the generated-content bucket

Commands:
  history        --repo owner/name --release <tag> --artifact <file>
  compare        --repo owner/name --release <tag> --artifact <file> --a <ver> --b <ver>
  rollback       --repo owner/name --release <tag> --artifact <file> --to <ver>
  promote-latest --repo owner/name --release <tag>

Common flags: --bucket (or $CONTENT_BUCKET), --prefix (default generated-content), --region
`)
}

// common flags shared by every subcommand.
type common struct {
	bucket, prefix, region, repo, release string
}

func addCommon(fs *flag.FlagSet, c *common) {
	fs.StringVar(&c.bucket, "bucket", os.Getenv("CONTENT_BUCKET"), "content bucket name")
	fs.StringVar(&c.prefix, "prefix", "generated-content", "key prefix")
	fs.StringVar(&c.region, "region", os.Getenv("AWS_REGION"), "AWS region")
	fs.StringVar(&c.repo, "repo", "", "repository (owner/name)")
	fs.StringVar(&c.release, "release", "", "release tag (e.g. v0.3.0)")
}

func (c common) validate() error {
	if c.bucket == "" {
		return fmt.Errorf("--bucket (or $CONTENT_BUCKET) is required")
	}
	if c.repo == "" || c.release == "" {
		return fmt.Errorf("--repo and --release are required")
	}
	return nil
}

func newAdmin(ctx context.Context, c common) (*contentadmin.Admin, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if c.region != "" {
		opts = append(opts, awsconfig.WithRegion(c.region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &contentadmin.Admin{API: s3.NewFromConfig(cfg), Bucket: c.bucket, Prefix: c.prefix}, nil
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func cmdHistory(args []string) int {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	var c common
	addCommon(fs, &c)
	artifact := fs.String("artifact", "", "artifact filename (e.g. blog.md)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := c.validate(); err != nil {
		return fail(err)
	}
	if *artifact == "" {
		return fail(fmt.Errorf("--artifact is required"))
	}
	ctx := context.Background()
	admin, err := newAdmin(ctx, c)
	if err != nil {
		return fail(err)
	}
	versions, err := admin.History(ctx, c.repo, c.release, *artifact)
	if err != nil {
		return fail(err)
	}
	if len(versions) == 0 {
		fmt.Printf("no versions found for %s/%s %s\n", c.repo, c.release, *artifact)
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tLAST MODIFIED\tSIZE\tCURRENT")
	for _, v := range versions {
		cur := ""
		if v.IsLatest {
			cur = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", v.VersionID, v.LastModified.Format("2006-01-02 15:04:05"), v.Size, cur)
	}
	w.Flush()
	return 0
}

func cmdCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	var c common
	addCommon(fs, &c)
	artifact := fs.String("artifact", "", "artifact filename")
	va := fs.String("a", "", "version A (blank = current)")
	vb := fs.String("b", "", "version B (blank = current)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := c.validate(); err != nil {
		return fail(err)
	}
	if *artifact == "" {
		return fail(fmt.Errorf("--artifact is required"))
	}
	ctx := context.Background()
	admin, err := newAdmin(ctx, c)
	if err != nil {
		return fail(err)
	}
	diff, err := admin.Compare(ctx, c.repo, c.release, *artifact, *va, *vb)
	if err != nil {
		return fail(err)
	}
	if diff == "" {
		fmt.Println("(identical)")
		return 0
	}
	fmt.Print(diff)
	return 0
}

func cmdRollback(args []string) int {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	var c common
	addCommon(fs, &c)
	artifact := fs.String("artifact", "", "artifact filename")
	to := fs.String("to", "", "version id to roll back to")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := c.validate(); err != nil {
		return fail(err)
	}
	if *artifact == "" || *to == "" {
		return fail(fmt.Errorf("--artifact and --to are required"))
	}
	ctx := context.Background()
	admin, err := newAdmin(ctx, c)
	if err != nil {
		return fail(err)
	}
	newVer, err := admin.Rollback(ctx, c.repo, c.release, *artifact, *to)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("rolled %s back to %s (new current version %s)\n", *artifact, *to, newVer)
	return 0
}

func cmdPromoteLatest(args []string) int {
	fs := flag.NewFlagSet("promote-latest", flag.ContinueOnError)
	var c common
	addCommon(fs, &c)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := c.validate(); err != nil {
		return fail(err)
	}
	ctx := context.Background()
	admin, err := newAdmin(ctx, c)
	if err != nil {
		return fail(err)
	}
	n, err := admin.PromoteLatest(ctx, c.repo, c.release)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("promoted %s to latest/ (%d artifacts)\n", c.release, n)
	return 0
}

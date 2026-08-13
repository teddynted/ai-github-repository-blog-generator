// Command publish-release runs the human-approved publish for a release. It is
// invoked (via a Lambda Function URL) by the n8n Approval Poller when a reviewer
// comments /approve on the release's approval issue. It does the AWS + GitHub work
// n8n can't: reads the generated blog from S3, opens a PR adding it to the target
// repo, and promotes the release's artifacts to an approved-only "published/" S3
// prefix.
//
// Auth: the Function URL is unauthenticated at the edge; this handler rejects any
// request whose X-Publish-Secret header does not match the shared secret in SSM,
// so only the n8n poller (which holds the secret) can invoke it.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
)

// Request is the JSON body the poller POSTs.
type Request struct {
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	ReleaseTag string `json:"release_tag"`
}

type env struct {
	bucket       string
	prefix       string // generated-content
	published    string // published
	targetRepo   string // owner/repo the PR is opened in
	blogPath     string // docs/blog
	githubToken  string
	sharedSecret string
	s3           *s3.Client
	http         *http.Client
}

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion"); err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion),
		awsconfig.WithRetryMode(aws.RetryModeAdaptive), awsconfig.WithRetryMaxAttempts(5))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	sm := ssm.NewFromConfig(awsCfg)
	e := &env{
		bucket:     os.Getenv("CONTENT_BUCKET"),
		prefix:     envStr("CONTENT_PREFIX", "generated-content"),
		published:  envStr("PUBLISHED_PREFIX", "published"),
		targetRepo: os.Getenv("TARGET_REPO"),
		blogPath:   envStr("BLOG_PATH_PREFIX", "docs/blog"),
		s3:         s3.NewFromConfig(awsCfg),
		http:       &http.Client{Timeout: 20 * time.Second},
	}
	if e.bucket == "" || e.targetRepo == "" {
		log.Fatalf("CONTENT_BUCKET and TARGET_REPO are required")
	}
	e.githubToken = mustParam(ctx, sm, os.Getenv("GITHUB_TOKEN_PARAM"))
	e.sharedSecret = mustParam(ctx, sm, os.Getenv("SHARED_SECRET_PARAM"))

	lambda.Start(func(ctx context.Context, evt events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		if subtle.ConstantTimeCompare([]byte(header(evt.Headers, "x-publish-secret")), []byte(e.sharedSecret)) != 1 {
			return resp(401, map[string]string{"error": "unauthorized"}), nil
		}
		var req Request
		if err := json.Unmarshal([]byte(evt.Body), &req); err != nil || req.Owner == "" || req.Name == "" || req.ReleaseTag == "" {
			return resp(400, map[string]string{"error": "owner, name and release_tag are required"}), nil
		}
		prURL, err := e.publish(ctx, req, a.Logger)
		if err != nil {
			a.Logger.Error("publish failed", "repo", req.Owner+"/"+req.Name, "tag", req.ReleaseTag, "error", err.Error())
			return resp(500, map[string]string{"error": err.Error()}), nil
		}
		a.Logger.Info("published", "tag", req.ReleaseTag, "pr", prURL)
		return resp(200, map[string]string{"status": "ok", "pr": prURL}), nil
	})
}

func (e *env) publish(ctx context.Context, r Request, logger interface{ Info(string, ...any) }) (string, error) {
	relBase := path.Join(e.prefix, r.Owner, r.Name, "releases", r.ReleaseTag)

	// 1) read the generated blog from S3.
	blog, err := e.getObject(ctx, path.Join(relBase, "blog.md"))
	if err != nil {
		return "", fmt.Errorf("read blog from s3: %w", err)
	}

	// 2) promote every release artifact to the approved-only published/ prefix.
	if err := e.promote(ctx, relBase, path.Join(e.published, r.Owner, r.Name, r.ReleaseTag), logger); err != nil {
		return "", fmt.Errorf("promote to published prefix: %w", err)
	}

	// 3) open a PR adding the blog to the target repo.
	filePath := path.Join(e.blogPath, fmt.Sprintf("%s-%s-%s.md", r.Owner, r.Name, r.ReleaseTag))
	branch := fmt.Sprintf("publish/%s-%s-%s", r.Owner, r.Name, r.ReleaseTag)
	title := fmt.Sprintf("Publish blog: %s/%s %s", r.Owner, r.Name, r.ReleaseTag)
	prURL, err := e.openPR(ctx, branch, filePath, title, blog)
	if err != nil {
		return "", fmt.Errorf("open pull request: %w", err)
	}
	return prURL, nil
}

// --- S3 helpers -------------------------------------------------------------

func (e *env) getObject(ctx context.Context, key string) ([]byte, error) {
	out, err := e.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: &e.bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (e *env) promote(ctx context.Context, srcPrefix, dstPrefix string, logger interface{ Info(string, ...any) }) error {
	var token *string
	for {
		list, err := e.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &e.bucket, Prefix: aws.String(srcPrefix + "/"), ContinuationToken: token})
		if err != nil {
			return err
		}
		for _, obj := range list.Contents {
			src := aws.ToString(obj.Key)
			dst := dstPrefix + strings.TrimPrefix(src, srcPrefix)
			if _, err := e.s3.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     &e.bucket,
				Key:        aws.String(dst),
				CopySource: aws.String(e.bucket + "/" + src),
			}); err != nil {
				return fmt.Errorf("copy %s: %w", src, err)
			}
		}
		if list.IsTruncated == nil || !*list.IsTruncated {
			break
		}
		token = list.NextContinuationToken
	}
	logger.Info("promoted artifacts", "from", srcPrefix, "to", dstPrefix)
	return nil
}

// --- GitHub helpers (raw REST, no SDK) --------------------------------------

func (e *env) gh(ctx context.Context, method, apiPath string, body any) (map[string]any, int, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com/repos/"+e.targetRepo+apiPath, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+e.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, resp.StatusCode, fmt.Errorf("github %s %s: %d: %s", method, apiPath, resp.StatusCode, string(raw))
	}
	return out, resp.StatusCode, nil
}

func (e *env) openPR(ctx context.Context, branch, filePath, title string, content []byte) (string, error) {
	// default branch + its head sha
	repo, _, err := e.gh(ctx, http.MethodGet, "", nil)
	if err != nil {
		return "", err
	}
	base, _ := repo["default_branch"].(string)
	if base == "" {
		base = "main"
	}
	ref, _, err := e.gh(ctx, http.MethodGet, "/git/ref/heads/"+base, nil)
	if err != nil {
		return "", err
	}
	baseSHA, _ := nestedString(ref, "object", "sha")
	if baseSHA == "" {
		return "", fmt.Errorf("could not resolve %s head sha", base)
	}
	// branch (ignore "already exists")
	if _, code, err := e.gh(ctx, http.MethodPost, "/git/refs", map[string]string{"ref": "refs/heads/" + branch, "sha": baseSHA}); err != nil && code != http.StatusUnprocessableEntity {
		return "", err
	}
	// commit the blog file on the branch
	if _, _, err := e.gh(ctx, http.MethodPut, "/contents/"+filePath, map[string]any{
		"message": title,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}); err != nil {
		return "", err
	}
	// open the PR
	pr, _, err := e.gh(ctx, http.MethodPost, "/pulls", map[string]string{
		"title": title,
		"head":  branch,
		"base":  base,
		"body":  "Automated publish of the approved release blog.",
	})
	if err != nil {
		return "", err
	}
	url, _ := pr["html_url"].(string)
	return url, nil
}

// --- small helpers ----------------------------------------------------------

func mustParam(ctx context.Context, sm *ssm.Client, name string) string {
	if name == "" {
		log.Fatalf("required SSM parameter env not set")
	}
	out, err := sm.GetParameter(ctx, &ssm.GetParameterInput{Name: &name, WithDecryption: aws.Bool(true)})
	if err != nil {
		log.Fatalf("read ssm parameter %s: %v", name, err)
	}
	v := aws.ToString(out.Parameter.Value)
	if v == "" {
		log.Fatalf("ssm parameter %s is empty", name)
	}
	return v
}

func header(headers map[string]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func nestedString(m map[string]any, keys ...string) (string, bool) {
	cur := any(m)
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur = mm[k]
	}
	s, ok := cur.(string)
	return s, ok
}

func resp(code int, body any) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(body)
	return events.APIGatewayV2HTTPResponse{StatusCode: code, Headers: map[string]string{"Content-Type": "application/json"}, Body: string(b)}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

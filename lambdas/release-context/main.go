// Command release-context is the AWS Lambda behind the authenticated
// POST /release-context REST endpoint. It validates a JSON request, builds the
// structured Release Context for the given repository + release via the
// Content Intelligence engine, optionally persists it to S3, and returns an
// acceptance envelope with the context id.
//
// The route is distinct from POST /process (which is the manual pipeline
// trigger) so both entry points coexist; /release-context is the Content
// Intelligence entrypoint for Milestone 2.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasesource"
)

// contextBuilder builds a Release Context (satisfied by *releasecontext.Builder;
// an interface so handle is unit-testable).
type contextBuilder interface {
	Build(ctx context.Context, req rc.Request) (*rc.ReleaseContext, error)
}

// store persists a built context and returns a location reference. Optional.
type store interface {
	Save(ctx context.Context, id string, data []byte) (string, error)
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
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	// A GitHub token is optional (public repos work rate-limited); a scoped
	// read token via GITHUB_TOKEN lifts the limits and reaches private repos.
	token := os.Getenv("GITHUB_TOKEN")
	src := releasesource.New(github.New(), token)
	builder := &rc.Builder{Sources: src, Logger: a.Logger}

	var st store
	if bucket := os.Getenv("CONTEXT_BUCKET"); bucket != "" {
		st = &s3Store{client: s3.NewFromConfig(awsCfg), bucket: bucket}
	}

	lambda.Start(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		body := []byte(req.Body)
		if req.IsBase64Encoded {
			if decoded, derr := base64.StdEncoding.DecodeString(req.Body); derr == nil {
				body = decoded
			}
		}
		status, respBody := handle(ctx, builder, st, a.Logger, req.RequestContext.RequestID, body)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(respBody),
		}, nil
	})
}

// requestBody is the POST /release-context payload.
type requestBody struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	ReleaseTag string `json:"releaseTag"`
}

// acceptedResponse is the success body (HTTP 202).
type acceptedResponse struct {
	Status     string `json:"status"`
	ContextID  string `json:"contextId"`
	Repository string `json:"repository"`
	ReleaseTag string `json:"releaseTag"`
	Location   string `json:"location,omitempty"`
	Warnings   int    `json:"warnings,omitempty"`
}

type errorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// handle validates the request, builds the Release Context, optionally persists
// it, and maps the outcome to an HTTP status + JSON body. It is separated from
// the Lambda wiring so it is unit-testable.
func handle(ctx context.Context, b contextBuilder, st store, logger *slog.Logger, requestID string, body []byte) (int, []byte) {
	start := time.Now()

	var in requestBody
	if err := json.Unmarshal(body, &in); err != nil {
		return jsonErr(http.StatusBadRequest, "invalid JSON body")
	}
	req := rc.Request{Owner: in.Owner, Repository: in.Repository, ReleaseTag: in.ReleaseTag}

	built, err := b.Build(ctx, req)
	if err != nil {
		status, msg := mapError(err)
		logBuild(logger, requestID, req, "", false, err, time.Since(start))
		return jsonErr(status, msg)
	}

	location := ""
	if st != nil {
		if data, merr := json.Marshal(built); merr == nil {
			if loc, serr := st.Save(ctx, built.ContextID, data); serr != nil {
				if logger != nil {
					logger.Warn("persist release context failed", slog.String("error", serr.Error()))
				}
			} else {
				location = loc
			}
		}
	}

	logBuild(logger, requestID, req, built.ContextID, true, nil, time.Since(start))
	return jsonResp(http.StatusAccepted, acceptedResponse{
		Status: "accepted", ContextID: built.ContextID,
		Repository: req.Repository, ReleaseTag: req.ReleaseTag,
		Location: location, Warnings: len(built.Warnings),
	})
}

// mapError maps a typed application error to an HTTP status and safe message.
func mapError(err error) (int, string) {
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidInput:
		return http.StatusBadRequest, err.Error()
	case apperror.CodeNotFound:
		return http.StatusNotFound, "repository or release not found"
	case apperror.CodeUnauthorized:
		return http.StatusForbidden, "not authorized to read this repository"
	case apperror.CodeUpstream, apperror.CodeUnavailable:
		return http.StatusBadGateway, "GitHub is temporarily unavailable; please retry"
	default:
		return http.StatusInternalServerError, "internal error building release context"
	}
}

func logBuild(logger *slog.Logger, requestID string, req rc.Request, contextID string, ok bool, err error, dur time.Duration) {
	if logger == nil {
		return
	}
	outcome := "success"
	if !ok {
		outcome = "failure"
	}
	attrs := []any{
		slog.String("request_id", requestID),
		slog.String("repository", req.FullName()),
		slog.String("release", req.ReleaseTag),
		slog.String("outcome", outcome),
		slog.Duration("duration", dur),
	}
	if contextID != "" {
		attrs = append(attrs, slog.String("context_id", contextID))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	logger.Info("release context request", attrs...)
}

func jsonResp(status int, v any) (int, []byte) {
	b, err := json.Marshal(v)
	if err != nil {
		return http.StatusInternalServerError, []byte(`{"status":"error","error":"failed to encode response"}`)
	}
	return status, b
}

func jsonErr(status int, reason string) (int, []byte) {
	return jsonResp(status, errorResponse{Status: "error", Error: reason})
}

// Command dispatch-publish fires a GitHub repository_dispatch event so the
// content-approval workflow (publish-content.yml) opens its "Approve release …"
// issue automatically once the serverless pipeline has generated + reviewed +
// published a release's content. It is the last hop of the content state machine.
//
// It calls the public GitHub API (POST /repos/{repo}/dispatches), so it runs
// outside any VPC and needs only a token read from Secrets Manager — the token
// must have Contents: write on the dispatch repo (the one holding the workflow).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
)

// Event is the release identity from the content state machine.
type Event struct {
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	ReleaseTag string `json:"release_tag"`
}

type dispatchBody struct {
	EventType     string        `json:"event_type"`
	ClientPayload clientPayload `json:"client_payload"`
}

type clientPayload struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Tag   string `json:"tag"`
}

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion"); err != nil {
		log.Fatalf("config: %v", err)
	}

	dispatchRepo := os.Getenv("DISPATCH_REPO") // "owner/repo" holding the workflow
	eventType := envStr("DISPATCH_EVENT_TYPE", "content-generated")
	tokenSecret := os.Getenv("GITHUB_TOKEN_SECRET_ARN")
	if dispatchRepo == "" || tokenSecret == "" {
		log.Fatalf("DISPATCH_REPO and GITHUB_TOKEN_SECRET_ARN are required")
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(a.Config.AWSRegion),
		awsconfig.WithRetryMode(aws.RetryModeAdaptive),
		awsconfig.WithRetryMaxAttempts(5),
	)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	sm := secretsmanager.NewFromConfig(awsCfg)
	httpClient := &http.Client{Timeout: 10 * time.Second}

	lambda.Start(func(ctx context.Context, ev Event) (string, error) {
		if ev.ReleaseTag == "" {
			return "", fmt.Errorf("dispatch-publish: event has no release_tag")
		}
		out, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(tokenSecret)})
		if err != nil {
			return "", fmt.Errorf("read github token secret: %w", err)
		}
		token := aws.ToString(out.SecretString)
		if token == "" {
			return "", fmt.Errorf("github token secret is empty (set the real token with put-secret-value)")
		}

		payload, _ := json.Marshal(dispatchBody{
			EventType:     eventType,
			ClientPayload: clientPayload{Owner: ev.Owner, Name: ev.Name, Tag: ev.ReleaseTag},
		})
		url := "https://api.github.com/repos/" + dispatchRepo + "/dispatches"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			a.Logger.Error("repository_dispatch failed", "url", url, "error", err.Error())
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("repository_dispatch %s returned %d: %s", url, resp.StatusCode, string(body))
		}
		a.Logger.Info("repository_dispatch sent", "repo", dispatchRepo, "event", eventType, "tag", ev.ReleaseTag)
		return "ok", nil
	})
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

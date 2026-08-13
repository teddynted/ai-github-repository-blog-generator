// Command notify-n8n is the in-VPC bridge from the serverless content pipeline
// to the n8n orchestration plane. Option B of the trigger-ingress design: rather
// than exposing n8n to the public internet for a GitHub webhook, the content
// state machine invokes this Lambda after generation and it POSTs the release
// payload to n8n's private webhook (http://<private-ip>:5678/webhook/<path>).
//
// The state machine owns the host lifecycle (it starts the On-Demand EC2 host
// and reads its private IP via native SDK integrations, which run AWS-side and
// need no VPC egress). This Lambda receives that private IP, so it only talks to
// the box: it runs inside the VPC (same subnet), waits for n8n to report healthy,
// then delivers the payload. Keeping it POST-only means it needs no AWS API
// access — and therefore no NAT gateway or VPC endpoints.
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
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
)

// Event is the input from the content state machine: the host's private IP plus
// the release identity the review workflow needs.
type Event struct {
	PrivateIP  string `json:"private_ip"`
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	ReleaseTag string `json:"release_tag"`
}

// webhookBody is the JSON delivered to n8n. The shape matches what the review
// workflow's downstream nodes read ($json.body.release.tag_name), so the n8n
// side needs no rewrite when the caller changes.
type webhookBody struct {
	Release struct {
		TagName string `json:"tag_name"`
	} `json:"release"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	port := envInt("N8N_PORT", 5678)
	path := envStr("N8N_WEBHOOK_PATH", "release-review")
	readyTimeout := time.Duration(envInt("READY_TIMEOUT_SECONDS", 180)) * time.Second
	httpClient := &http.Client{Timeout: 10 * time.Second}

	lambda.Start(func(ctx context.Context, ev Event) (string, error) {
		if ev.PrivateIP == "" {
			return "", fmt.Errorf("notify-n8n: event has no private_ip")
		}
		base := fmt.Sprintf("http://%s:%d", ev.PrivateIP, port)

		// One overall deadline for "get n8n ready and deliver", bounded well under
		// the Lambda timeout so health-wait + webhook-retry never overrun.
		deadline := time.Now().Add(readyTimeout)

		// n8n takes a little while to come up after a cold host start; wait for
		// /healthz. Fail-loud so the state machine records a missed hand-off
		// rather than silently dropping the release.
		if err := waitForHealthy(ctx, httpClient, base, deadline, a.Logger); err != nil {
			a.Logger.Error("n8n never became healthy", "base", base, "error", err.Error())
			return "", err
		}

		var body webhookBody
		body.Release.TagName = ev.ReleaseTag
		body.Owner, body.Name = ev.Owner, ev.Name
		payload, _ := json.Marshal(body)

		// After /healthz is OK the active workflow's production webhook can take a
		// few more seconds to register — a cold start returns 404 "Cannot POST
		// /webhook/…". Retry past that (and transient connection blips) until the
		// shared deadline, so the hand-off is robust when the host was just started.
		url := base + "/webhook/" + path
		for {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
			if err != nil {
				return "", err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := httpClient.Do(req)
			if err != nil {
				if time.Now().After(deadline) {
					a.Logger.Error("post to n8n webhook failed", "url", url, "error", err.Error())
					return "", err
				}
				a.Logger.Info("webhook post retry (connection)", "url", url, "error", err.Error())
				if serr := sleep(ctx, 5*time.Second); serr != nil {
					return "", serr
				}
				continue
			}
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound && time.Now().Before(deadline) {
				a.Logger.Info("webhook not registered yet; retrying", "url", url)
				if serr := sleep(ctx, 5*time.Second); serr != nil {
					return "", serr
				}
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return "", fmt.Errorf("n8n webhook %s returned %d: %s", url, resp.StatusCode, string(respBody))
			}
			a.Logger.Info("release handed to n8n", "url", url, "tag", ev.ReleaseTag, "status", resp.StatusCode)
			return "ok", nil
		}
	})
}

// waitForHealthy polls n8n's /healthz until it returns 2xx or the deadline hits.
func waitForHealthy(ctx context.Context, client *http.Client, base string, deadline time.Time, logger interface {
	Info(string, ...any)
}) error {
	url := base + "/healthz"
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		logger.Info("waiting for n8n health", "url", url)
		if time.Now().After(deadline) {
			return fmt.Errorf("n8n at %s did not become healthy before deadline", url)
		}
		if err := sleep(ctx, 5*time.Second); err != nil {
			return err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

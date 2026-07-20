package contentanalytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// getJSON performs a GET via the HTTPDoer and decodes the JSON body into out.
// It classifies transport and status errors into recoverable/permanent
// CollectErrors so the collector can decide whether to retry. It never logs
// credentials — headers are set by the caller and not echoed.
func getJSON(ctx context.Context, doer HTTPDoer, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errMissing("build request: " + err.Error())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := doer.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return errTimeout("request context: " + ctx.Err().Error())
		}
		return errNetwork("request failed", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		return classifyStatus(resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return errPartial("decode response: " + err.Error())
		}
	}
	return nil
}

// postJSON performs a POST with a JSON body (e.g. Hashnode GraphQL).
func postJSON(ctx context.Context, doer HTTPDoer, url string, headers map[string]string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return errMissing("encode request: " + err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return errMissing("build request: " + err.Error())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := doer.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return errTimeout("request context: " + ctx.Err().Error())
		}
		return errNetwork("request failed", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		return classifyStatus(resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return errPartial("decode response: " + err.Error())
		}
	}
	return nil
}

// classifyStatus maps an HTTP status into a classified CollectError.
func classifyStatus(status int, body string) *CollectError {
	snippet := body
	if len(snippet) > 160 {
		snippet = snippet[:160]
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return errAuth(fmt.Sprintf("status %d: %s", status, snippet))
	case status == http.StatusPaymentRequired:
		return errQuota(fmt.Sprintf("status %d: %s", status, snippet))
	case status == http.StatusTooManyRequests:
		return errRateLimit(fmt.Sprintf("status %d: %s", status, snippet))
	case status == http.StatusNotFound:
		return errMissing(fmt.Sprintf("status %d: %s", status, snippet))
	case status >= 500:
		return errServer(fmt.Sprintf("status %d: %s", status, snippet))
	default:
		return &CollectError{Code: "http", Message: fmt.Sprintf("status %d: %s", status, snippet), Recoverable: false}
	}
}

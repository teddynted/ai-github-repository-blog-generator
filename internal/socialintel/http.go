package socialintel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// getJSON performs a GET via the HTTPDoer and decodes JSON into out. It
// classifies transport and HTTP-status failures into recoverable/permanent
// CollectErrors so the collector's retry can act on them. It never logs or
// returns credential values.
func getJSON(ctx context.Context, doer HTTPDoer, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errMissing("failed to build request: " + err.Error())
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := doer.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return errTimeout(err)
		}
		return errNetwork(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err := classifyStatus(resp.StatusCode, raw); err != nil {
		return err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return errPartial("failed to parse response")
		}
	}
	return nil
}

func classifyStatus(code int, body []byte) error {
	if code >= 200 && code < 300 {
		return nil
	}
	msg := snippet(body)
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return errAuth("authentication/authorization failed")
	case code == http.StatusTooManyRequests:
		return errRateLimit("rate limited: " + msg)
	case code == http.StatusPaymentRequired:
		return errQuota("quota exceeded: " + msg)
	case code >= 500:
		return errPlatformDown("platform error: " + msg)
	default:
		return errMissing("unexpected status: " + msg)
	}
}

func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

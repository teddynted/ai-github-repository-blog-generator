package publishing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// doJSON performs a JSON request via the HTTPDoer and decodes the response into
// out (when non-nil). It classifies transport and HTTP-status failures into
// recoverable/permanent PubErrors so the retry engine can act on them. It never
// logs or returns credential values.
func doJSON(ctx context.Context, doer HTTPDoer, method, url string, headers map[string]string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, errInvalid("failed to encode request body")
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, errInvalid("failed to build request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := doer.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, errTimeout(err)
		}
		return 0, errNetwork(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if err := classifyStatus(resp.StatusCode, raw); err != nil {
		return resp.StatusCode, err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, errInvalid("failed to parse response")
		}
	}
	return resp.StatusCode, nil
}

// classifyStatus maps an HTTP status to a recoverable/permanent PubError.
func classifyStatus(code int, body []byte) error {
	if code >= 200 && code < 300 {
		return nil
	}
	msg := snippet(body)
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return errAuth("authentication/authorization failed (" + itoa(code) + ")")
	case code == http.StatusTooManyRequests:
		return errRateLimit("rate limited (429): " + msg)
	case code == http.StatusConflict:
		return errDuplicate("duplicate publication (409): " + msg)
	case code == http.StatusPaymentRequired:
		return errQuota("quota exceeded (402): " + msg)
	case code == http.StatusUnprocessableEntity || code == http.StatusBadRequest:
		return errInvalid("invalid request (" + itoa(code) + "): " + msg)
	case code >= 500:
		return errPlatformDown("platform error (" + itoa(code) + "): " + msg)
	default:
		return newErr("http", "unexpected status "+itoa(code)+": "+msg, false, nil)
	}
}

func snippet(b []byte) string {
	s := collapse(string(b))
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.TrimSpace(s)
}

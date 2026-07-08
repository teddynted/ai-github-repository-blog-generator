package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestErrorMessageIncludesCode(t *testing.T) {
	e := New(CodeInvalidInput, "bad repo url")
	if got := e.Error(); got != "invalid_input: bad repo url" {
		t.Errorf("Error() = %q", got)
	}
}

func TestWrapUnwrapsCause(t *testing.T) {
	cause := errors.New("dial tcp: timeout")
	e := Wrap(cause, CodeUpstream, "github unreachable")
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	if !contains(e.Error(), "dial tcp") {
		t.Errorf("Error() should include cause: %q", e.Error())
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	cases := map[Code]int{
		CodeInvalidInput: http.StatusBadRequest,
		CodeUnauthorized: http.StatusUnauthorized,
		CodeNotFound:     http.StatusNotFound,
		CodeConflict:     http.StatusConflict,
		CodeUpstream:     http.StatusBadGateway,
		CodeUnavailable:  http.StatusServiceUnavailable,
		CodeInternal:     http.StatusInternalServerError,
	}
	for code, want := range cases {
		if got := New(code, "x").HTTPStatus(); got != want {
			t.Errorf("HTTPStatus(%s) = %d, want %d", code, got, want)
		}
	}
}

func TestCodeOfAndHTTPStatusOf(t *testing.T) {
	// A plain error defaults to internal / 500.
	plain := errors.New("boom")
	if CodeOf(plain) != CodeInternal {
		t.Errorf("CodeOf(plain) = %s", CodeOf(plain))
	}
	if HTTPStatusOf(plain) != http.StatusInternalServerError {
		t.Errorf("HTTPStatusOf(plain) = %d", HTTPStatusOf(plain))
	}

	// A wrapped *Error is discovered even through fmt.Errorf.
	wrapped := fmt.Errorf("context: %w", New(CodeNotFound, "repo not registered"))
	if CodeOf(wrapped) != CodeNotFound {
		t.Errorf("CodeOf(wrapped) = %s", CodeOf(wrapped))
	}
	if HTTPStatusOf(wrapped) != http.StatusNotFound {
		t.Errorf("HTTPStatusOf(wrapped) = %d", HTTPStatusOf(wrapped))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

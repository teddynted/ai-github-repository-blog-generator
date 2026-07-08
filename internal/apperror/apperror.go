// Package apperror defines a small, typed error used across the platform so
// that failures carry a stable machine-readable code and an HTTP status,
// while still composing with the standard errors package (Is/As/Unwrap).
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable classifier for a failure. Codes are kept
// coarse on purpose; they map to HTTP statuses and drive alerting.
type Code string

const (
	CodeInvalidInput Code = "invalid_input"
	CodeUnauthorized Code = "unauthorized"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeUpstream     Code = "upstream_error"
	CodeInternal     Code = "internal_error"
	CodeUnavailable  Code = "unavailable"
)

// Error is the platform's typed error. It is safe to surface Message to
// clients; wrapped causes (Err) are for logs only and should never contain
// secret material.
type Error struct {
	Code    Code
	Message string
	Err     error
}

// New creates an Error without an underlying cause.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap annotates an underlying error with a code and message.
func Wrap(err error, code Code, message string) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Err }

// HTTPStatus maps the code to an HTTP status for API Gateway responses.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeInvalidInput:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeUpstream:
		return http.StatusBadGateway
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// CodeOf extracts the Code from any error, returning CodeInternal when the
// error is not (and does not wrap) an *Error.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// HTTPStatusOf returns the HTTP status for any error, defaulting to 500.
func HTTPStatusOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.HTTPStatus()
	}
	return http.StatusInternalServerError
}

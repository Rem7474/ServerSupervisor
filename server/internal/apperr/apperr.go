// Package apperr defines typed application errors so the service layer can express
// failure *semantics* (not found / validation / conflict / …) independently of
// HTTP, and handlers can translate them uniformly. It carries a stable machine
// code alongside the human message; the HTTP layer renders both (the message
// keeps the existing `{"error": "..."}` shape for backward compatibility and adds
// a `code` field). apperr imports nothing framework-specific on purpose.
package apperr

import "errors"

// Error is a domain error with a stable code and an HTTP status.
type Error struct {
	Code       string // machine-readable: not_found | validation | conflict | forbidden | unauthorized | internal
	Message    string // human-facing message (unchanged from the previous gin.H{"error": …})
	HTTPStatus int
	wrapped    error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.wrapped }

// NotFound — the requested resource does not exist (404).
func NotFound(msg string) *Error { return &Error{Code: "not_found", Message: msg, HTTPStatus: 404} }

// Validation — the request was malformed or failed validation (400).
func Validation(msg string) *Error { return &Error{Code: "validation", Message: msg, HTTPStatus: 400} }

// Conflict — the request conflicts with current state, e.g. a duplicate (409).
func Conflict(msg string) *Error { return &Error{Code: "conflict", Message: msg, HTTPStatus: 409} }

// Forbidden — authenticated but not allowed (403).
func Forbidden(msg string) *Error { return &Error{Code: "forbidden", Message: msg, HTTPStatus: 403} }

// Unauthorized — authentication required or failed (401).
func Unauthorized(msg string) *Error {
	return &Error{Code: "unauthorized", Message: msg, HTTPStatus: 401}
}

// TooManyRequests — the caller is rate-limited / brute-force-blocked (429).
func TooManyRequests(msg string) *Error {
	return &Error{Code: "too_many_requests", Message: msg, HTTPStatus: 429}
}

// Failed — the operation failed for a reason worth surfacing verbatim (500),
// e.g. an external-dependency diagnostic (SMTP/ntfy connectivity test) where the
// message itself is the useful result. Unlike Internal it keeps the human message.
func Failed(msg string) *Error { return &Error{Code: "failed", Message: msg, HTTPStatus: 500} }

// Internal wraps an unexpected error as a 500 with a generic message; the cause is
// preserved via Unwrap for logging.
func Internal(err error) *Error {
	return &Error{Code: "internal", Message: "internal server error", HTTPStatus: 500, wrapped: err}
}

// From returns err as an *Error if it already is one (anywhere in its chain),
// otherwise it wraps it as Internal. Handlers use this so any error — typed or
// not — renders consistently.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}

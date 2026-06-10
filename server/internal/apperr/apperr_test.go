package apperr

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

func TestConstructorsCarryCodeAndStatus(t *testing.T) {
	cases := []struct {
		err        *Error
		wantCode   string
		wantStatus int
	}{
		{NotFound("x"), "not_found", 404},
		{Validation("x"), "validation", 400},
		{Conflict("x"), "conflict", 409},
		{Forbidden("x"), "forbidden", 403},
		{Unauthorized("x"), "unauthorized", 401},
	}
	for _, c := range cases {
		if c.err.Code != c.wantCode || c.err.HTTPStatus != c.wantStatus {
			t.Errorf("got code=%q status=%d, want %q/%d", c.err.Code, c.err.HTTPStatus, c.wantCode, c.wantStatus)
		}
		if c.err.Error() != "x" {
			t.Errorf("Error() should return the message, got %q", c.err.Error())
		}
	}
}

func TestFrom_PassesThroughTypedError(t *testing.T) {
	orig := NotFound("nope")
	// Even wrapped a few layers deep, From must recover the typed error.
	wrapped := fmt.Errorf("context: %w", orig)
	got := From(wrapped)
	if got != orig {
		t.Fatalf("From should recover the original *Error, got %#v", got)
	}
	if got.HTTPStatus != 404 {
		t.Errorf("recovered status = %d, want 404", got.HTTPStatus)
	}
}

func TestFrom_WrapsUnknownAsInternal(t *testing.T) {
	got := From(errors.New("boom"))
	if got.Code != "internal" || got.HTTPStatus != 500 {
		t.Errorf("unknown error should become internal/500, got %q/%d", got.Code, got.HTTPStatus)
	}
	// The cause must be preserved for logging.
	if !errors.Is(got, got.wrapped) || got.wrapped == nil {
		t.Error("Internal must keep the wrapped cause")
	}
}

func TestInternal_UnwrapsCause(t *testing.T) {
	e := Internal(sql.ErrConnDone)
	if !errors.Is(e, sql.ErrConnDone) {
		t.Error("errors.Is should see through Internal to the cause")
	}
}

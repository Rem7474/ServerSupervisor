package ws

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/serversupervisor/server/internal/apperr"
)

func TestVNCCloseFor(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   int
		wantReason string
	}{
		{"catalog key wins over message", apperr.Validation("x").I18n(apperr.CodeProxmoxVNCQemuOnly, nil), vncCloseBadRequest, apperr.CodeProxmoxVNCQemuOnly},
		{"forbidden", apperr.Forbidden("admin only").I18n(apperr.CodeProxmoxConsoleAdminOnly, nil), vncCloseForbidden, apperr.CodeProxmoxConsoleAdminOnly},
		{"not found", apperr.NotFound("guest not found"), vncCloseNotFound, "guest not found"},
		{"bad gateway keeps PVE's message", apperr.BadGateway("pve login: HTTP 401"), vncCloseBadGateway, "pve login: HTTP 401"},
		{"other apperr status", apperr.Conflict("busy"), vncCloseInternal, "busy"},
		{"plain error", errors.New("boom"), vncCloseInternal, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, reason := vncCloseFor(tc.err)
			if code != tc.wantCode || reason != tc.wantReason {
				t.Errorf("got %d %q, want %d %q", code, reason, tc.wantCode, tc.wantReason)
			}
		})
	}
}

// A close reason longer than RFC 6455 allows makes gorilla refuse to send the
// frame at all, so the browser would see no reason; it must also stay valid
// UTF-8 when the cut lands inside a multi-byte character.
func TestTruncateCloseReason(t *testing.T) {
	if got := truncateCloseReason("short"); got != "short" {
		t.Errorf("short reason changed: %q", got)
	}
	long := strings.Repeat("a", maxCloseReasonLen-1) + "é" + "tail"
	got := truncateCloseReason(long)
	if len(got) > maxCloseReasonLen || !utf8.ValidString(got) {
		t.Errorf("truncated reason = %d bytes, valid=%v", len(got), utf8.ValidString(got))
	}
	if got != strings.Repeat("a", maxCloseReasonLen-1) {
		t.Errorf("the split character should be dropped entirely, got %q", got[len(got)-3:])
	}
}

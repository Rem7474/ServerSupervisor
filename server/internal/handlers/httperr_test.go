package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// TestRespondError_RecordsDetailOnGinContext covers both detail sources
// respondError feeds to the access log via c.Error (internal/api/
// middleware.go's RequestLogger reads it back): a plain typed error's own
// message, and — the previously-uncovered branch — an apperr.Internal's
// wrapped cause, which e.Message hides from the client on purpose.
func TestRespondError_RecordsDetailOnGinContext(t *testing.T) {
	t.Run("non-internal error records its own message", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		respondError(c, apperr.BadGateway("proxmox API returned HTTP 401"))

		if len(c.Errors) != 1 {
			t.Fatalf("expected exactly one recorded error, got %d", len(c.Errors))
		}
		if got := c.Errors.Last().Error(); got != "proxmox API returned HTTP 401" {
			t.Errorf("expected the BadGateway message verbatim, got %q", got)
		}
	})

	t.Run("internal error records the unwrapped cause, not the generic message", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		cause := errors.New("dial tcp 10.0.0.5:8006: connect: connection refused")
		respondError(c, apperr.Internal(cause))

		if len(c.Errors) != 1 {
			t.Fatalf("expected exactly one recorded error, got %d", len(c.Errors))
		}
		if got := c.Errors.Last().Error(); got != cause.Error() {
			t.Errorf("expected the unwrapped cause %q, got %q", cause.Error(), got)
		}
	})
}

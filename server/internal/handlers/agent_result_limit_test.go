package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// A command result larger than maxCommandResultSize is refused before it is
// decoded, so the service (nil here) is never reached.
func TestReportCommandResult_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AgentHandler{}
	r := gin.New()
	r.POST("/result", func(c *gin.Context) {
		c.Set("host_id", "h1")
		h.ReportCommandResult(c)
	})

	body := `{"command_id":"c1","status":"completed","output":"` + strings.Repeat("a", maxCommandResultSize) + `"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/result", strings.NewReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

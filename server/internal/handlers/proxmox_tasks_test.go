package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetTaskLog_InvalidUPID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProxmoxHandler{}

	for _, invalid := range []string{"", "../etc/passwd", "UPID/123", "UPID?foo=bar"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{
			{Key: "id", Value: "node-1"},
			{Key: "upid", Value: invalid},
		}
		c.Request, _ = http.NewRequest(http.MethodGet, "/tasks/log", nil)

		h.GetTaskLog(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("for upid %q, expected status 400, got %d", invalid, w.Code)
		}
	}
}

package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

func TestDashboardInit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/dashboard/init", nil)

	handler := &WSHandler{
		dashCache: &models.WSDashboardSnapshot{
			Type: "dashboard",
			Hosts: []models.Host{
				{ID: "host-1", Name: "Host 1"},
			},
		},
		dashCacheAt: time.Now(),
	}

	handler.DashboardInit(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp models.WSDashboardSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.Hosts) != 1 || resp.Hosts[0].ID != "host-1" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

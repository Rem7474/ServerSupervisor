package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
	networksvc "github.com/serversupervisor/server/internal/services/network"
)

type fakeNetworkRepo struct {
	saved  *models.NetworkTopologyConfig
	audits []string
}

func (f *fakeNetworkRepo) GetNetworkTopologyConfig(context.Context) (*models.NetworkTopologyConfig, error) {
	return f.saved, nil
}
func (f *fakeNetworkRepo) SaveNetworkTopologyConfig(_ context.Context, cfg *models.NetworkTopologyConfig) error {
	f.saved = cfg
	return nil
}
func (f *fakeNetworkRepo) CreateAuditLog(_ context.Context, username, action, _, _, _, status string) (int64, error) {
	f.audits = append(f.audits, username+":"+action+":"+status)
	return 1, nil
}

func TestNetworkHandler_SaveTopologyConfig_RBACAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	run := func(role string, repo *fakeNetworkRepo) int {
		h := NewNetworkHandler(networksvc.NewService(repo, nil, nil))
		r := gin.New()
		r.PUT("/network/config", func(c *gin.Context) {
			c.Set("role", role)
			c.Set("username", "alice")
			h.SaveTopologyConfig(c)
		})
		req := httptest.NewRequest(http.MethodPut, "/network/config", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("viewer denied with 403 and no write or audit", func(t *testing.T) {
		repo := &fakeNetworkRepo{}
		if code := run(models.RoleViewer, repo); code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", code)
		}
		if repo.saved != nil || len(repo.audits) != 0 {
			t.Fatalf("viewer must not persist or audit: saved=%v audits=%v", repo.saved, repo.audits)
		}
	})

	for _, role := range []string{models.RoleOperator, models.RoleAdmin} {
		t.Run(role+" succeeds and writes audit", func(t *testing.T) {
			repo := &fakeNetworkRepo{}
			if code := run(role, repo); code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if repo.saved == nil || len(repo.audits) != 1 || repo.audits[0] != "alice:update_network_config:success" {
				t.Fatalf("expected save + audit, got saved=%v audits=%v", repo.saved, repo.audits)
			}
		})
	}
}

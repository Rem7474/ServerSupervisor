package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

// fakeHostAccess stubs the GetHostAccess lookup requireHostAccess depends on.
type fakeHostAccess struct {
	restricted bool
	level      string
	err        error
}

func (f fakeHostAccess) GetHostAccess(context.Context, string, string) (bool, string, error) {
	return f.restricted, f.level, f.err
}

// runRequireHostAccess invokes requireHostAccess for a caller with the given
// global role and returns whether access was granted plus the HTTP status the
// helper wrote (200 when it wrote nothing).
func runRequireHostAccess(role string, access fakeHostAccess, hostID, requiredLevel string) (bool, int) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("role", role)
	c.Set("username", "someone")
	ok := requireHostAccess(c, access, hostID, requiredLevel)
	if ok {
		return true, http.StatusOK
	}
	return false, w.Code
}

func TestRequireHostAccess_NoRows_UsesGlobalRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	noRows := fakeHostAccess{restricted: false}

	cases := []struct {
		name     string
		role     string
		required string
		wantOK   bool
	}{
		{"viewer w/o rows denied operator action", models.RoleViewer, "operator", false},
		{"unknown role w/o rows denied operator action", "", "operator", false},
		{"viewer w/o rows allowed viewer action", models.RoleViewer, "viewer", true},
		{"operator w/o rows allowed operator action", models.RoleOperator, "operator", true},
		{"operator w/o rows allowed viewer action", models.RoleOperator, "viewer", true},
		{"admin always allowed", models.RoleAdmin, "operator", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, code := runRequireHostAccess(tc.role, noRows, "host-1", tc.required)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (status %d)", ok, tc.wantOK, code)
			}
			if !tc.wantOK && code != http.StatusForbidden {
				t.Fatalf("denied request must be 403, got %d", code)
			}
		})
	}
}

func TestRequireHostAccess_ExplicitGrants(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("per-host operator grant passes operator action", func(t *testing.T) {
		ok, _ := runRequireHostAccess(models.RoleViewer, fakeHostAccess{restricted: true, level: "operator"}, "host-1", "operator")
		if !ok {
			t.Fatal("explicit per-host operator grant must pass")
		}
	})
	t.Run("per-host viewer grant denied operator action", func(t *testing.T) {
		ok, code := runRequireHostAccess(models.RoleOperator, fakeHostAccess{restricted: true, level: "viewer"}, "host-1", "operator")
		if ok || code != http.StatusForbidden {
			t.Fatalf("viewer grant must be denied operator action, ok=%v code=%d", ok, code)
		}
	})
	t.Run("restricted user on unlisted host denied", func(t *testing.T) {
		ok, code := runRequireHostAccess(models.RoleOperator, fakeHostAccess{restricted: true, level: ""}, "host-2", "viewer")
		if ok || code != http.StatusForbidden {
			t.Fatalf("unlisted host must be 403, ok=%v code=%d", ok, code)
		}
	})
}

func TestRequireHostAccess_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	if ok, code := runRequireHostAccess(models.RoleOperator, fakeHostAccess{}, "", "operator"); ok || code != http.StatusBadRequest {
		t.Errorf("missing host_id must be 400, ok=%v code=%d", ok, code)
	}
	if ok, code := runRequireHostAccess(models.RoleOperator, fakeHostAccess{err: errors.New("db down")}, "host-1", "operator"); ok || code < 500 {
		t.Errorf("lookup failure must be a 5xx, ok=%v code=%d", ok, code)
	}
}

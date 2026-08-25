package proxmoxclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetNodeTasksTypeFilter(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)

	if _, err := c.GetNodeTasks("pve1", 50, ""); err != nil {
		t.Fatalf("GetNodeTasks: %v", err)
	}
	if gotQuery != "limit=50" {
		t.Errorf("no typeFilter: query = %q, want %q", gotQuery, "limit=50")
	}

	if _, err := c.GetNodeTasks("pve1", 50, "vzdump"); err != nil {
		t.Fatalf("GetNodeTasks: %v", err)
	}
	if gotQuery != "limit=50&typefilter=vzdump" {
		t.Errorf("with typeFilter: query = %q, want %q", gotQuery, "limit=50&typefilter=vzdump")
	}
}

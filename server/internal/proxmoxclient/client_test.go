package proxmoxclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetWithTotal_PreservesBaseURLPathPrefix is the regression guard for a
// real production bug: a real Proxmox connection's base URL always carries
// an API path prefix (e.g. "https://pve.example.com:8006/api2/json"), not
// just a bare host. getWithTotal used to resolve each request path via
// url.URL.ResolveReference, which — per RFC 3986 — replaces a base URL's
// *entire* path with an absolute-path reference (any path starting with
// "/", which every call site uses) instead of appending to it, silently
// dropping "/api2/json" from every single GET request. PVE's web server then
// received a request for a literal path like "/nodes" instead of
// "/api2/json/nodes", which isn't a valid API route, and reported it as
// "no such file '/nodes'" — indistinguishable at the client from a broken
// Proxmox connection, when the client itself was building the wrong URL.
//
// This is deliberately NOT caught by httptest.NewServer(...).URL alone: that
// URL has no path component, so ResolveReference's bug is invisible unless
// the base URL under test actually carries a path prefix the way a real PVE
// connection does — hence constructing one explicitly below.
func TestGetWithTotal_PreservesBaseURLPathPrefix(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL+"/api2/json", "user@pve!token", "secret-token", false)

	if _, err := c.GetNodes(); err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if want := "/api2/json/nodes"; gotPath != want {
		t.Errorf("request path = %q, want %q (base URL's /api2/json prefix must be preserved)", gotPath, want)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type openAPIOperation struct {
	OperationID string         `json:"operationId"`
	Responses   map[string]any `json:"responses"`
	Security    *[]any         `json:"security"`
}

type openAPIDoc struct {
	OpenAPI string                                 `json:"openapi"`
	Servers []struct{ URL string }                 `json:"servers"`
	Paths   map[string]map[string]openAPIOperation `json:"paths"`
	Comps   struct {
		SecuritySchemes map[string]struct {
			Type, Scheme string
		} `json:"securitySchemes"`
	} `json:"components"`
}

var openAPIParam = regexp.MustCompile(`\{([^}]+)\}`)

// documentedRoutes returns every documented operation as "METHOD /api/v1/path"
// with gin-style :params, the shape gin.RouteInfo uses.
func documentedRoutes(t *testing.T) (map[string]openAPIOperation, openAPIDoc) {
	t.Helper()
	var doc openAPIDoc
	if err := json.Unmarshal(openAPISpec, &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "/api/v1" {
		t.Fatalf("expected a single /api/v1 server, got %+v", doc.Servers)
	}
	out := map[string]openAPIOperation{}
	for path, ops := range doc.Paths {
		ginPath := "/api/v1" + openAPIParam.ReplaceAllString(path, ":$1")
		for method, op := range ops {
			out[strings.ToUpper(method)+" "+ginPath] = op
		}
	}
	return out, doc
}

func TestOpenAPISpec_WellFormed(t *testing.T) {
	routes, doc := documentedRoutes(t)
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Fatalf("openapi version = %q, want 3.x", doc.OpenAPI)
	}
	if s := doc.Comps.SecuritySchemes["apiKey"]; s.Type != "http" || s.Scheme != "bearer" {
		t.Fatalf("apiKey security scheme must be HTTP bearer, got %+v", s)
	}
	if len(routes) < 15 {
		t.Fatalf("only %d operations documented", len(routes))
	}
	ids := map[string]string{}
	for route, op := range routes {
		if op.OperationID == "" || len(op.Responses) == 0 {
			t.Errorf("%s: operationId and responses are required", route)
		}
		if prev, dup := ids[op.OperationID]; dup {
			t.Errorf("operationId %q used by %s and %s", op.OperationID, prev, route)
		}
		ids[op.OperationID] = route
	}
}

// Every documented operation must be usable with an API key: documenting a
// route the key middleware refuses would advertise a dead endpoint.
func TestOpenAPISpec_DocumentsOnlyKeyReachableRoutes(t *testing.T) {
	routes, _ := documentedRoutes(t)
	for route, op := range routes {
		method, path, _ := strings.Cut(route, " ")
		if op.Security != nil && len(*op.Security) == 0 {
			continue // public operation (the spec itself)
		}
		if apiKeyForbiddenRoute(method, path) {
			t.Errorf("%s is documented but closed to API keys", route)
		}
	}
}

func TestOpenAPIRoute_ServesTheEmbeddedSpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerOpenAPIRoute(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, content-type %q", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Body.String() != string(openAPISpec) {
		t.Fatal("served body differs from the embedded spec")
	}
}

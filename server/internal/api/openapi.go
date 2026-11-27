package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// openAPISpec is the OpenAPI 3 description of the public API subset (the
// endpoints usable with a personal API key). openapi_test.go checks that every
// documented operation is a registered route and is not closed to API keys.
//
//go:embed openapi.json
var openAPISpec []byte

// registerOpenAPIRoute serves the spec publicly: it describes the API, holds
// no data, and tools (Swagger UI, client generators) fetch it unauthenticated.
func registerOpenAPIRoute(r *gin.Engine) {
	r.GET("/api/v1/openapi.json", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "application/json; charset=utf-8", openAPISpec)
	})
}

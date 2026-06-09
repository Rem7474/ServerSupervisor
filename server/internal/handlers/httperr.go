package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// respondError renders any error as the uniform JSON envelope
// `{"error": "<message>", "code": "<machine-code>"}` with the status carried by
// the typed apperr.Error (unknown errors become a 500 "internal" error). The
// `error` string preserves the historical shape so existing frontend consumers
// keep working; `code` is the new machine-readable discriminator.
func respondError(c *gin.Context, err error) {
	e := apperr.From(err)
	c.JSON(e.HTTPStatus, gin.H{"error": e.Message, "code": e.Code})
}

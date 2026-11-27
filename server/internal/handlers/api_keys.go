package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	apikeysvc "github.com/serversupervisor/server/internal/services/apikey"
)

// APIKeyHandler translates HTTP to the API-key service. Every route it serves
// is closed to API-key authentication itself (see api.apiKeyForbiddenRoute):
// a key can neither mint nor revoke keys.
type APIKeyHandler struct {
	svc *apikeysvc.Service
}

func NewAPIKeyHandler(svc *apikeysvc.Service) *APIKeyHandler {
	return &APIKeyHandler{svc: svc}
}

// ListMine — GET /api/v1/auth/api-keys
func (h *APIKeyHandler) ListMine(c *gin.Context) {
	keys, err := h.svc.ListMine(c.Request.Context(), c.GetString("username"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

// Create — POST /api/v1/auth/api-keys. The response holds the full key once.
func (h *APIKeyHandler) Create(c *gin.Context) {
	var req models.APIKeyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request").I18n(apperr.CodeInvalidInput, nil))
		return
	}
	created, err := h.svc.Create(c.Request.Context(), c.GetString("username"), c.ClientIP(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, created)
}

// Revoke — DELETE /api/v1/auth/api-keys/:id. Owner, or any key for an admin.
func (h *APIKeyHandler) Revoke(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, apperr.Validation("invalid api key id").I18n(apperr.CodeInvalidInput, nil))
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), c.GetString("username"), c.GetString("role"), c.ClientIP(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListAll — GET /api/v1/api-keys (admin only, enforced by the router).
func (h *APIKeyHandler) ListAll(c *gin.Context) {
	keys, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

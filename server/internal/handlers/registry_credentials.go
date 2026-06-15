package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// Registry credentials are managed by admins and consumed only by the release
// tracker poller for private-image manifest polling. Passwords are write-only.

func (h *ReleaseTrackerHandler) ListRegistryCredentials(c *gin.Context) {
	creds, err := h.svc.ListRegistryCredentials(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"credentials": creds})
}

func (h *ReleaseTrackerHandler) CreateRegistryCredential(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req models.RegistryCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, err := h.svc.CreateRegistryCredential(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"credential": created})
}

func (h *ReleaseTrackerHandler) UpdateRegistryCredential(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req models.RegistryCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if err := h.svc.UpdateRegistryCredential(c.Request.Context(), c.Param("id"), req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ReleaseTrackerHandler) DeleteRegistryCredential(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	if err := h.svc.DeleteRegistryCredential(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

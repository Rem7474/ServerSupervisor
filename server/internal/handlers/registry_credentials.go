package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

// Registry credentials are managed by admins and consumed only by the release
// tracker poller for private-image manifest polling. Passwords are write-only:
// never returned in list/get responses.

func (h *ReleaseTrackerHandler) ListRegistryCredentials(c *gin.Context) {
	creds, err := h.db.ListRegistryCredentials(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list credentials"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.RegistryHost == "" || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, registry_host, username and password are required"})
		return
	}
	created, err := h.db.CreateRegistryCredential(c.Request.Context(), req.ToModel())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), fmt.Sprintf("CreateRegistryCredential: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create credential"})
		return
	}
	created.Password = "" // never echo the secret
	c.JSON(http.StatusCreated, gin.H{"credential": created})
}

func (h *ReleaseTrackerHandler) UpdateRegistryCredential(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")
	var req models.RegistryCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.RegistryHost == "" || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, registry_host and username are required"})
		return
	}
	if err := h.db.UpdateRegistryCredential(c.Request.Context(), id, req.ToModel()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update credential"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ReleaseTrackerHandler) DeleteRegistryCredential(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	if err := h.db.DeleteRegistryCredential(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete credential"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

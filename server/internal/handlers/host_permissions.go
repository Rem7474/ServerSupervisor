package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type HostPermissionHandler struct {
	db *database.DB
}

func NewHostPermissionHandler(db *database.DB) *HostPermissionHandler {
	return &HostPermissionHandler{db: db}
}

// ListHostPermissions returns all users who have explicit permissions on a host.
func (h *HostPermissionHandler) ListHostPermissions(c *gin.Context) {
	hostID := c.Param("id")
	perms, err := h.db.ListHostPermissions(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if perms == nil {
		perms = []models.HostPermission{}
	}
	c.JSON(http.StatusOK, perms)
}

// SetHostPermission grants or updates a user's access level to a host.
func (h *HostPermissionHandler) SetHostPermission(c *gin.Context) {
	hostID := c.Param("id")
	username := c.Param("username")

	var req struct {
		Level string `json:"level" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level requis"})
		return
	}
	if req.Level != "viewer" && req.Level != "operator" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level doit être 'viewer' ou 'operator'"})
		return
	}

	if err := h.db.SetHostPermission(username, hostID, req.Level); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": username, "host_id": hostID, "level": req.Level})
}

// DeleteHostPermission revokes a user's access to a host.
func (h *HostPermissionHandler) DeleteHostPermission(c *gin.Context) {
	hostID := c.Param("id")
	username := c.Param("username")

	if err := h.db.DeleteHostPermission(username, hostID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetMyHostPermissions returns the calling user's host permission entries.
func (h *HostPermissionHandler) GetMyHostPermissions(c *gin.Context) {
	username := c.GetString("username")
	perms, err := h.db.ListUserHostPermissions(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if perms == nil {
		perms = []models.HostPermission{}
	}
	c.JSON(http.StatusOK, perms)
}

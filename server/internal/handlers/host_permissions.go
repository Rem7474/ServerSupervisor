package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	hostpermsvc "github.com/serversupervisor/server/internal/services/hostperm"
)

// HostPermissionHandler translates HTTP to the host-permission service.
type HostPermissionHandler struct {
	svc *hostpermsvc.Service
}

func NewHostPermissionHandler(svc *hostpermsvc.Service) *HostPermissionHandler {
	return &HostPermissionHandler{svc: svc}
}

// ListHostPermissions returns all users who have explicit permissions on a host.
func (h *HostPermissionHandler) ListHostPermissions(c *gin.Context) {
	perms, err := h.svc.List(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
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
		respondError(c, apperr.Validation("level requis"))
		return
	}
	if err := h.svc.Set(c.Request.Context(), username, hostID, req.Level); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": username, "host_id": hostID, "level": req.Level})
}

// DeleteHostPermission revokes a user's access to a host.
func (h *HostPermissionHandler) DeleteHostPermission(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("username"), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetMyHostPermissions returns the calling user's host permission entries.
func (h *HostPermissionHandler) GetMyHostPermissions(c *gin.Context) {
	perms, err := h.svc.ListForUser(c.Request.Context(), c.GetString("username"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, perms)
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	maintenancesvc "github.com/serversupervisor/server/internal/services/maintenance"
)

// MaintenanceWindowHandler translates HTTP to the maintenance-window
// service. It keeps a db reference solely for requireHostAccess in
// DeleteMaintenanceWindow — HTTP-level host authorization for a resource
// whose :id is the window id, not the host id — all other logic lives in
// internal/services/maintenance.
type MaintenanceWindowHandler struct {
	svc *maintenancesvc.Service
	db  *database.DB
}

func NewMaintenanceWindowHandler(svc *maintenancesvc.Service, db *database.DB) *MaintenanceWindowHandler {
	return &MaintenanceWindowHandler{svc: svc, db: db}
}

// ListAllMaintenanceWindows returns every maintenance window (global view).
func (h *MaintenanceWindowHandler) ListAllMaintenanceWindows(c *gin.Context) {
	windows, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, windows)
}

// ListMaintenanceWindowsForHost returns the windows applicable to a host.
func (h *MaintenanceWindowHandler) ListMaintenanceWindowsForHost(c *gin.Context) {
	windows, err := h.svc.ListForHost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, windows)
}

// CreateMaintenanceWindow creates a window scoped to a single host. Requires
// Operator+ on that host — same shape as CreateScheduledTask.
func (h *MaintenanceWindowHandler) CreateMaintenanceWindow(c *gin.Context) {
	if !requireHostAccess(c, h.db, c.Param("id"), "operator") {
		return
	}
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}
	var req models.MaintenanceWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, err := h.svc.CreateForHost(c.Request.Context(), c.Param("id"), username, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// CreateGlobalMaintenanceWindow creates a window applying to every host.
// Admin-only — a global window has the largest possible blast radius (it
// silences every alert in the system at once), the same reasoning as
// runbooks' admin-only gating (see root CLAUDE.md's Runbooks section).
func (h *MaintenanceWindowHandler) CreateGlobalMaintenanceWindow(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}
	var req models.MaintenanceWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, err := h.svc.CreateGlobal(c.Request.Context(), username, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// DeleteMaintenanceWindow removes a window. Requires Operator+ on the
// window's host for a host-scoped window (resolved first, since :id is the
// window id, not the host id); admin-only for a global window (HostID nil).
func (h *MaintenanceWindowHandler) DeleteMaintenanceWindow(c *gin.Context) {
	w, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if w.HostID == nil {
		if c.GetString("role") != models.RoleAdmin {
			respondError(c, apperr.Forbidden("admin required to delete a global maintenance window"))
			return
		}
	} else if !requireHostAccess(c, h.db, *w.HostID, "operator") {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

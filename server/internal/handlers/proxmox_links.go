package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

// ─── Guest ↔ Host links ───────────────────────────────────────────────────────

// ListLinks returns all guest-host links, optionally filtered by ?status=suggested|confirmed|ignored.
func (h *ProxmoxHandler) ListLinks(c *gin.Context) {
	links, err := h.db.ListProxmoxGuestLinks(c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, links)
}

// CreateLink creates or replaces a guest-host link (upserts on guest_id).
func (h *ProxmoxHandler) CreateLink(c *gin.Context) {
	var req models.ProxmoxGuestLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Status == "" {
		req.Status = "confirmed"
	}
	if req.MetricsSource == "" {
		req.MetricsSource = "auto"
	}
	link, err := h.db.UpsertProxmoxGuestLink(req.GuestID, req.HostID, req.Status, req.MetricsSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, link)
}

// GetLink returns a single link by its ID.
func (h *ProxmoxHandler) GetLink(c *gin.Context) {
	link, err := h.db.GetProxmoxGuestLink(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if link == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	c.JSON(http.StatusOK, link)
}

// UpdateLink updates status and/or metrics_source for a link.
func (h *ProxmoxHandler) UpdateLink(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxGuestLink(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}

	var req models.ProxmoxGuestLinkUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validStatuses := map[string]bool{"suggested": true, "confirmed": true, "ignored": true}
	validMetricsSources := map[string]bool{"auto": true, "agent": true, "proxmox": true}
	if req.Status != nil && !validStatuses[*req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status invalide : doit être suggested, confirmed ou ignored"})
		return
	}
	if req.MetricsSource != nil && !validMetricsSources[*req.MetricsSource] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metrics_source invalide : doit être auto, agent ou proxmox"})
		return
	}

	link, err := h.db.UpdateProxmoxGuestLink(id, req.Status, req.MetricsSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link)
}

// DeleteLink removes a guest-host link.
func (h *ProxmoxHandler) DeleteLink(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxGuestLink(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err := h.db.DeleteProxmoxGuestLink(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link deleted"})
}

// GetLinkByGuest returns the link for a specific Proxmox guest, or null when none exists.
// Returns 200 in both cases to avoid spurious 404s in the browser console.
func (h *ProxmoxHandler) GetLinkByGuest(c *gin.Context) {
	link, err := h.db.GetProxmoxGuestLinkByGuest(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link) // nil marshals to JSON null
}

// GetLinkByHost returns the confirmed/suggested Proxmox link for a host, or null when none exists.
// Returns 200 in both cases to avoid spurious 404s in the browser console.
func (h *ProxmoxHandler) GetLinkByHost(c *gin.Context) {
	link, err := h.db.GetProxmoxGuestLinkByHost(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link) // nil marshals to JSON null
}

// ListLinkCandidates returns Proxmox guests that could be linked to a host,
// ordered by name similarity. Used for the manual-link dropdown.
func (h *ProxmoxHandler) ListLinkCandidates(c *gin.Context) {
	hostID := c.Param("id")
	candidates, err := h.db.ListProxmoxLinkCandidates(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, candidates)
}

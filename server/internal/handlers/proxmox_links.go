package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// ─── Guest ↔ Host links ───────────────────────────────────────────────────────

// ListLinks returns all guest-host links, optionally filtered by ?status=.
func (h *ProxmoxHandler) ListLinks(c *gin.Context) {
	links, err := h.svc.ListLinks(c.Request.Context(), c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, links)
}

// CreateLink creates or replaces a guest-host link (upserts on guest_id).
func (h *ProxmoxHandler) CreateLink(c *gin.Context) {
	var req models.ProxmoxGuestLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	link, err := h.svc.CreateLink(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, link)
}

// GetLink returns a single link by its ID.
func (h *ProxmoxHandler) GetLink(c *gin.Context) {
	link, err := h.svc.GetLink(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

// UpdateLink updates status and/or metrics_source for a link.
func (h *ProxmoxHandler) UpdateLink(c *gin.Context) {
	var req models.ProxmoxGuestLinkUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	link, err := h.svc.UpdateLink(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

// DeleteLink removes a guest-host link.
func (h *ProxmoxHandler) DeleteLink(c *gin.Context) {
	if err := h.svc.DeleteLink(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link deleted"})
}

// GetLinkByGuest returns the link for a Proxmox guest, or null when none exists (200).
func (h *ProxmoxHandler) GetLinkByGuest(c *gin.Context) {
	link, err := h.svc.LinkByGuest(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, link) // nil marshals to JSON null
}

// GetLinkByHost returns the Proxmox link for a host, or null when none exists (200).
func (h *ProxmoxHandler) GetLinkByHost(c *gin.Context) {
	link, err := h.svc.LinkByHost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, link) // nil marshals to JSON null
}

// GetHostProxmoxDisks returns the physical disks of the Proxmox node hosting the
// guest linked to this host.
func (h *ProxmoxHandler) GetHostProxmoxDisks(c *gin.Context) {
	disks, err := h.svc.HostProxmoxDisks(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, disks)
}

// ListLinkCandidates returns Proxmox guests that could be linked to a host.
func (h *ProxmoxHandler) ListLinkCandidates(c *gin.Context) {
	candidates, err := h.svc.LinkCandidates(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, candidates)
}

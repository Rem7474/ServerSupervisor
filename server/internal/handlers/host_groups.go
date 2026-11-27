package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	hostgroupsvc "github.com/serversupervisor/server/internal/services/hostgroup"
)

// HostGroupsHandler translates HTTP to the host-group service. Admin-only
// (router group): a group can widen or narrow the coverage of every alert
// rule that targets it.
type HostGroupsHandler struct {
	svc *hostgroupsvc.Service
}

func NewHostGroupsHandler(svc *hostgroupsvc.Service) *HostGroupsHandler {
	return &HostGroupsHandler{svc: svc}
}

func (h *HostGroupsHandler) List(c *gin.Context) {
	groups, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *HostGroupsHandler) Create(c *gin.Context) {
	var req models.HostGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	g, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (h *HostGroupsHandler) Update(c *gin.Context) {
	var req models.HostGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	g, err := h.svc.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *HostGroupsHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	discoverysvc "github.com/serversupervisor/server/internal/services/discovery"
)

// DiscoveryHandler translates HTTP to the discovery services: the server-side
// ping sweep and the agent-run probes (ARP, port scan, SNMP).
type DiscoveryHandler struct {
	svc   *discoverysvc.Service
	agent *discoverysvc.AgentService
}

func NewDiscoveryHandler(svc *discoverysvc.Service, agent *discoverysvc.AgentService) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc, agent: agent}
}

// requireDiscoveryAdmin guards every discovery endpoint: probing a network is an
// onboarding action, restricted like host registration.
func requireDiscoveryAdmin(c *gin.Context) bool {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return false
	}
	return true
}

func discoveryActor(c *gin.Context) discoverysvc.Actor {
	return discoverysvc.Actor{Username: c.GetString("username"), IPAddress: c.ClientIP()}
}

// Scan ping-sweeps a subnet and reports which addresses answered.
func (h *DiscoveryHandler) Scan(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	var req models.NetworkScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	results, err := h.svc.Scan(c.Request.Context(), req.CIDR)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// Limits returns the bounds of agent-run discovery (default ports, maximum
// port count, narrowest prefix, allowed subnets).
func (h *DiscoveryHandler) Limits(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	c.JSON(http.StatusOK, h.agent.Limits())
}

func (h *DiscoveryHandler) started(c *gin.Context, id string, err error) {
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, models.DiscoveryJobStarted{JobID: id})
}

// StartARP asks an agent for its ARP/neighbour table.
func (h *DiscoveryHandler) StartARP(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	var req models.DiscoveryAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	id, err := h.agent.StartARP(c.Request.Context(), discoveryActor(c), req.HostID)
	h.started(c, id, err)
}

// StartPortScan queues a bounded TCP connect scan on an agent.
func (h *DiscoveryHandler) StartPortScan(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	var req models.PortScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	id, err := h.agent.StartPortScan(c.Request.Context(), discoveryActor(c), req)
	h.started(c, id, err)
}

// StartSNMP queues an SNMP GET from an agent. The credentials in the body
// are never echoed back, stored or logged.
func (h *DiscoveryHandler) StartSNMP(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	var req models.SNMPProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request body"))
		return
	}
	id, err := h.agent.StartSNMP(c.Request.Context(), discoveryActor(c), req)
	h.started(c, id, err)
}

// GetJob returns a discovery job's state and result.
func (h *DiscoveryHandler) GetJob(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	job, err := h.agent.Job(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

// CancelJob stops a pending or running discovery job.
func (h *DiscoveryHandler) CancelJob(c *gin.Context) {
	if !requireDiscoveryAdmin(c) {
		return
	}
	job, err := h.agent.Cancel(c.Request.Context(), discoveryActor(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

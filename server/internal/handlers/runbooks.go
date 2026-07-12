package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	runbooksvc "github.com/serversupervisor/server/internal/services/runbook"
)

// RunbooksHandler translates HTTP to the runbook service. The whole domain
// is admin-only in this MVP: a runbook can name any host and chain several
// already-whitelisted actions across the fleet in one go, which is a bigger
// blast radius than a single scheduled task — see internal/services/runbook
// for the execution model.
type RunbooksHandler struct {
	svc *runbooksvc.Service
}

func NewRunbooksHandler(svc *runbooksvc.Service) *RunbooksHandler {
	return &RunbooksHandler{svc: svc}
}

func (h *RunbooksHandler) ListRunbooks(c *gin.Context) {
	runbooks, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, runbooks)
}

func (h *RunbooksHandler) GetRunbook(c *gin.Context) {
	rb, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rb)
}

func (h *RunbooksHandler) CreateRunbook(c *gin.Context) {
	var req models.RunbookCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	rb, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rb)
}

func (h *RunbooksHandler) UpdateRunbook(c *gin.Context) {
	var req models.RunbookUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	if err := h.svc.Update(c.Request.Context(), c.Param("id"), req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Runbook mis à jour"})
}

func (h *RunbooksHandler) DeleteRunbook(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Runbook supprimé"})
}

func (h *RunbooksHandler) RunRunbook(c *gin.Context) {
	exec, err := h.svc.Run(c.Request.Context(), c.Param("id"), c.GetString("username"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, exec)
}

func (h *RunbooksHandler) ListRunbookExecutions(c *gin.Context) {
	limit := clampQueryInt(c, "limit", 20, 100)
	execs, err := h.svc.ListExecutions(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, execs)
}

func (h *RunbooksHandler) GetRunbookExecution(c *gin.Context) {
	exec, err := h.svc.GetExecution(c.Request.Context(), c.Param("execution_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if exec.RunbookID != c.Param("id") {
		respondError(c, apperr.NotFound("Exécution introuvable."))
		return
	}
	c.JSON(http.StatusOK, exec)
}

// HandleCommandCompletion implements agentsvc.CommandCompletionListener.
func (h *RunbooksHandler) HandleCommandCompletion(commandID, status string) {
	h.svc.NotifyComplete(context.Background(), commandID, status)
}

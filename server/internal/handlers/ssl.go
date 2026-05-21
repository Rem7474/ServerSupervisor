package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/synthetic"
)

type SSLHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewSSLHandler(db *database.DB, cfg *config.Config) *SSLHandler {
	return &SSLHandler{db: db, cfg: cfg}
}

type sslCertPayload struct {
	Name       string `json:"name" binding:"required"`
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port"`
	ServerName string `json:"server_name"`
	Enabled    *bool  `json:"enabled"`
}

func (p sslCertPayload) toModel() models.SSLCertificate {
	m := models.SSLCertificate{
		Name:       strings.TrimSpace(p.Name),
		Host:       strings.TrimSpace(p.Host),
		Port:       p.Port,
		ServerName: strings.TrimSpace(p.ServerName),
		Enabled:    true,
	}
	if m.Port == 0 {
		m.Port = 443
	}
	if p.Enabled != nil {
		m.Enabled = *p.Enabled
	}
	return m
}

func (h *SSLHandler) List(c *gin.Context) {
	certs, err := h.db.ListSSLCertificates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if certs == nil {
		certs = []models.SSLCertificate{}
	}
	c.JSON(http.StatusOK, gin.H{"certificates": certs})
}

func (h *SSLHandler) Get(c *gin.Context) {
	cert, err := h.db.GetSSLCertificate(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cert)
}

func (h *SSLHandler) Create(c *gin.Context) {
	var req sslCertPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.db.CreateSSLCertificate(c.Request.Context(), req.toModel())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *SSLHandler) Update(c *gin.Context) {
	var req sslCertPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m := req.toModel()
	m.ID = c.Param("id")
	if err := h.db.UpdateSSLCertificate(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := h.db.GetSSLCertificate(c.Request.Context(), m.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *SSLHandler) Delete(c *gin.Context) {
	if err := h.db.DeleteSSLCertificate(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// CheckNow runs a TLS handshake immediately, persists the result, returns the fresh record.
func (h *SSLHandler) CheckNow(c *gin.Context) {
	cert, err := h.db.GetSSLCertificate(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated := synthetic.CheckCertificate(context.Background(), *cert)
	if err := h.db.UpdateSSLCertificateCheckResult(context.Background(), updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := h.db.GetSSLCertificate(c.Request.Context(), updated.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

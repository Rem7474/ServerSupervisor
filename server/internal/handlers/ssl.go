package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	sslsvc "github.com/serversupervisor/server/internal/services/ssl"
)

// SSLHandler translates HTTP to the SSL service; all certificate logic lives in
// the service layer (internal/services/ssl) and error semantics are carried by
// typed apperr values rendered uniformly via respondError.
type SSLHandler struct {
	svc *sslsvc.Service
}

func NewSSLHandler(svc *sslsvc.Service) *SSLHandler {
	return &SSLHandler{svc: svc}
}

func (h *SSLHandler) List(c *gin.Context) {
	certs, err := h.svc.ListCerts(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"certificates": certs})
}

func (h *SSLHandler) Get(c *gin.Context) {
	cert, err := h.svc.GetCert(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cert)
}

func (h *SSLHandler) Create(c *gin.Context) {
	var req models.SSLCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	out, err := h.svc.CreateCert(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *SSLHandler) Update(c *gin.Context) {
	var req models.SSLCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	out, err := h.svc.UpdateCert(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *SSLHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteCert(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CheckNow runs a TLS handshake immediately, persists the result, returns the fresh record.
func (h *SSLHandler) CheckNow(c *gin.Context) {
	out, err := h.svc.CheckNow(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/cookies"
	oidcsvc "github.com/serversupervisor/server/internal/services/oidc"
)

// OIDCHandler translates HTTP requests to the OIDC service layer and sets session cookies on callback.
type OIDCHandler struct {
	svc *oidcsvc.Service
	cfg *config.Config
}

// NewOIDCHandler returns a new OIDCHandler instance.
func NewOIDCHandler(svc *oidcsvc.Service, cfg *config.Config) *OIDCHandler {
	return &OIDCHandler{
		svc: svc,
		cfg: cfg,
	}
}

// GetStatus returns the public OIDC configuration status (whether SSO is enabled, provider name, local login allowed).
func (h *OIDCHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Status())
}

// Login initiates the OIDC authorization ceremony with PKCE and redirects the user to the IdP.
func (h *OIDCHandler) Login(c *gin.Context) {
	returnURL := c.Query("return_to")
	if returnURL == "" {
		returnURL = c.Query("redirect")
	}

	authURL, err := h.svc.BeginAuth(c.Request.Context(), returnURL)
	if err != nil {
		respondError(c, err)
		return
	}

	// If requested via JSON API (e.g. SPA fetch), return JSON payload; otherwise HTTP 302 redirect.
	if strings.Contains(c.GetHeader("Accept"), "application/json") && !strings.Contains(c.GetHeader("Accept"), "text/html") {
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// Callback handles the authorization code redirect from the Identity Provider.
func (h *OIDCHandler) Callback(c *gin.Context) {
	// Check for error parameters returned directly by the IdP
	if idpError := c.Query("error"); idpError != "" {
		errDesc := c.Query("error_description")
		if errDesc == "" {
			errDesc = idpError
		}
		target := "/login?error=" + url.QueryEscape("sso_error: "+errDesc)
		c.Redirect(http.StatusFound, target)
		return
	}

	state := c.Query("state")
	code := c.Query("code")

	if state == "" || code == "" {
		c.Redirect(http.StatusFound, "/login?error="+url.QueryEscape("missing authorization code or state"))
		return
	}

	_, tokens, redirectURL, err := h.svc.CompleteAuth(
		c.Request.Context(),
		state,
		code,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		appErr := apperr.From(err)
		c.Redirect(http.StatusFound, "/login?error="+url.QueryEscape(appErr.Message))
		return
	}

	// Write standard session cookies (ss_access, ss_refresh, ss_csrf)
	cookies.SetAccess(c, h.cfg, tokens.AccessToken, tokens.AccessExpiresAt, tokens.CSRFToken)
	cookies.SetRefresh(c, h.cfg, tokens.RefreshToken, tokens.RefreshExpiresAt)

	target := redirectURL
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		target = "/"
	}

	c.Redirect(http.StatusFound, target)
}

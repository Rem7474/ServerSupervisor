package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// respondError renders any error as the uniform JSON envelope
// `{"error": "<message>", "code": "<machine-code>"}` with the status carried by
// the typed apperr.Error (unknown errors become a 500 "internal" error). The
// `error` string preserves the historical shape so existing frontend consumers
// keep working; `code` is the machine-readable discriminator. When the error
// carries an I18nKey (see apperr.Error.I18n), `error` is instead the message
// resolved from apperr.ErrorCatalog in the caller's language (Accept-Language),
// and the response gains `i18nKey`/`params` so a frontend that already has its
// own translation for that key can render it directly instead of trusting the
// server's text — this is additive, so a not-yet-migrated call site (no
// I18nKey set) renders exactly as before.
func respondError(c *gin.Context, err error) {
	e := apperr.From(err)
	body := gin.H{"error": e.Message, "code": e.Code}
	if e.I18nKey != "" {
		lang := apperr.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		body["error"] = apperr.GetMessage(e.I18nKey, lang, e.Params)
		body["i18nKey"] = e.I18nKey
		if len(e.Params) > 0 {
			body["params"] = e.Params
		}
	}
	c.JSON(e.HTTPStatus, body)
}

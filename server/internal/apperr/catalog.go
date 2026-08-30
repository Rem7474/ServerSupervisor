package apperr

import "strings"

// Error codes — stable identifiers for ErrorCatalog, independent of the
// coarse HTTP-category Code above (e.g. "not_found"). A handler/service
// attaches one via Error.I18n so respondError can render it in the
// caller's language; Message stays the English/dev-facing fallback.
const (
	// Authentication & Authorization
	CodeAdminRequired    = "ADMIN_REQUIRED"
	CodeAuthRequired     = "AUTH_REQUIRED"
	CodeHostAccessDenied = "HOST_ACCESS_DENIED"
	CodeOperatorRequired = "OPERATOR_REQUIRED"
	CodePermissionDenied = "PERMISSION_DENIED"
	CodeInvalidToken     = "INVALID_TOKEN"
	CodeTokenExpired     = "TOKEN_EXPIRED"

	// Validation
	CodeInvalidInput     = "INVALID_INPUT"
	CodeInvalidTimeframe = "INVALID_TIMEFRAME"
	CodeMissingField     = "MISSING_FIELD"
	CodeMissingParameter = "MISSING_PARAMETER"

	// Resource
	CodeNotFound     = "NOT_FOUND"
	CodeNodeNotFound = "NODE_NOT_FOUND"
	CodeConflict     = "CONFLICT"

	// Server
	CodeInternalError    = "INTERNAL_ERROR"
	CodePermissionFailed = "PERMISSION_FAILED"
	CodeProxmoxError     = "PROXMOX_ERROR"

	// Notifications
	CodeInvalidMetric   = "INVALID_METRIC"
	CodeInvalidOperator = "INVALID_OPERATOR"

	// Git release providers (internal/gitprovider)
	CodeGitProviderUnauthorized = "GIT_PROVIDER_UNAUTHORIZED"
	CodeGitProviderRateLimited  = "GIT_PROVIDER_RATE_LIMITED"
	CodeGitProviderNotFound     = "GIT_PROVIDER_NOT_FOUND"
	CodeGitProviderError        = "GIT_PROVIDER_ERROR"
)

// ErrorMessage holds translated strings for a catalog entry. {name}
// placeholders are substituted from the Params passed to GetMessage.
type ErrorMessage struct {
	EN string
	FR string
}

// ErrorCatalog maps stable i18n keys to translated messages.
var ErrorCatalog = map[string]ErrorMessage{
	CodeAdminRequired: {
		EN: "admin access required",
		FR: "accès administrateur requis",
	},
	CodeAuthRequired: {
		EN: "authentication required",
		FR: "authentification requise",
	},
	CodeHostAccessDenied: {
		EN: "access denied to this host",
		FR: "accès refusé à cet hôte",
	},
	CodeOperatorRequired: {
		EN: "operator rights required on this host",
		FR: "droits opérateur requis sur cet hôte",
	},
	CodePermissionDenied: {
		EN: "permission denied",
		FR: "accès refusé",
	},
	CodeInvalidToken: {
		EN: "invalid token",
		FR: "jeton invalide",
	},
	CodeTokenExpired: {
		EN: "token expired",
		FR: "jeton expiré",
	},
	CodeInvalidInput: {
		EN: "invalid input",
		FR: "entrée invalide",
	},
	CodeInvalidTimeframe: {
		EN: "invalid timeframe; allowed: hour day week month year",
		FR: "période invalide ; autorisées : hour day week month year",
	},
	CodeMissingField: {
		EN: "missing required field",
		FR: "champ obligatoire manquant",
	},
	CodeMissingParameter: {
		EN: "missing required parameter",
		FR: "paramètre obligatoire manquant",
	},
	CodeNotFound: {
		EN: "not found",
		FR: "non trouvé",
	},
	CodeNodeNotFound: {
		EN: "node not found",
		FR: "nœud non trouvé",
	},
	CodeConflict: {
		EN: "conflict",
		FR: "conflit",
	},
	CodeInternalError: {
		EN: "internal server error",
		FR: "erreur interne du serveur",
	},
	CodePermissionFailed: {
		EN: "permission check failed",
		FR: "vérification des permissions échouée",
	},
	CodeProxmoxError: {
		EN: "proxmox operation failed",
		FR: "opération Proxmox échouée",
	},
	CodeInvalidMetric: {
		EN: "invalid metric",
		FR: "métrique invalide",
	},
	CodeInvalidOperator: {
		EN: "invalid operator",
		FR: "opérateur invalide",
	},
	CodeGitProviderUnauthorized: {
		EN: "invalid or expired GitHub token (401) — check GITHUB_TOKEN in settings",
		FR: "token GitHub invalide ou expiré (401) — vérifiez GITHUB_TOKEN dans les paramètres",
	},
	CodeGitProviderRateLimited: {
		EN: "GitHub rate limit reached (403) — configure a GITHUB_TOKEN to raise the limit",
		FR: "limite de taux GitHub atteinte (403) — configurez un GITHUB_TOKEN pour augmenter la limite",
	},
	CodeGitProviderNotFound: {
		EN: "repository not found on GitHub (404) — check owner/repo",
		FR: "dépôt introuvable sur GitHub (404) — vérifiez owner/repo",
	},
	CodeGitProviderError: {
		EN: "GitHub API error ({status})",
		FR: "erreur GitHub API ({status})",
	},
}

// ErrorResponse is the standalone error-response shape used by call sites
// that render an error directly (outside the apperr.Error/respondError
// path), e.g. request-validation checks that fail before a service call.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// NewErrorResponse builds an ErrorResponse from an i18n key and language.
func NewErrorResponse(errorCode, lang string, params map[string]string) ErrorResponse {
	return ErrorResponse{
		Error: GetMessage(errorCode, lang, params),
		Code:  errorCode,
	}
}

// GetMessage returns the translated message for a catalog key, with any
// {name} placeholders substituted from params. Falls back to English if lang
// isn't "fr"; falls back to the bare code if it isn't in the catalog.
func GetMessage(errorCode, lang string, params map[string]string) string {
	msg, found := ErrorCatalog[errorCode]
	if !found {
		return "error: " + errorCode
	}

	text := msg.EN
	if strings.ToLower(lang) == "fr" {
		text = msg.FR
	}
	for name, value := range params {
		text = strings.ReplaceAll(text, "{"+name+"}", value)
	}
	return text
}

// GetLanguageFromAcceptLanguage extracts a supported language ("fr" or "en")
// from an Accept-Language header, e.g. "fr-FR,fr;q=0.9,en;q=0.8" → "fr".
// Defaults to "en" if not found, empty, or unsupported.
func GetLanguageFromAcceptLanguage(acceptLanguage string) string {
	if acceptLanguage == "" {
		return "en"
	}

	parts := strings.Split(acceptLanguage, ",")
	if len(parts) == 0 {
		return "en"
	}

	first := strings.TrimSpace(parts[0])
	if idx := strings.Index(first, "-"); idx > 0 {
		first = first[:idx]
	}
	if idx := strings.Index(first, ";"); idx > 0 {
		first = first[:idx]
	}

	if strings.ToLower(strings.TrimSpace(first)) == "fr" {
		return "fr"
	}
	return "en"
}

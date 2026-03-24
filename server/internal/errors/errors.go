package errors

import "strings"

// Error codes — used to identify error types consistently
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
)

// ErrorMessage holds translated strings for errors
type ErrorMessage struct {
	EN string // English message
	FR string // French message
}

// ErrorCatalog maps error codes to translated messages
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
}

// ErrorResponse is the standard error response structure
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"` // Optional: error code for client-side handling
}

// NewErrorResponse creates an ErrorResponse from error code and language
func NewErrorResponse(errorCode, lang string) ErrorResponse {
	return ErrorResponse{
		Error: GetMessage(errorCode, lang),
		Code:  errorCode,
	}
}

// GetMessage returns translated error message for given code and language
// Defaults to English if language is not "fr" or message not found
func GetMessage(errorCode, lang string) string {
	msg, found := ErrorCatalog[errorCode]
	if !found {
		return "error: " + errorCode
	}

	// Normalize language to lowercase
	lang = strings.ToLower(lang)
	if lang == "fr" {
		return msg.FR
	}
	return msg.EN
}

// GetMessageFromHeader extracts language from Accept-Language header
// Format: "fr-FR,fr;q=0.9,en;q=0.8" returns "fr"
// Defaults to "en" if not found or empty
func GetLanguageFromAcceptLanguage(acceptLanguage string) string {
	if acceptLanguage == "" {
		return "en"
	}

	// Split by comma and get first component
	parts := strings.Split(acceptLanguage, ",")
	if len(parts) == 0 {
		return "en"
	}

	// Get language code (e.g., "fr" from "fr-FR" or "fr;q=0.9")
	first := strings.TrimSpace(parts[0])
	if idx := strings.Index(first, "-"); idx > 0 {
		first = first[:idx]
	}
	if idx := strings.Index(first, ";"); idx > 0 {
		first = first[:idx]
	}

	lang := strings.ToLower(strings.TrimSpace(first))
	if lang == "fr" {
		return "fr"
	}
	return "en"
}

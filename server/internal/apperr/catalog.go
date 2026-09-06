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

	// Alert rules (internal/services/alertrule)
	CodeAlertRuleNotFound                = "ALERT_RULE_NOT_FOUND"
	CodeAlertSourceTypeImmutable         = "ALERT_SOURCE_TYPE_IMMUTABLE"
	CodeAlertIncidentResolveFailed       = "ALERT_INCIDENT_RESOLVE_FAILED"
	CodeAlertDockerScopeRequired         = "ALERT_DOCKER_SCOPE_REQUIRED"
	CodeAlertDockerHostNotFound          = "ALERT_DOCKER_HOST_NOT_FOUND"
	CodeAlertDockerContainerNotFound     = "ALERT_DOCKER_CONTAINER_NOT_FOUND"
	CodeAlertComposeProjectNotFound      = "ALERT_COMPOSE_PROJECT_NOT_FOUND"
	CodeAlertProxmoxScopeRequired        = "ALERT_PROXMOX_SCOPE_REQUIRED"
	CodeAlertProxmoxConnNotFound         = "ALERT_PROXMOX_CONNECTION_NOT_FOUND"
	CodeAlertProxmoxNodeNotFound         = "ALERT_PROXMOX_NODE_NOT_FOUND"
	CodeAlertProxmoxStorageNotFound      = "ALERT_PROXMOX_STORAGE_NOT_FOUND"
	CodeAlertProxmoxGuestNotFound        = "ALERT_PROXMOX_GUEST_NOT_FOUND"
	CodeAlertProxmoxDiskNotFound         = "ALERT_PROXMOX_DISK_NOT_FOUND"
	CodeAlertRollingWindowNotAllowed     = "ALERT_ROLLING_WINDOW_NOT_ALLOWED"
	CodeAlertRollingWindowInvalid        = "ALERT_ROLLING_WINDOW_INVALID"
	CodeAlertCooldownNegative            = "ALERT_COOLDOWN_NEGATIVE"
	CodeAlertEscalationNegative          = "ALERT_ESCALATION_NEGATIVE"
	CodeAlertCommandTriggerIncomplete    = "ALERT_COMMAND_TRIGGER_INCOMPLETE"
	CodeAlertMetricUnsupportedForLogs    = "ALERT_METRIC_UNSUPPORTED_FOR_LOGS"
	CodeAlertMetricRequiresProxmox       = "ALERT_METRIC_REQUIRES_PROXMOX"
	CodeAlertTemplateMetricNotHostScoped = "ALERT_TEMPLATE_METRIC_NOT_HOST_SCOPED"
	CodeAlertTemplateNotFound            = "ALERT_TEMPLATE_NOT_FOUND"

	// Runbooks (internal/services/runbook)
	CodeRunbookNotFound          = "RUNBOOK_NOT_FOUND"
	CodeRunbookNameRequired      = "RUNBOOK_NAME_REQUIRED"
	CodeRunbookCreateFailed      = "RUNBOOK_CREATE_FAILED"
	CodeRunbookUpdateFailed      = "RUNBOOK_UPDATE_FAILED"
	CodeRunbookNoSteps           = "RUNBOOK_NO_STEPS"
	CodeRunbookRunFailed         = "RUNBOOK_RUN_FAILED"
	CodeRunbookExecutionNotFound = "RUNBOOK_EXECUTION_NOT_FOUND"
	CodeRunbookStepsRequired     = "RUNBOOK_STEPS_REQUIRED"
	CodeRunbookStepsInvalid      = "RUNBOOK_STEPS_INVALID"
	CodeAlertRuleIDInvalid       = "ALERT_RULE_ID_INVALID"
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

	CodeAlertRuleNotFound:             {EN: "alert rule not found", FR: "règle d'alerte introuvable"},
	CodeAlertSourceTypeImmutable:      {EN: "changing source_type is not allowed", FR: "le changement de source_type n'est pas autorisé"},
	CodeAlertIncidentResolveFailed:    {EN: "rule updated, but resolving its open incidents failed", FR: "règle mise à jour, mais échec de résolution des incidents ouverts"},
	CodeAlertDockerScopeRequired:      {EN: "a Docker scope is required", FR: "le scope Docker est requis"},
	CodeAlertDockerHostNotFound:       {EN: "host not found for this Docker scope", FR: "hôte introuvable pour ce scope Docker"},
	CodeAlertDockerContainerNotFound:  {EN: "Docker container not found for this scope", FR: "conteneur Docker introuvable pour ce scope"},
	CodeAlertComposeProjectNotFound:   {EN: "Compose project not found for this scope", FR: "projet Compose introuvable pour ce scope"},
	CodeAlertProxmoxScopeRequired:     {EN: "a Proxmox scope is required", FR: "le scope Proxmox est requis"},
	CodeAlertProxmoxConnNotFound:      {EN: "Proxmox connection not found for this scope", FR: "connexion Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxNodeNotFound:      {EN: "Proxmox node not found for this scope", FR: "nœud Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxStorageNotFound:   {EN: "Proxmox storage not found for this scope", FR: "stockage Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxGuestNotFound:     {EN: "Proxmox VM/LXC not found for this scope", FR: "VM/LXC Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxDiskNotFound:      {EN: "Proxmox physical disk not found for this scope", FR: "disque physique Proxmox introuvable pour ce scope"},
	CodeAlertRollingWindowNotAllowed:  {EN: "the rolling-average window does not apply to this metric", FR: "la fenêtre de moyenne glissante n'est pas applicable à cette métrique"},
	CodeAlertRollingWindowInvalid:     {EN: "invalid rolling-average window (allowed: 1h, 6h, 24h)", FR: "fenêtre de moyenne glissante invalide (valeurs autorisées : 1h, 6h, 24h)"},
	CodeAlertCooldownNegative:         {EN: "the silence period must be zero or positive", FR: "la période de silence doit être positive ou nulle"},
	CodeAlertEscalationNegative:       {EN: "the escalation delay must be zero or positive", FR: "le délai d'escalade doit être positif ou nul"},
	CodeAlertCommandTriggerIncomplete: {EN: "a command trigger must define both a module and an action", FR: "le déclencheur de commande doit définir un module et une action"},
	CodeAlertMetricUnsupportedForLogs: {EN: "metric not supported for logs", FR: "métrique non supportée pour les logs"},
	CodeAlertMetricRequiresProxmox:    {EN: "this metric requires a Proxmox source", FR: "la métrique requiert une source Proxmox"},
	CodeAlertTemplateMetricNotHostScoped: {
		EN: "Docker, Proxmox and synthetic metrics cannot be used in a template — they do not apply host by host",
		FR: "les métriques Docker, Proxmox ou synthétiques ne peuvent pas être utilisées dans un template — elles ne s'appliquent pas hôte par hôte",
	},
	CodeAlertTemplateNotFound: {EN: "template not found", FR: "modèle introuvable"},

	CodeRunbookNotFound:          {EN: "runbook not found", FR: "runbook introuvable"},
	CodeRunbookNameRequired:      {EN: "the runbook name is required", FR: "le nom du runbook est requis"},
	CodeRunbookCreateFailed:      {EN: "could not create the runbook", FR: "erreur lors de la création du runbook"},
	CodeRunbookUpdateFailed:      {EN: "could not update the runbook", FR: "erreur lors de la mise à jour du runbook"},
	CodeRunbookNoSteps:           {EN: "this runbook has no steps", FR: "ce runbook n'a aucune étape"},
	CodeRunbookRunFailed:         {EN: "could not start the runbook", FR: "erreur lors du lancement du runbook"},
	CodeRunbookExecutionNotFound: {EN: "execution not found", FR: "exécution introuvable"},
	CodeRunbookStepsRequired:     {EN: "a runbook must have at least one step", FR: "le runbook doit avoir au moins une étape"},
	CodeRunbookStepsInvalid:      {EN: "could not validate the steps", FR: "erreur lors de la validation des étapes"},
	CodeAlertRuleIDInvalid:       {EN: "invalid alert rule id", FR: "identifiant de règle invalide"},
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

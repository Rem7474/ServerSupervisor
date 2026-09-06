package apperr

import (
	"strings"

	"golang.org/x/text/language"
)

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
	CodeRunbookNotFound            = "RUNBOOK_NOT_FOUND"
	CodeRunbookNameRequired        = "RUNBOOK_NAME_REQUIRED"
	CodeRunbookCreateFailed        = "RUNBOOK_CREATE_FAILED"
	CodeRunbookUpdateFailed        = "RUNBOOK_UPDATE_FAILED"
	CodeRunbookNoSteps             = "RUNBOOK_NO_STEPS"
	CodeRunbookRunFailed           = "RUNBOOK_RUN_FAILED"
	CodeRunbookExecutionNotFound   = "RUNBOOK_EXECUTION_NOT_FOUND"
	CodeRunbookStepsRequired       = "RUNBOOK_STEPS_REQUIRED"
	CodeRunbookStepsInvalid        = "RUNBOOK_STEPS_INVALID"
	CodeAlertRuleIDInvalid         = "ALERT_RULE_ID_INVALID"
	CodeAlertChannelInvalid        = "ALERT_CHANNEL_INVALID"
	CodeAlertCommandModuleInvalid  = "ALERT_COMMAND_MODULE_INVALID"
	CodeAlertCommandActionInvalid  = "ALERT_COMMAND_ACTION_INVALID"
	CodeAlertCommandTargetRequired = "ALERT_COMMAND_TARGET_REQUIRED"
	CodeRunbookStepHostRequired    = "RUNBOOK_STEP_HOST_REQUIRED"
	CodeRunbookStepHostNotFound    = "RUNBOOK_STEP_HOST_NOT_FOUND"
	CodeRunbookStepModuleInvalid   = "RUNBOOK_STEP_MODULE_INVALID"
	CodeRunbookStepActionInvalid   = "RUNBOOK_STEP_ACTION_INVALID"
	CodeRunbookStepTargetRequired  = "RUNBOOK_STEP_TARGET_REQUIRED"
)

// DefaultLanguage is what GetMessage falls back to for a language it has no
// text for. It is also the language the literal apperr.Error.Message is
// written in, so the two agree in logs.
const DefaultLanguage = "en"

// SupportedLanguages are the languages every catalog entry must provide.
// Adding one means adding its code here and its text to each entry —
// catalog_test.go fails on any entry that misses it.
var SupportedLanguages = []string{"en", "fr"}

// ErrorMessage holds one catalog entry's text per language code. {name}
// placeholders are substituted from the Params passed to GetMessage.
//
// A map rather than a struct with one field per language: a third language is
// then purely additive at each entry instead of a struct change that touches
// every one of them.
type ErrorMessage map[string]string

// ErrorCatalog maps stable i18n keys to translated messages.
var ErrorCatalog = map[string]ErrorMessage{
	CodeAdminRequired: {
		"en": "admin access required",
		"fr": "accès administrateur requis",
	},
	CodeAuthRequired: {
		"en": "authentication required",
		"fr": "authentification requise",
	},
	CodeHostAccessDenied: {
		"en": "access denied to this host",
		"fr": "accès refusé à cet hôte",
	},
	CodeOperatorRequired: {
		"en": "operator rights required on this host",
		"fr": "droits opérateur requis sur cet hôte",
	},
	CodePermissionDenied: {
		"en": "permission denied",
		"fr": "accès refusé",
	},
	CodeInvalidToken: {
		"en": "invalid token",
		"fr": "jeton invalide",
	},
	CodeTokenExpired: {
		"en": "token expired",
		"fr": "jeton expiré",
	},
	CodeInvalidInput: {
		"en": "invalid input",
		"fr": "entrée invalide",
	},
	CodeInvalidTimeframe: {
		"en": "invalid timeframe; allowed: hour day week month year",
		"fr": "période invalide ; autorisées : hour day week month year",
	},
	CodeMissingField: {
		"en": "missing required field",
		"fr": "champ obligatoire manquant",
	},
	CodeMissingParameter: {
		"en": "missing required parameter",
		"fr": "paramètre obligatoire manquant",
	},
	CodeNotFound: {
		"en": "not found",
		"fr": "non trouvé",
	},
	CodeNodeNotFound: {
		"en": "node not found",
		"fr": "nœud non trouvé",
	},
	CodeConflict: {
		"en": "conflict",
		"fr": "conflit",
	},
	CodeInternalError: {
		"en": "internal server error",
		"fr": "erreur interne du serveur",
	},
	CodePermissionFailed: {
		"en": "permission check failed",
		"fr": "vérification des permissions échouée",
	},
	CodeProxmoxError: {
		"en": "proxmox operation failed",
		"fr": "opération Proxmox échouée",
	},
	CodeInvalidMetric: {
		"en": "invalid metric",
		"fr": "métrique invalide",
	},
	CodeInvalidOperator: {
		"en": "invalid operator",
		"fr": "opérateur invalide",
	},
	CodeGitProviderUnauthorized: {
		"en": "invalid or expired GitHub token (401) — check GITHUB_TOKEN in settings",
		"fr": "token GitHub invalide ou expiré (401) — vérifiez GITHUB_TOKEN dans les paramètres",
	},
	CodeGitProviderRateLimited: {
		"en": "GitHub rate limit reached (403) — configure a GITHUB_TOKEN to raise the limit",
		"fr": "limite de taux GitHub atteinte (403) — configurez un GITHUB_TOKEN pour augmenter la limite",
	},
	CodeGitProviderNotFound: {
		"en": "repository not found on GitHub (404) — check owner/repo",
		"fr": "dépôt introuvable sur GitHub (404) — vérifiez owner/repo",
	},
	CodeGitProviderError: {
		"en": "GitHub API error ({status})",
		"fr": "erreur GitHub API ({status})",
	},

	CodeAlertRuleNotFound:             {"en": "alert rule not found", "fr": "règle d'alerte introuvable"},
	CodeAlertSourceTypeImmutable:      {"en": "changing source_type is not allowed", "fr": "le changement de source_type n'est pas autorisé"},
	CodeAlertIncidentResolveFailed:    {"en": "rule updated, but resolving its open incidents failed", "fr": "règle mise à jour, mais échec de résolution des incidents ouverts"},
	CodeAlertDockerScopeRequired:      {"en": "a Docker scope is required", "fr": "le scope Docker est requis"},
	CodeAlertDockerHostNotFound:       {"en": "host not found for this Docker scope", "fr": "hôte introuvable pour ce scope Docker"},
	CodeAlertDockerContainerNotFound:  {"en": "Docker container not found for this scope", "fr": "conteneur Docker introuvable pour ce scope"},
	CodeAlertComposeProjectNotFound:   {"en": "Compose project not found for this scope", "fr": "projet Compose introuvable pour ce scope"},
	CodeAlertProxmoxScopeRequired:     {"en": "a Proxmox scope is required", "fr": "le scope Proxmox est requis"},
	CodeAlertProxmoxConnNotFound:      {"en": "Proxmox connection not found for this scope", "fr": "connexion Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxNodeNotFound:      {"en": "Proxmox node not found for this scope", "fr": "nœud Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxStorageNotFound:   {"en": "Proxmox storage not found for this scope", "fr": "stockage Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxGuestNotFound:     {"en": "Proxmox VM/LXC not found for this scope", "fr": "VM/LXC Proxmox introuvable pour ce scope"},
	CodeAlertProxmoxDiskNotFound:      {"en": "Proxmox physical disk not found for this scope", "fr": "disque physique Proxmox introuvable pour ce scope"},
	CodeAlertRollingWindowNotAllowed:  {"en": "the rolling-average window does not apply to this metric", "fr": "la fenêtre de moyenne glissante n'est pas applicable à cette métrique"},
	CodeAlertRollingWindowInvalid:     {"en": "invalid rolling-average window (allowed: 1h, 6h, 24h)", "fr": "fenêtre de moyenne glissante invalide (valeurs autorisées : 1h, 6h, 24h)"},
	CodeAlertCooldownNegative:         {"en": "the silence period must be zero or positive", "fr": "la période de silence doit être positive ou nulle"},
	CodeAlertEscalationNegative:       {"en": "the escalation delay must be zero or positive", "fr": "le délai d'escalade doit être positif ou nul"},
	CodeAlertCommandTriggerIncomplete: {"en": "a command trigger must define both a module and an action", "fr": "le déclencheur de commande doit définir un module et une action"},
	CodeAlertMetricUnsupportedForLogs: {"en": "metric not supported for logs", "fr": "métrique non supportée pour les logs"},
	CodeAlertMetricRequiresProxmox:    {"en": "this metric requires a Proxmox source", "fr": "la métrique requiert une source Proxmox"},
	CodeAlertTemplateMetricNotHostScoped: {
		"en": "Docker, Proxmox and synthetic metrics cannot be used in a template — they do not apply host by host",
		"fr": "les métriques Docker, Proxmox ou synthétiques ne peuvent pas être utilisées dans un template — elles ne s'appliquent pas hôte par hôte",
	},
	CodeAlertTemplateNotFound: {"en": "template not found", "fr": "modèle introuvable"},

	CodeRunbookNotFound:            {"en": "runbook not found", "fr": "runbook introuvable"},
	CodeRunbookNameRequired:        {"en": "the runbook name is required", "fr": "le nom du runbook est requis"},
	CodeRunbookCreateFailed:        {"en": "could not create the runbook", "fr": "erreur lors de la création du runbook"},
	CodeRunbookUpdateFailed:        {"en": "could not update the runbook", "fr": "erreur lors de la mise à jour du runbook"},
	CodeRunbookNoSteps:             {"en": "this runbook has no steps", "fr": "ce runbook n'a aucune étape"},
	CodeRunbookRunFailed:           {"en": "could not start the runbook", "fr": "erreur lors du lancement du runbook"},
	CodeRunbookExecutionNotFound:   {"en": "execution not found", "fr": "exécution introuvable"},
	CodeRunbookStepsRequired:       {"en": "a runbook must have at least one step", "fr": "le runbook doit avoir au moins une étape"},
	CodeRunbookStepsInvalid:        {"en": "could not validate the steps", "fr": "erreur lors de la validation des étapes"},
	CodeAlertRuleIDInvalid:         {"en": "invalid alert rule id", "fr": "identifiant de règle invalide"},
	CodeAlertChannelInvalid:        {"en": "invalid notification channel: {channel}", "fr": "canal de notification invalide : {channel}"},
	CodeAlertCommandModuleInvalid:  {"en": "invalid command module: {module}", "fr": "module de commande invalide : {module}"},
	CodeAlertCommandActionInvalid:  {"en": "invalid action for module {module}: {action}", "fr": "action invalide pour le module {module} : {action}"},
	CodeAlertCommandTargetRequired: {"en": "module {module} requires a target", "fr": "le module {module} requiert une cible"},
	CodeRunbookStepHostRequired:    {"en": "step {step}: the host is required", "fr": "étape {step} : l'hôte est requis"},
	CodeRunbookStepHostNotFound:    {"en": "step {step}: host not found", "fr": "étape {step} : hôte introuvable"},
	CodeRunbookStepModuleInvalid:   {"en": "step {step}: invalid module ({module})", "fr": "étape {step} : module invalide ({module})"},
	CodeRunbookStepActionInvalid:   {"en": "step {step}: invalid action for module {module} ({action})", "fr": "étape {step} : action invalide pour le module {module} ({action})"},
	CodeRunbookStepTargetRequired:  {"en": "step {step}: module {module} requires a target", "fr": "étape {step} : le module {module} requiert une cible"},
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
// {name} placeholders substituted from params. Falls back to DefaultLanguage
// for a language the entry has no text for, and to a generic message for a
// code that isn't in the catalog at all.
func GetMessage(errorCode, lang string, params map[string]string) string {
	msg, found := ErrorCatalog[errorCode]
	if !found {
		// An unknown code means a caller passed a raw string rather than a
		// CodeXxx constant (catalog_test.go proves every constant has an entry).
		// The machine-readable code still reaches the client in the response's
		// `code` field, so nothing is lost by not echoing it as prose here.
		return genericMessage(lang)
	}

	text, ok := msg[normalizeLanguage(lang)]
	if !ok {
		text = msg[DefaultLanguage]
	}
	for name, value := range params {
		text = strings.ReplaceAll(text, "{"+name+"}", value)
	}
	return text
}

// genericMessage is the last resort for an unrecognised code. It reads the
// catalog directly rather than calling GetMessage, which would recurse if
// CodeInternalError were itself missing.
func genericMessage(lang string) string {
	if msg, ok := ErrorCatalog[CodeInternalError]; ok {
		if text, ok := msg[normalizeLanguage(lang)]; ok {
			return text
		}
		if text, ok := msg[DefaultLanguage]; ok {
			return text
		}
	}
	return "internal server error"
}

// normalizeLanguage maps an arbitrary language string onto a supported code,
// falling back to DefaultLanguage.
func normalizeLanguage(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	for _, supported := range SupportedLanguages {
		if lower == supported {
			return supported
		}
	}
	return DefaultLanguage
}

// GetLanguageFromAcceptLanguage picks the best supported language from an
// Accept-Language header, honouring quality values — "en;q=0.3, fr;q=0.9"
// resolves to "fr", which naive first-entry parsing gets backwards. Falls back
// to DefaultLanguage when the header is empty, malformed, or asks only for
// languages this catalog doesn't have.
func GetLanguageFromAcceptLanguage(acceptLanguage string) string {
	if strings.TrimSpace(acceptLanguage) == "" {
		return DefaultLanguage
	}

	tags, qualities, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil {
		return DefaultLanguage
	}

	best, bestQuality := DefaultLanguage, float32(0)
	for i, tag := range tags {
		// q=0 explicitly rejects a language rather than ranking it last.
		if qualities[i] <= 0 {
			continue
		}
		base, confidence := tag.Base()
		if confidence == language.No {
			continue
		}
		code := base.String()
		for _, supported := range SupportedLanguages {
			if code == supported && qualities[i] > bestQuality {
				best, bestQuality = supported, qualities[i]
			}
		}
	}
	return best
}

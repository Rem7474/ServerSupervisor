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
	CodeInvalidGitBaseURL       = "INVALID_GIT_BASE_URL"

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
	CodeAlertHostGroupNotFound           = "ALERT_HOST_GROUP_NOT_FOUND"

	// Host groups (internal/services/hostgroup)
	CodeHostGroupNotFound         = "HOST_GROUP_NOT_FOUND"
	CodeHostGroupNameRequired     = "HOST_GROUP_NAME_REQUIRED"
	CodeHostGroupFieldTooLong     = "HOST_GROUP_FIELD_TOO_LONG"
	CodeHostGroupNameTaken        = "HOST_GROUP_NAME_TAKEN"
	CodeHostGroupMatchModeInvalid = "HOST_GROUP_MATCH_MODE_INVALID"
	CodeHostGroupCriteriaRequired = "HOST_GROUP_CRITERIA_REQUIRED"
	CodeHostGroupInUse            = "HOST_GROUP_IN_USE"

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

	// Config / scheduled tasks / Proxmox / release trackers / discovery / host permissions
	CodeHostPermLevelInvalid          = "HOST_PERM_LEVEL_INVALID"
	CodeConfigParamUnknown            = "CONFIG_PARAM_UNKNOWN"
	CodeConfigParamEnvOnly            = "CONFIG_PARAM_ENV_ONLY"
	CodeConfigParamNotEditable        = "CONFIG_PARAM_NOT_EDITABLE"
	CodeConfigValueInvalid            = "CONFIG_VALUE_INVALID"
	CodeConfigSaveFailed              = "CONFIG_SAVE_FAILED"
	CodeConfigResetFailed             = "CONFIG_RESET_FAILED"
	CodeScheduledTaskModuleInvalid    = "SCHEDULED_TASK_MODULE_INVALID"
	CodeScheduledTaskActionRequired   = "SCHEDULED_TASK_ACTION_REQUIRED"
	CodeScheduledTaskCronInvalid      = "SCHEDULED_TASK_CRON_INVALID"
	CodeProxmoxLinkStatusInvalid      = "PROXMOX_LINK_STATUS_INVALID"
	CodeProxmoxMetricsSourceInvalid   = "PROXMOX_METRICS_SOURCE_INVALID"
	CodeProxmoxConsoleQemuUnsupported = "PROXMOX_CONSOLE_QEMU_UNSUPPORTED"
	CodeProxmoxConsoleNotConfigured   = "PROXMOX_CONSOLE_NOT_CONFIGURED"
	CodeTrackerComposeTargetRequired  = "TRACKER_COMPOSE_TARGET_REQUIRED"
	CodeTrackerNoInitialCheck         = "TRACKER_NO_INITIAL_CHECK"
	CodeTrackerWatchOnly              = "TRACKER_WATCH_ONLY"
	CodeTrackerNoInitialRelease       = "TRACKER_NO_INITIAL_RELEASE"
	CodeDiscoveryCIDRInvalid          = "DISCOVERY_CIDR_INVALID"
	CodeDiscoveryIPv4Only             = "DISCOVERY_IPV4_ONLY"
	CodeDiscoveryPrefixRange          = "DISCOVERY_PREFIX_RANGE"
	CodeSettingsWebhookURLInvalid     = "SETTINGS_WEBHOOK_URL_INVALID"
	CodeSettingsWebhookNotConfigured  = "SETTINGS_WEBHOOK_NOT_CONFIGURED"
	CodeSettingsWebhookSendFailed     = "SETTINGS_WEBHOOK_SEND_FAILED"
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
		"en": "invalid or expired {provider} token (401) — check the access token (GITHUB_TOKEN in settings for GitHub)",
		"fr": "token {provider} invalide ou expiré (401) — vérifiez le token d'accès (GITHUB_TOKEN dans les paramètres pour GitHub)",
	},
	CodeGitProviderRateLimited: {
		"en": "{provider} rate limit reached or access denied (403) — check the access token and its permissions",
		"fr": "limite de taux {provider} atteinte ou accès refusé (403) — vérifiez le token d'accès et ses permissions",
	},
	CodeGitProviderNotFound: {
		"en": "repository not found on {provider} (404) — check owner/repo and the instance URL",
		"fr": "dépôt introuvable sur {provider} (404) — vérifiez owner/repo et l'URL de l'instance",
	},
	CodeGitProviderError: {
		"en": "{provider} API error ({status})",
		"fr": "erreur {provider} API ({status})",
	},
	CodeInvalidGitBaseURL: {
		"en": "invalid instance URL: it must be an http(s) URL with a host, without credentials, query string, loopback or link-local address",
		"fr": "URL d'instance invalide : elle doit être une URL http(s) avec un hôte, sans identifiants, paramètres, adresse loopback ni lien-local",
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
	CodeAlertTemplateNotFound:     {"en": "template not found", "fr": "modèle introuvable"},
	CodeAlertHostGroupNotFound:    {"en": "host group not found for this rule", "fr": "groupe d'hôtes introuvable pour cette règle"},
	CodeHostGroupNotFound:         {"en": "host group not found", "fr": "groupe d'hôtes introuvable"},
	CodeHostGroupNameRequired:     {"en": "the group name is required", "fr": "le nom du groupe est requis"},
	CodeHostGroupFieldTooLong:     {"en": "the group name and OS filter must not exceed 100 characters", "fr": "le nom du groupe et le filtre d'OS ne doivent pas dépasser 100 caractères"},
	CodeHostGroupNameTaken:        {"en": "a host group with this name already exists", "fr": "un groupe d'hôtes porte déjà ce nom"},
	CodeHostGroupMatchModeInvalid: {"en": "invalid match mode (allowed: all, any)", "fr": "mode de correspondance invalide (valeurs autorisées : all, any)"},
	CodeHostGroupCriteriaRequired: {"en": "a group needs at least one tag or an OS filter", "fr": "un groupe doit définir au moins un tag ou un filtre d'OS"},
	CodeHostGroupInUse:            {"en": "this group is targeted by {count} alert rule(s): retarget them before deleting it", "fr": "ce groupe est ciblé par {count} règle(s) d'alerte : changez leur cible avant de le supprimer"},

	CodeRunbookNotFound:               {"en": "runbook not found", "fr": "runbook introuvable"},
	CodeRunbookNameRequired:           {"en": "the runbook name is required", "fr": "le nom du runbook est requis"},
	CodeRunbookCreateFailed:           {"en": "could not create the runbook", "fr": "erreur lors de la création du runbook"},
	CodeRunbookUpdateFailed:           {"en": "could not update the runbook", "fr": "erreur lors de la mise à jour du runbook"},
	CodeRunbookNoSteps:                {"en": "this runbook has no steps", "fr": "ce runbook n'a aucune étape"},
	CodeRunbookRunFailed:              {"en": "could not start the runbook", "fr": "erreur lors du lancement du runbook"},
	CodeRunbookExecutionNotFound:      {"en": "execution not found", "fr": "exécution introuvable"},
	CodeRunbookStepsRequired:          {"en": "a runbook must have at least one step", "fr": "le runbook doit avoir au moins une étape"},
	CodeRunbookStepsInvalid:           {"en": "could not validate the steps", "fr": "erreur lors de la validation des étapes"},
	CodeAlertRuleIDInvalid:            {"en": "invalid alert rule id", "fr": "identifiant de règle invalide"},
	CodeAlertChannelInvalid:           {"en": "invalid notification channel: {channel}", "fr": "canal de notification invalide : {channel}"},
	CodeAlertCommandModuleInvalid:     {"en": "invalid command module: {module}", "fr": "module de commande invalide : {module}"},
	CodeAlertCommandActionInvalid:     {"en": "invalid action for module {module}: {action}", "fr": "action invalide pour le module {module} : {action}"},
	CodeAlertCommandTargetRequired:    {"en": "module {module} requires a target", "fr": "le module {module} requiert une cible"},
	CodeRunbookStepHostRequired:       {"en": "step {step}: the host is required", "fr": "étape {step} : l'hôte est requis"},
	CodeRunbookStepHostNotFound:       {"en": "step {step}: host not found", "fr": "étape {step} : hôte introuvable"},
	CodeRunbookStepModuleInvalid:      {"en": "step {step}: invalid module ({module})", "fr": "étape {step} : module invalide ({module})"},
	CodeRunbookStepActionInvalid:      {"en": "step {step}: invalid action for module {module} ({action})", "fr": "étape {step} : action invalide pour le module {module} ({action})"},
	CodeRunbookStepTargetRequired:     {"en": "step {step}: module {module} requires a target", "fr": "étape {step} : le module {module} requiert une cible"},
	CodeHostPermLevelInvalid:          {"en": "level must be 'viewer' or 'operator'", "fr": "le niveau doit être 'viewer' ou 'operator'"},
	CodeConfigParamUnknown:            {"en": "unknown configuration parameter: {key}", "fr": "paramètre de configuration inconnu : {key}"},
	CodeConfigParamEnvOnly:            {"en": "parameter {key} is managed through the Docker environment only", "fr": "le paramètre {key} est géré uniquement via l'environnement Docker"},
	CodeConfigParamNotEditable:        {"en": "parameter {key} is not editable", "fr": "le paramètre {key} n'est pas modifiable"},
	CodeConfigValueInvalid:            {"en": "invalid value for {key}: {reason}", "fr": "valeur invalide pour {key} : {reason}"},
	CodeConfigSaveFailed:              {"en": "could not save {key}: {reason}", "fr": "erreur lors de l'enregistrement de {key} : {reason}"},
	CodeConfigResetFailed:             {"en": "could not reset {key}: {reason}", "fr": "erreur lors de la réinitialisation de {key} : {reason}"},
	CodeScheduledTaskModuleInvalid:    {"en": "invalid module: {module}", "fr": "module invalide : {module}"},
	CodeScheduledTaskActionRequired:   {"en": "action is required", "fr": "action requise"},
	CodeScheduledTaskCronInvalid:      {"en": "invalid cron expression: {reason}", "fr": "expression cron invalide : {reason}"},
	CodeProxmoxLinkStatusInvalid:      {"en": "invalid status: must be suggested, confirmed or ignored", "fr": "status invalide : doit être suggested, confirmed ou ignored"},
	CodeProxmoxMetricsSourceInvalid:   {"en": "invalid metrics_source: must be auto, agent or proxmox", "fr": "metrics_source invalide : doit être auto, agent ou proxmox"},
	CodeProxmoxConsoleQemuUnsupported: {"en": "console is not available for QEMU VMs yet", "fr": "console non disponible pour les VM QEMU pour le moment"},
	CodeProxmoxConsoleNotConfigured:   {"en": "PVE console is not configured for this connection: credentials required", "fr": "console PVE non configurée pour cette connexion : identifiants requis"},
	CodeTrackerComposeTargetRequired:  {"en": "compose mode: configure a target VM and a compose project to trigger manually", "fr": "mode compose : configurez une VM cible et un projet compose pour déclencher manuellement"},
	CodeTrackerNoInitialCheck:         {"en": "no initial check has run yet: wait for the next polling cycle before triggering manually", "fr": "aucune vérification initiale effectuée — attendez le prochain cycle de polling avant de déclencher manuellement"},
	CodeTrackerWatchOnly:              {"en": "tracker is watch-only: configure a target VM and a task to trigger manually", "fr": "tracker en mode surveillance seule — configurez une VM cible et une tâche pour déclencher manuellement"},
	CodeTrackerNoInitialRelease:       {"en": "no initial release recorded: wait for the next polling cycle before triggering manually", "fr": "aucune release initiale enregistrée — attendez le prochain cycle de polling avant de déclencher manuellement"},
	CodeDiscoveryCIDRInvalid:          {"en": "invalid CIDR address", "fr": "adresse CIDR invalide"},
	CodeDiscoveryIPv4Only:             {"en": "only IPv4 is supported for network discovery", "fr": "seul IPv4 est pris en charge pour la découverte réseau"},
	CodeSettingsWebhookURLInvalid:     {"en": "invalid {provider} webhook URL: it must be an https URL that does not target a local or private address", "fr": "URL de webhook {provider} invalide : elle doit être en https et ne pas cibler une adresse locale ou privée"},
	CodeSettingsWebhookNotConfigured:  {"en": "{provider} webhook URL not configured", "fr": "URL de webhook {provider} non configurée"},
	CodeSettingsWebhookSendFailed:     {"en": "could not send the {provider} test message: {reason}", "fr": "échec de l'envoi du message de test {provider} : {reason}"},
	CodeDiscoveryPrefixRange:          {"en": "the prefix must be between /{min} and /{max} (254 addresses max)", "fr": "le préfixe doit être compris entre /{min} et /{max} (254 adresses max)"},
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

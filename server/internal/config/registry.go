package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ParamType represents the data type of a configuration parameter.
type ParamType string

const (
	TypeString   ParamType = "string"
	TypeInt      ParamType = "int"
	TypeFloat    ParamType = "float"
	TypeBool     ParamType = "bool"
	TypeDuration ParamType = "duration"
	TypeCSV      ParamType = "csv"
	TypeSelect   ParamType = "select"
)

// ConfigParamDef defines the metadata, validation, and defaults for a parameter.
type ConfigParamDef struct {
	Key             string                 `json:"key"`
	SettingKey      string                 `json:"setting_key"`
	EnvVar          string                 `json:"env_var"`
	Type            ParamType              `json:"type"`
	DefaultValue    string                 `json:"default_value"`
	Category        string                 `json:"category"`
	Label           string                 `json:"label"`
	Description     string                 `json:"description"`
	IsSecret        bool                   `json:"is_secret"`
	IsEditable      bool                   `json:"is_editable"`
	RequiresRestart bool                   `json:"requires_restart"`
	Options         []string               `json:"options,omitempty"`
	Validate        func(val string) error `json:"-"`
}

func defParam(key, settingKey, category string, ptype ParamType, defVal, label, desc string, isSecret, isEditable, requiresRestart bool, val func(string) error, options ...string) ConfigParamDef {
	return ConfigParamDef{
		Key:             key,
		SettingKey:      settingKey,
		EnvVar:          key,
		Type:            ptype,
		DefaultValue:    defVal,
		Category:        category,
		Label:           label,
		Description:     desc,
		IsSecret:        isSecret,
		IsEditable:      isEditable,
		RequiresRestart: requiresRestart,
		Validate:        val,
		Options:         options,
	}
}

func strParam(key, settingKey, cat, def, label, desc string, secret, restart bool, val ...func(string) error) ConfigParamDef {
	var v func(string) error
	if len(val) > 0 {
		v = val[0]
	}
	return defParam(key, settingKey, cat, TypeString, def, label, desc, secret, true, restart, v)
}

func readOnlyParam(key, settingKey, cat, def, label, desc string, secret, restart bool) ConfigParamDef {
	return defParam(key, settingKey, cat, TypeString, def, label, desc, secret, false, restart, nil)
}

func boolParam(key, settingKey, cat, def, label, desc string, restart bool, editable ...bool) ConfigParamDef {
	ed := true
	if len(editable) > 0 {
		ed = editable[0]
	}
	return defParam(key, settingKey, cat, TypeBool, def, label, desc, false, ed, restart, validateBool)
}

func intParam(key, settingKey, cat, def, label, desc string, restart bool, val func(string) error) ConfigParamDef {
	return defParam(key, settingKey, cat, TypeInt, def, label, desc, false, true, restart, val)
}

func floatParam(key, settingKey, cat, def, label, desc string, val func(string) error) ConfigParamDef {
	return defParam(key, settingKey, cat, TypeFloat, def, label, desc, false, true, false, val)
}

func durationParam(key, settingKey, cat, def, label, desc string, restart bool) ConfigParamDef {
	return defParam(key, settingKey, cat, TypeDuration, def, label, desc, false, true, restart, validateDuration)
}

func csvParam(key, settingKey, cat, def, label, desc string, restart bool) ConfigParamDef {
	return defParam(key, settingKey, cat, TypeCSV, def, label, desc, false, true, restart, nil)
}

func selectParam(key, settingKey, cat, def, label, desc string, restart bool, options ...string) ConfigParamDef {
	return defParam(key, settingKey, cat, TypeSelect, def, label, desc, false, true, restart, validateOptions(options), options...)
}

// AllParams contains the exhaustive catalog of system configuration parameters.
var AllParams = []ConfigParamDef{
	// ==================== Server ====================
	intParam("SERVER_PORT", "server_port", "server", "8080", "Port d'écoute du serveur", "Port TCP sur lequel le serveur écoute les requêtes HTTP.", true, validatePort),
	strParam("BASE_URL", "base_url", "server", "http://localhost:8080", "URL de base publique", "URL racine utilisée pour générer les liens absolus et redirections d'authentification.", false, false, validateURL),
	boolParam("TLS_ENABLED", "tls_enabled", "server", "false", "HTTPS / TLS activé", "Indique si l'application est servie directement derrière une terminaison TLS.", false),
	boolParam("DEMO_MODE", "demo_mode", "server", "false", "Mode démonstration", "Désactive tout appel réseau sortant (probes, git, proxmox). Modifiable uniquement via ENV.", true, false),
	strParam("TZ", "tz", "server", "UTC", "Fuseau horaire (Timezone)", "Fuseau horaire utilisé pour les plannings cron et l'affichage des dates.", false, true),

	// ==================== Logging ====================
	selectParam("LOG_LEVEL", "log_level", "logging", "info", "Niveau de journalisation", "Niveau minimal des messages enregistrés dans les logs du serveur.", false, "debug", "info", "warn", "error"),
	selectParam("LOG_FORMAT", "log_format", "logging", "json", "Format des journaux", "Format de sérialisation des logs applicatifs (json pour conteneur, text pour debug).", true, "json", "text"),

	// ==================== Network & Proxies ====================
	csvParam("TRUSTED_PROXIES", "trusted_proxies", "network", "", "Proxies de confiance (CIDRs)", "Liste séparée par virgules des plages IP / reverse proxies autorisés à relayer les en-têtes X-Forwarded-For.", false),
	csvParam("ALLOWED_ORIGINS", "allowed_origins", "network", "", "Origines WebSocket autorisées (CORS)", "Liste des origines web autorisées pour la connexion au flux WebSocket en direct.", false),

	// ==================== Database ====================
	strParam("DB_HOST", "db_host", "database", "localhost", "Hôte de la base de données", "Nom d'hôte ou adresse IP du serveur PostgreSQL / TimescaleDB.", false, true),
	intParam("DB_PORT", "db_port", "database", "5432", "Port de la base de données", "Port TCP du service PostgreSQL.", true, validatePort),
	strParam("DB_USER", "db_user", "database", "supervisor", "Utilisateur de la base de données", "Identifiant de connexion au serveur de base de données.", false, true),
	strParam("DB_PASSWORD", "db_password", "database", "supervisor", "Mot de passe de la base de données", "Mot de passe associé à l'utilisateur PostgreSQL.", true, true),
	strParam("DB_NAME", "db_name", "database", "serversupervisor", "Nom de la base de données", "Base de données de ServerSupervisor.", false, true),
	selectParam("DB_SSLMODE", "db_sslmode", "database", "disable", "Mode SSL de la base de données", "Mode de chiffrement SSL pour la liaison PostgreSQL.", true, "disable", "require", "verify-ca", "verify-full"),

	// ==================== Auth ====================
	strParam("JWT_SECRET", "jwt_secret", "auth", "", "Secret de signature JWT", "Clé cryptographique secrète pour signer les jetons d'accès JWT (minimum 32 caractères).", true, false),
	durationParam("JWT_EXPIRATION", "jwt_expiration", "auth", "24h", "Durée de validité des jetons JWT", "Durée de validité d'une session utilisateur avant renouvellement.", false),
	durationParam("REFRESH_TOKEN_EXPIRATION", "refresh_token_expiration", "auth", "168h", "Durée de validité du jeton de rafraîchissement", "Durée maximale avant obligation de reconnexion complète (par défaut 7 jours).", false),
	readOnlyParam("ADMIN_USER", "admin_user", "auth", "admin", "Nom d'utilisateur administrateur par défaut", "Nom du compte administrateur initial créé lors du premier démarrage.", false, true),
	readOnlyParam("ADMIN_PASSWORD", "admin_password", "auth", "", "Mot de passe initial administrateur", "Mot de passe temporaire pour le bootstrap du compte admin (optionnel si défini au boot).", true, true),
	intParam("RATE_LIMIT_RPS", "rate_limit_rps", "auth", "100", "Limite de requêtes globales (RPS)", "Nombre maximal de requêtes par seconde autorisées par IP.", true, validatePositiveInt),
	intParam("RATE_LIMIT_BURST", "rate_limit_burst", "auth", "200", "Capacité de rafale requêtes (Burst)", "Pic de requêtes admissible au-delà de la limite standard.", true, validatePositiveInt),

	// ==================== OIDC / SSO ====================
	boolParam("OIDC_ENABLED", "oidc_enabled", "oidc", "false", "Activer OpenID Connect / SSO", "Active l'authentification unique d'entreprise via un fournisseur d'identité OIDC.", false),
	strParam("OIDC_DISPLAY_NAME", "oidc_display_name", "oidc", "SSO / OpenID Connect", "Nom affiché du fournisseur OIDC", "Texte affiché sur le bouton de connexion de la page de login.", false, false),
	strParam("OIDC_ISSUER_URL", "oidc_issuer_url", "oidc", "", "URL de l'émetteur OIDC (Issuer URL)", "URL racine du fournisseur d'identité OIDC (ex: https://auth.example.com).", false, false),
	strParam("OIDC_CLIENT_ID", "oidc_client_id", "oidc", "", "Identifiant client OIDC (Client ID)", "Client ID fourni par votre serveur d'identité OIDC.", false, false),
	strParam("OIDC_CLIENT_SECRET", "oidc_client_secret", "oidc", "", "Secret client OIDC (Client Secret)", "Secret partagé du client OIDC.", true, false),
	strParam("OIDC_REDIRECT_URL", "oidc_redirect_url", "oidc", "", "URL de redirection OIDC (Callback)", "URL de retour après authentification (ex: https://supervisor.example.com/api/v1/auth/oidc/callback).", false, false),
	csvParam("OIDC_SCOPES", "oidc_scopes", "oidc", "openid, profile, email, groups", "Portées OIDC (Scopes)", "Scopes demandés au fournisseur (séparés par des virgules).", false),
	strParam("OIDC_USERNAME_CLAIM", "oidc_username_claim", "oidc", "preferred_username", "Revendication du nom d'utilisateur (Username Claim)", "Champ du token ID à utiliser comme nom d'utilisateur.", false, false),
	strParam("OIDC_EMAIL_CLAIM", "oidc_email_claim", "oidc", "email", "Revendication de l'email (Email Claim)", "Champ du token ID contenant l'adresse email.", false, false),
	strParam("OIDC_GROUPS_CLAIM", "oidc_groups_claim", "oidc", "groups", "Revendication des groupes (Groups Claim)", "Champ du token contenant la liste des groupes de l'utilisateur.", false, false),
	strParam("OIDC_ADMIN_GROUP", "oidc_admin_group", "oidc", "serversupervisor-admins", "Groupe administrateurs OIDC", "Groupe accordant le rôle 'admin'.", false, false),
	strParam("OIDC_OPERATOR_GROUP", "oidc_operator_group", "oidc", "serversupervisor-operators", "Groupe opérateurs OIDC", "Groupe accordant le rôle 'operator'.", false, false),
	strParam("OIDC_VIEWER_GROUP", "oidc_viewer_group", "oidc", "serversupervisor-viewers", "Groupe observateurs OIDC", "Groupe accordant le rôle 'viewer'.", false, false),
	selectParam("OIDC_DEFAULT_ROLE", "oidc_default_role", "oidc", "viewer", "Rôle OIDC par défaut", "Rôle attribué à un utilisateur sans correspondance de groupe spécifique.", false, "viewer", "operator", "admin"),
	boolParam("OIDC_AUTO_CREATE_USER", "oidc_auto_create_user", "oidc", "true", "Création automatique des comptes", "Créer automatiquement un compte local lors de la première connexion OIDC réussie.", false),
	boolParam("OIDC_ALLOW_LOCAL_LOGIN", "oidc_allow_local_login", "oidc", "true", "Autoriser l'authentification locale", "Garde actif le formulaire login/mot de passe classique même si OIDC est activé.", false),
	boolParam("OIDC_INSECURE_SKIP_VERIFY", "oidc_insecure_skip_verify", "oidc", "false", "Ignorer la vérification SSL OIDC", "Désactive la vérification des certificats TLS de l'émetteur OIDC (déconseillé en production).", false),

	// ==================== Notifications / Alerts ====================
	strParam("NOTIFY_URL", "ntfy_url", "notifications", "", "URL du topic ntfy.sh", "URL complète du topic ntfy pour les alertes push instantanées (ex: https://ntfy.sh/mon-serveur).", false, false),
	strParam("NTFY_AUTH_TOKEN", "ntfy_auth_token", "notifications", "", "Jeton d'authentification ntfy", "Token d'accès optionnel pour les topics ntfy privés.", true, false),
	strParam("SMTP_HOST", "smtp_host", "notifications", "", "Hôte SMTP", "Adresse du serveur de messagerie sortant pour l'envoi d'emails d'alerte.", false, false),
	intParam("SMTP_PORT", "smtp_port", "notifications", "587", "Port SMTP", "Port de connexion SMTP (généralement 587 pour STARTTLS, 465 pour SMTPS).", false, validatePort),
	strParam("SMTP_USER", "smtp_user", "notifications", "", "Utilisateur SMTP", "Identifiant d'authentification auprès du serveur SMTP.", false, false),
	strParam("SMTP_PASS", "smtp_pass", "notifications", "", "Mot de passe SMTP", "Mot de passe du compte SMTP pour l'envoi d'emails.", true, false),
	strParam("SMTP_FROM", "smtp_from", "notifications", "", "Adresse expéditeur (From)", "Adresse affichée comme expéditeur des notifications email.", false, false),
	strParam("SMTP_TO", "smtp_to", "notifications", "", "Adresse destinataire par défaut (To)", "Adresse recevant par défaut les alertes et rapports par email.", false, false),
	boolParam("SMTP_TLS", "smtp_tls", "notifications", "true", "Activer TLS pour SMTP", "Chiffre la communication avec le serveur SMTP via STARTTLS ou TLS direct.", false),

	// ==================== Integrations ====================
	strParam("GITHUB_TOKEN", "github_token", "integrations", "", "Token d'accès personnel GitHub", "Jeton GitHub pour surveiller les nouvelles versions et éviter le rate-limiting de l'API publique.", true, false),
	// Both poll intervals are read once at startup to size a fixed
	// time.Ticker (poller.Every, called from main.go) — changing the
	// in-memory Config value afterwards doesn't reach an already-running
	// ticker, so this genuinely needs a restart, unlike most other
	// duration/secret settings in this catalog.
	durationParam("GITHUB_POLL_INTERVAL", "github_poll_interval", "integrations", "15m", "Intervalle de scrutation GitHub", "Cadence de vérification des releases et tags GitHub distants.", true),
	durationParam("DOCKER_IMAGE_POLL_INTERVAL", "docker_image_poll_interval", "integrations", "6h", "Intervalle de scrutation des images Docker", "Cadence de vérification des nouveaux digests de conteneurs sur les registres distants.", true),

	// ==================== Retention ====================
	intParam("METRICS_RETENTION_DAYS", "metrics_retention_days", "retention", "30", "Rétention des métriques (jours)", "Nombre de jours pendant lesquels les séries temporelles de métriques sont conservées.", false, validatePositiveInt),
	intParam("AUDIT_RETENTION_DAYS", "audit_retention_days", "retention", "90", "Rétention des journaux d'audit (jours)", "Durée de conservation des traces d'audit de sécurité et d'actions d'administration.", false, validatePositiveInt),
	intParam("WEB_LOGS_RETENTION_DAYS", "web_logs_retention_days", "retention", "30", "Rétention des logs d'accès web (jours)", "Durée de conservation des requêtes HTTP capturées pour l'analyse de trafic et sécurité.", false, validatePositiveInt),
	intParam("NETWORK_FLOWS_RETENTION_DAYS", "network_flows_retention_days", "retention", "14", "Rétention des flux réseau (jours)", "Durée de conservation des métriques de connexions réseau (top talkers).", false, validatePositiveInt),

	// ==================== Threat Detection ====================
	floatParam("THREAT_WEIGHT_WORDPRESS", "threat_weight_wordpress", "threats", "2", "Pondération des scans WordPress", "Poids attribué aux sondes ciblant wp-login, xmlrpc ou des chemins WordPress.", validateNonNegativeFloat),
	floatParam("THREAT_WEIGHT_ADMIN_PANEL", "threat_weight_adminpanel", "threats", "3", "Pondération panneaux d'admin", "Poids des tentatives d'accès aux interfaces administratives non autorisées.", validateNonNegativeFloat),
	floatParam("THREAT_WEIGHT_PATH_TRAVERSAL", "threat_weight_pathtraversal", "threats", "5", "Pondération traversée de répertoires", "Poids des requêtes contenant ../ ou des patterns d'évasion de chemin.", validateNonNegativeFloat),
	floatParam("THREAT_WEIGHT_KNOWN_SCANNER", "threat_weight_knownscanner", "threats", "4", "Pondération scanners connus", "Poids des User-Agents identifiés comme outils de scan de vulnérabilités.", validateNonNegativeFloat),
	floatParam("THREAT_WEIGHT_SUSPICIOUS_METHOD", "threat_weight_suspiciousmethod", "threats", "2", "Pondération méthodes HTTP suspectes", "Poids des méthodes HTTP inhabituelles ou dangereuses (CONNECT, PROPFIND, etc.).", validateNonNegativeFloat),
	floatParam("THREAT_THRESHOLD_MEDIUM", "threat_threshold_medium", "threats", "15", "Seuil de menace : Modéré", "Score total à partir duquel une adresse IP est classée comme menace modérée.", validatePositiveFloat),
	floatParam("THREAT_THRESHOLD_HIGH", "threat_threshold_high", "threats", "50", "Seuil de menace : Élevé", "Score total à partir duquel une adresse IP est classée comme menace élevée.", validatePositiveFloat),
	floatParam("THREAT_THRESHOLD_CRITICAL", "threat_threshold_critical", "threats", "150", "Seuil de menace : Critique", "Score total à partir duquel une adresse IP est classée comme menace critique.", validatePositiveFloat),
}

// ParamsByKey indexes AllParams by its canonical Key for fast lookup.
var ParamsByKey = func() map[string]ConfigParamDef {
	m := make(map[string]ConfigParamDef, len(AllParams)*2)
	for _, p := range AllParams {
		m[p.Key] = p
		if p.SettingKey != "" {
			m[p.SettingKey] = p
		}
	}
	return m
}()

// FindParam finds a parameter definition by either its canonical Key, SettingKey, or EnvVar.
func FindParam(name string) (ConfigParamDef, bool) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	lower := strings.ToLower(strings.TrimSpace(name))
	if p, ok := ParamsByKey[upper]; ok {
		return p, true
	}
	if p, ok := ParamsByKey[lower]; ok {
		return p, true
	}
	for _, p := range AllParams {
		if strings.EqualFold(p.Key, name) || strings.EqualFold(p.SettingKey, name) || strings.EqualFold(p.EnvVar, name) {
			return p, true
		}
	}
	return ConfigParamDef{}, false
}

// Validation helpers
func validatePort(val string) error {
	p, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("port must be an integer between 1 and 65535, got '%s'", val)
	}
	return nil
}

func validateURL(val string) error {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL: '%s'", val)
	}
	return nil
}

func validateBool(val string) error {
	trimmed := strings.ToLower(strings.TrimSpace(val))
	if trimmed != "true" && trimmed != "false" && trimmed != "1" && trimmed != "0" {
		return fmt.Errorf("boolean value expected (true/false), got '%s'", val)
	}
	return nil
}

func validatePositiveInt(val string) error {
	i, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || i <= 0 {
		return fmt.Errorf("positive integer expected, got '%s'", val)
	}
	return nil
}

func validateDuration(val string) error {
	_, err := time.ParseDuration(strings.TrimSpace(val))
	if err != nil {
		return fmt.Errorf("invalid duration format (e.g. 15m, 24h): '%s'", val)
	}
	return nil
}

func validateOptions(allowed []string) func(string) error {
	return func(val string) error {
		trimmed := strings.ToLower(strings.TrimSpace(val))
		for _, opt := range allowed {
			if strings.EqualFold(opt, trimmed) {
				return nil
			}
		}
		return fmt.Errorf("value must be one of [%s], got '%s'", strings.Join(allowed, ", "), val)
	}
}

func validatePositiveFloat(val string) error {
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err != nil || f <= 0 {
		return fmt.Errorf("positive number expected, got '%s'", val)
	}
	return nil
}

func validateNonNegativeFloat(val string) error {
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err != nil || f < 0 {
		return fmt.Errorf("non-negative number expected, got '%s'", val)
	}
	return nil
}

func ValidateIPOrCIDR(val string) error {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	for _, item := range strings.Split(trimmed, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(item); err != nil {
			if ip := net.ParseIP(item); ip == nil {
				return fmt.Errorf("invalid IP or CIDR '%s'", item)
			}
		}
	}
	return nil
}

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
	Key             string                 `json:"key"`              // Technical canonical key e.g. "SERVER_PORT"
	SettingKey      string                 `json:"setting_key"`      // DB key in settings table e.g. "server_port"
	EnvVar          string                 `json:"env_var"`          // Environment variable name e.g. "SERVER_PORT"
	Type            ParamType              `json:"type"`             // string, int, bool, float, duration, csv, select
	DefaultValue    string                 `json:"default_value"`    // Hardcoded fallback default value
	Category        string                 `json:"category"`         // server, logging, network, database, auth, oidc, notifications, integrations, retention, threats
	Label           string                 `json:"label"`            // Human readable label
	Description     string                 `json:"description"`      // Explanation of what the parameter does
	IsSecret        bool                   `json:"is_secret"`        // Mask in UI and audit logs
	IsEditable      bool                   `json:"is_editable"`      // Whether parameter can be edited via UI
	RequiresRestart bool                   `json:"requires_restart"` // Whether change takes effect only after container restart
	Options         []string               `json:"options,omitempty"`// Valid options if TypeSelect
	Validate        func(val string) error `json:"-"`                // Custom validation
}

// AllParams contains the exhaustive catalog of system configuration parameters.
var AllParams = []ConfigParamDef{
	// ==================== Server ====================
	{
		Key:             "SERVER_PORT",
		SettingKey:      "server_port",
		EnvVar:          "SERVER_PORT",
		Type:            TypeInt,
		DefaultValue:    "8080",
		Category:        "server",
		Label:           "Port d'écoute du serveur",
		Description:     "Port TCP sur lequel le serveur écoute les requêtes HTTP.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
		Validate:        validatePort,
	},
	{
		Key:             "BASE_URL",
		SettingKey:      "base_url",
		EnvVar:          "BASE_URL",
		Type:            TypeString,
		DefaultValue:    "http://localhost:8080",
		Category:        "server",
		Label:           "URL de base publique",
		Description:     "URL racine utilisée pour générer les liens absolus et redirections d'authentification.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateURL,
	},
	{
		Key:             "TLS_ENABLED",
		SettingKey:      "tls_enabled",
		EnvVar:          "TLS_ENABLED",
		Type:            TypeBool,
		DefaultValue:    "false",
		Category:        "server",
		Label:           "HTTPS / TLS activé",
		Description:     "Indique si l'application est servie directement derrière une terminaison TLS.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateBool,
	},
	{
		Key:             "DEMO_MODE",
		SettingKey:      "demo_mode",
		EnvVar:          "DEMO_MODE",
		Type:            TypeBool,
		DefaultValue:    "false",
		Category:        "server",
		Label:           "Mode démonstration",
		Description:     "Désactive tout appel réseau sortant (probes, git, proxmox). Modifiable uniquement via ENV.",
		IsSecret:        false,
		IsEditable:      false,
		RequiresRestart: true,
		Validate:        validateBool,
	},
	{
		Key:             "TZ",
		SettingKey:      "tz",
		EnvVar:          "TZ",
		Type:            TypeString,
		DefaultValue:    "UTC",
		Category:        "server",
		Label:           "Fuseau horaire (Timezone)",
		Description:     "Fuseau horaire utilisé pour les plannings cron et l'affichage des dates.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
	},

	// ==================== Logging ====================
	{
		Key:             "LOG_LEVEL",
		SettingKey:      "log_level",
		EnvVar:          "LOG_LEVEL",
		Type:            TypeSelect,
		DefaultValue:    "info",
		Category:        "logging",
		Label:           "Niveau de journalisation",
		Description:     "Niveau minimal des messages enregistrés dans les logs du serveur.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Options:         []string{"debug", "info", "warn", "error"},
		Validate:        validateOptions([]string{"debug", "info", "warn", "error"}),
	},
	{
		Key:             "LOG_FORMAT",
		SettingKey:      "log_format",
		EnvVar:          "LOG_FORMAT",
		Type:            TypeSelect,
		DefaultValue:    "json",
		Category:        "logging",
		Label:           "Format des journaux",
		Description:     "Format de sérialisation des logs applicatifs (json pour conteneur, text pour debug).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
		Options:         []string{"json", "text"},
		Validate:        validateOptions([]string{"json", "text"}),
	},

	// ==================== Network & Proxies ====================
	{
		Key:             "TRUSTED_PROXIES",
		SettingKey:      "trusted_proxies",
		EnvVar:          "TRUSTED_PROXIES",
		Type:            TypeCSV,
		DefaultValue:    "",
		Category:        "network",
		Label:           "Proxies de confiance (CIDRs)",
		Description:     "Liste séparée par virgules des plages IP / reverse proxies autorisés à relayer les en-têtes X-Forwarded-For.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "ALLOWED_ORIGINS",
		SettingKey:      "allowed_origins",
		EnvVar:          "ALLOWED_ORIGINS",
		Type:            TypeCSV,
		DefaultValue:    "",
		Category:        "network",
		Label:           "Origines WebSocket autorisées (CORS)",
		Description:     "Liste des origines web autorisées pour la connexion au flux WebSocket en direct.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},

	// ==================== Database ====================
	{
		Key:             "DB_HOST",
		SettingKey:      "db_host",
		EnvVar:          "DB_HOST",
		Type:            TypeString,
		DefaultValue:    "localhost",
		Category:        "database",
		Label:           "Hôte de la base de données",
		Description:     "Nom d'hôte ou adresse IP du serveur PostgreSQL / TimescaleDB.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
	},
	{
		Key:             "DB_PORT",
		SettingKey:      "db_port",
		EnvVar:          "DB_PORT",
		Type:            TypeInt,
		DefaultValue:    "5432",
		Category:        "database",
		Label:           "Port de la base de données",
		Description:     "Port TCP du service PostgreSQL.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
		Validate:        validatePort,
	},
	{
		Key:             "DB_USER",
		SettingKey:      "db_user",
		EnvVar:          "DB_USER",
		Type:            TypeString,
		DefaultValue:    "supervisor",
		Category:        "database",
		Label:           "Utilisateur de la base de données",
		Description:     "Identifiant de connexion au serveur de base de données.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
	},
	{
		Key:             "DB_PASSWORD",
		SettingKey:      "db_password",
		EnvVar:          "DB_PASSWORD",
		Type:            TypeString,
		DefaultValue:    "supervisor",
		Category:        "database",
		Label:           "Mot de passe de la base de données",
		Description:     "Mot de passe associé à l'utilisateur PostgreSQL.",
		IsSecret:        true,
		IsEditable:      true,
		RequiresRestart: true,
	},
	{
		Key:             "DB_NAME",
		SettingKey:      "db_name",
		EnvVar:          "DB_NAME",
		Type:            TypeString,
		DefaultValue:    "serversupervisor",
		Category:        "database",
		Label:           "Nom de la base de données",
		Description:     "Base de données de ServerSupervisor.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
	},
	{
		Key:             "DB_SSLMODE",
		SettingKey:      "db_sslmode",
		EnvVar:          "DB_SSLMODE",
		Type:            TypeSelect,
		DefaultValue:    "disable",
		Category:        "database",
		Label:           "Mode SSL de la base de données",
		Description:     "Mode de chiffrement SSL pour la liaison PostgreSQL.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
		Options:         []string{"disable", "require", "verify-ca", "verify-full"},
		Validate:        validateOptions([]string{"disable", "require", "verify-ca", "verify-full"}),
	},

	// ==================== Auth ====================
	{
		Key:             "JWT_SECRET",
		SettingKey:      "jwt_secret",
		EnvVar:          "JWT_SECRET",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "auth",
		Label:           "Secret de signature JWT",
		Description:     "Clé cryptographique secrète pour signer les jetons d'accès JWT (minimum 32 caractères).",
		IsSecret:        true,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "JWT_EXPIRATION",
		SettingKey:      "jwt_expiration",
		EnvVar:          "JWT_EXPIRATION",
		Type:            TypeDuration,
		DefaultValue:    "24h",
		Category:        "auth",
		Label:           "Durée de validité des jetons JWT",
		Description:     "Durée de validité d'une session utilisateur avant renouvellement.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateDuration,
	},
	{
		Key:             "REFRESH_TOKEN_EXPIRATION",
		SettingKey:      "refresh_token_expiration",
		EnvVar:          "REFRESH_TOKEN_EXPIRATION",
		Type:            TypeDuration,
		DefaultValue:    "168h",
		Category:        "auth",
		Label:           "Durée de validité du jeton de rafraîchissement",
		Description:     "Durée maximale avant obligation de reconnexion complète (par défaut 7 jours).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateDuration,
	},
	{
		Key:             "ADMIN_USER",
		SettingKey:      "admin_user",
		EnvVar:          "ADMIN_USER",
		Type:            TypeString,
		DefaultValue:    "admin",
		Category:        "auth",
		Label:           "Nom d'utilisateur administrateur par défaut",
		Description:     "Nom du compte administrateur initial créé lors du premier démarrage.",
		IsSecret:        false,
		IsEditable:      false,
		RequiresRestart: true,
	},
	{
		Key:             "ADMIN_PASSWORD",
		SettingKey:      "admin_password",
		EnvVar:          "ADMIN_PASSWORD",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "auth",
		Label:           "Mot de passe initial administrateur",
		Description:     "Mot de passe temporaire pour le bootstrap du compte admin (optionnel si défini au boot).",
		IsSecret:        true,
		IsEditable:      false,
		RequiresRestart: true,
	},
	{
		Key:             "RATE_LIMIT_RPS",
		SettingKey:      "rate_limit_rps",
		EnvVar:          "RATE_LIMIT_RPS",
		Type:            TypeInt,
		DefaultValue:    "100",
		Category:        "auth",
		Label:           "Limite de requêtes globales (RPS)",
		Description:     "Nombre maximal de requêtes par seconde autorisées par IP.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
		Validate:        validatePositiveInt,
	},
	{
		Key:             "RATE_LIMIT_BURST",
		SettingKey:      "rate_limit_burst",
		EnvVar:          "RATE_LIMIT_BURST",
		Type:            TypeInt,
		DefaultValue:    "200",
		Category:        "auth",
		Label:           "Capacité de rafale requêtes (Burst)",
		Description:     "Pic de requêtes admissible au-delà de la limite standard.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: true,
		Validate:        validatePositiveInt,
	},

	// ==================== OIDC / SSO ====================
	{
		Key:             "OIDC_ENABLED",
		SettingKey:      "oidc_enabled",
		EnvVar:          "OIDC_ENABLED",
		Type:            TypeBool,
		DefaultValue:    "false",
		Category:        "oidc",
		Label:           "Activer OpenID Connect / SSO",
		Description:     "Active l'authentification unique d'entreprise via un fournisseur d'identité OIDC.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateBool,
	},
	{
		Key:             "OIDC_DISPLAY_NAME",
		SettingKey:      "oidc_display_name",
		EnvVar:          "OIDC_DISPLAY_NAME",
		Type:            TypeString,
		DefaultValue:    "SSO / OpenID Connect",
		Category:        "oidc",
		Label:           "Nom affiché du fournisseur OIDC",
		Description:     "Texte affiché sur le bouton de connexion de la page de login.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_ISSUER_URL",
		SettingKey:      "oidc_issuer_url",
		EnvVar:          "OIDC_ISSUER_URL",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "oidc",
		Label:           "URL de l'émetteur OIDC (Issuer URL)",
		Description:     "URL racine du fournisseur d'identité OIDC (ex: https://auth.example.com).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_CLIENT_ID",
		SettingKey:      "oidc_client_id",
		EnvVar:          "OIDC_CLIENT_ID",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "oidc",
		Label:           "Identifiant client OIDC (Client ID)",
		Description:     "Client ID fourni par votre serveur d'identité OIDC.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_CLIENT_SECRET",
		SettingKey:      "oidc_client_secret",
		EnvVar:          "OIDC_CLIENT_SECRET",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "oidc",
		Label:           "Secret client OIDC (Client Secret)",
		Description:     "Secret partagé du client OIDC.",
		IsSecret:        true,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_REDIRECT_URL",
		SettingKey:      "oidc_redirect_url",
		EnvVar:          "OIDC_REDIRECT_URL",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "oidc",
		Label:           "URL de redirection OIDC (Callback)",
		Description:     "URL de retour après authentification (ex: https://supervisor.example.com/api/v1/auth/oidc/callback).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_SCOPES",
		SettingKey:      "oidc_scopes",
		EnvVar:          "OIDC_SCOPES",
		Type:            TypeCSV,
		DefaultValue:    "openid, profile, email, groups",
		Category:        "oidc",
		Label:           "Portées OIDC (Scopes)",
		Description:     "Scopes demandés au fournisseur (séparés par des virgules).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_USERNAME_CLAIM",
		SettingKey:      "oidc_username_claim",
		EnvVar:          "OIDC_USERNAME_CLAIM",
		Type:            TypeString,
		DefaultValue:    "preferred_username",
		Category:        "oidc",
		Label:           "Revendication du nom d'utilisateur (Username Claim)",
		Description:     "Champ du token ID à utiliser comme nom d'utilisateur.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_EMAIL_CLAIM",
		SettingKey:      "oidc_email_claim",
		EnvVar:          "OIDC_EMAIL_CLAIM",
		Type:            TypeString,
		DefaultValue:    "email",
		Category:        "oidc",
		Label:           "Revendication de l'email (Email Claim)",
		Description:     "Champ du token ID contenant l'adresse email.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_GROUPS_CLAIM",
		SettingKey:      "oidc_groups_claim",
		EnvVar:          "OIDC_GROUPS_CLAIM",
		Type:            TypeString,
		DefaultValue:    "groups",
		Category:        "oidc",
		Label:           "Revendication des groupes (Groups Claim)",
		Description:     "Champ du token contenant la liste des groupes de l'utilisateur.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_ADMIN_GROUP",
		SettingKey:      "oidc_admin_group",
		EnvVar:          "OIDC_ADMIN_GROUP",
		Type:            TypeString,
		DefaultValue:    "serversupervisor-admins",
		Category:        "oidc",
		Label:           "Groupe administrateurs OIDC",
		Description:     "Groupe accordant le rôle 'admin'.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_OPERATOR_GROUP",
		SettingKey:      "oidc_operator_group",
		EnvVar:          "OIDC_OPERATOR_GROUP",
		Type:            TypeString,
		DefaultValue:    "serversupervisor-operators",
		Category:        "oidc",
		Label:           "Groupe opérateurs OIDC",
		Description:     "Groupe accordant le rôle 'operator'.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_VIEWER_GROUP",
		SettingKey:      "oidc_viewer_group",
		EnvVar:          "OIDC_VIEWER_GROUP",
		Type:            TypeString,
		DefaultValue:    "serversupervisor-viewers",
		Category:        "oidc",
		Label:           "Groupe observateurs OIDC",
		Description:     "Groupe accordant le rôle 'viewer'.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "OIDC_DEFAULT_ROLE",
		SettingKey:      "oidc_default_role",
		EnvVar:          "OIDC_DEFAULT_ROLE",
		Type:            TypeSelect,
		DefaultValue:    "viewer",
		Category:        "oidc",
		Label:           "Rôle OIDC par défaut",
		Description:     "Rôle attribué à un utilisateur sans correspondance de groupe spécifique.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Options:         []string{"viewer", "operator", "admin"},
		Validate:        validateOptions([]string{"viewer", "operator", "admin"}),
	},
	{
		Key:             "OIDC_AUTO_CREATE_USER",
		SettingKey:      "oidc_auto_create_user",
		EnvVar:          "OIDC_AUTO_CREATE_USER",
		Type:            TypeBool,
		DefaultValue:    "true",
		Category:        "oidc",
		Label:           "Création automatique des comptes",
		Description:     "Créer automatiquement un compte local lors de la première connexion OIDC réussie.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateBool,
	},
	{
		Key:             "OIDC_ALLOW_LOCAL_LOGIN",
		SettingKey:      "oidc_allow_local_login",
		EnvVar:          "OIDC_ALLOW_LOCAL_LOGIN",
		Type:            TypeBool,
		DefaultValue:    "true",
		Category:        "oidc",
		Label:           "Autoriser l'authentification locale",
		Description:     "Garde actif le formulaire login/mot de passe classique même si OIDC est activé.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateBool,
	},
	{
		Key:             "OIDC_INSECURE_SKIP_VERIFY",
		SettingKey:      "oidc_insecure_skip_verify",
		EnvVar:          "OIDC_INSECURE_SKIP_VERIFY",
		Type:            TypeBool,
		DefaultValue:    "false",
		Category:        "oidc",
		Label:           "Ignorer la vérification SSL OIDC",
		Description:     "Désactive la vérification des certificats TLS de l'émetteur OIDC (déconseillé en production).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateBool,
	},

	// ==================== Notifications / Alerts ====================
	{
		Key:             "NOTIFY_URL",
		SettingKey:      "ntfy_url",
		EnvVar:          "NOTIFY_URL",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "URL du topic ntfy.sh",
		Description:     "URL complète du topic ntfy pour les alertes push instantanées (ex: https://ntfy.sh/mon-serveur).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "NTFY_AUTH_TOKEN",
		SettingKey:      "ntfy_auth_token",
		EnvVar:          "NTFY_AUTH_TOKEN",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "Jeton d'authentification ntfy",
		Description:     "Token d'accès optionnel pour les topics ntfy privés.",
		IsSecret:        true,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "SMTP_HOST",
		SettingKey:      "smtp_host",
		EnvVar:          "SMTP_HOST",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "Hôte SMTP",
		Description:     "Adresse du serveur de messagerie sortant pour l'envoi d'emails d'alerte.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "SMTP_PORT",
		SettingKey:      "smtp_port",
		EnvVar:          "SMTP_PORT",
		Type:            TypeInt,
		DefaultValue:    "587",
		Category:        "notifications",
		Label:           "Port SMTP",
		Description:     "Port de connexion SMTP (généralement 587 pour STARTTLS, 465 pour SMTPS).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePort,
	},
	{
		Key:             "SMTP_USER",
		SettingKey:      "smtp_user",
		EnvVar:          "SMTP_USER",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "Utilisateur SMTP",
		Description:     "Identifiant d'authentification auprès du serveur SMTP.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "SMTP_PASS",
		SettingKey:      "smtp_pass",
		EnvVar:          "SMTP_PASS",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "Mot de passe SMTP",
		Description:     "Mot de passe du compte SMTP pour l'envoi d'emails.",
		IsSecret:        true,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "SMTP_FROM",
		SettingKey:      "smtp_from",
		EnvVar:          "SMTP_FROM",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "Adresse expéditeur (From)",
		Description:     "Adresse affichée comme expéditeur des notifications email.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "SMTP_TO",
		SettingKey:      "smtp_to",
		EnvVar:          "SMTP_TO",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "notifications",
		Label:           "Adresse destinataire par défaut (To)",
		Description:     "Adresse recevant par défaut les alertes et rapports par email.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "SMTP_TLS",
		SettingKey:      "smtp_tls",
		EnvVar:          "SMTP_TLS",
		Type:            TypeBool,
		DefaultValue:    "true",
		Category:        "notifications",
		Label:           "Activer TLS pour SMTP",
		Description:     "Chiffre la communication avec le serveur SMTP via STARTTLS ou TLS direct.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateBool,
	},

	// ==================== Integrations ====================
	{
		Key:             "GITHUB_TOKEN",
		SettingKey:      "github_token",
		EnvVar:          "GITHUB_TOKEN",
		Type:            TypeString,
		DefaultValue:    "",
		Category:        "integrations",
		Label:           "Token d'accès personnel GitHub",
		Description:     "Jeton GitHub pour surveiller les nouvelles versions et éviter le rate-limiting de l'API publique.",
		IsSecret:        true,
		IsEditable:      true,
		RequiresRestart: false,
	},
	{
		Key:             "GITHUB_POLL_INTERVAL",
		SettingKey:      "github_poll_interval",
		EnvVar:          "GITHUB_POLL_INTERVAL",
		Type:            TypeDuration,
		DefaultValue:    "15m",
		Category:        "integrations",
		Label:           "Intervalle de scrutation GitHub",
		Description:     "Cadence de vérification des releases et tags GitHub distants.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateDuration,
	},
	{
		Key:             "DOCKER_IMAGE_POLL_INTERVAL",
		SettingKey:      "docker_image_poll_interval",
		EnvVar:          "DOCKER_IMAGE_POLL_INTERVAL",
		Type:            TypeDuration,
		DefaultValue:    "6h",
		Category:        "integrations",
		Label:           "Intervalle de scrutation des images Docker",
		Description:     "Cadence de vérification des nouveaux digests de conteneurs sur les registres distants.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateDuration,
	},

	// ==================== Retention ====================
	{
		Key:             "METRICS_RETENTION_DAYS",
		SettingKey:      "metrics_retention_days",
		EnvVar:          "METRICS_RETENTION_DAYS",
		Type:            TypeInt,
		DefaultValue:    "30",
		Category:        "retention",
		Label:           "Rétention des métriques (jours)",
		Description:     "Nombre de jours pendant lesquels les séries temporelles de métriques sont conservées.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveInt,
	},
	{
		Key:             "AUDIT_RETENTION_DAYS",
		SettingKey:      "audit_retention_days",
		EnvVar:          "AUDIT_RETENTION_DAYS",
		Type:            TypeInt,
		DefaultValue:    "90",
		Category:        "retention",
		Label:           "Rétention des journaux d'audit (jours)",
		Description:     "Durée de conservation des traces d'audit de sécurité et d'actions d'administration.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveInt,
	},
	{
		Key:             "WEB_LOGS_RETENTION_DAYS",
		SettingKey:      "web_logs_retention_days",
		EnvVar:          "web_logs_retention_days",
		Type:            TypeInt,
		DefaultValue:    "30",
		Category:        "retention",
		Label:           "Rétention des logs d'accès web (jours)",
		Description:     "Durée de conservation des requêtes HTTP capturées pour l'analyse de trafic et sécurité.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveInt,
	},
	{
		Key:             "NETWORK_FLOWS_RETENTION_DAYS",
		SettingKey:      "network_flows_retention_days",
		EnvVar:          "NETWORK_FLOWS_RETENTION_DAYS",
		Type:            TypeInt,
		DefaultValue:    "14",
		Category:        "retention",
		Label:           "Rétention des flux réseau (jours)",
		Description:     "Durée de conservation des métriques de connexions réseau (top talkers).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveInt,
	},

	// ==================== Threat Detection ====================
	{
		Key:             "THREAT_WEIGHT_WORDPRESS",
		SettingKey:      "threat_weight_wordpress",
		EnvVar:          "THREAT_WEIGHT_WORDPRESS",
		Type:            TypeFloat,
		DefaultValue:    "2",
		Category:        "threats",
		Label:           "Pondération des scans WordPress",
		Description:     "Poids attribué aux sondes ciblant wp-login, xmlrpc ou des chemins WordPress.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateNonNegativeFloat,
	},
	{
		Key:             "THREAT_WEIGHT_ADMIN_PANEL",
		SettingKey:      "threat_weight_adminpanel",
		EnvVar:          "THREAT_WEIGHT_ADMIN_PANEL",
		Type:            TypeFloat,
		DefaultValue:    "3",
		Category:        "threats",
		Label:           "Pondération panneaux d'admin",
		Description:     "Poids des tentatives d'accès aux interfaces administratives non autorisées.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateNonNegativeFloat,
	},
	{
		Key:             "THREAT_WEIGHT_PATH_TRAVERSAL",
		SettingKey:      "threat_weight_pathtraversal",
		EnvVar:          "THREAT_WEIGHT_PATH_TRAVERSAL",
		Type:            TypeFloat,
		DefaultValue:    "5",
		Category:        "threats",
		Label:           "Pondération traversée de répertoires",
		Description:     "Poids des requêtes contenant ../ ou des patterns d'évasion de chemin.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateNonNegativeFloat,
	},
	{
		Key:             "THREAT_WEIGHT_KNOWN_SCANNER",
		SettingKey:      "threat_weight_knownscanner",
		EnvVar:          "THREAT_WEIGHT_KNOWN_SCANNER",
		Type:            TypeFloat,
		DefaultValue:    "4",
		Category:        "threats",
		Label:           "Pondération scanners connus",
		Description:     "Poids des User-Agents identifiés comme outils de scan de vulnérabilités.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateNonNegativeFloat,
	},
	{
		Key:             "THREAT_WEIGHT_SUSPICIOUS_METHOD",
		SettingKey:      "threat_weight_suspiciousmethod",
		EnvVar:          "THREAT_WEIGHT_SUSPICIOUS_METHOD",
		Type:            TypeFloat,
		DefaultValue:    "2",
		Category:        "threats",
		Label:           "Pondération méthodes HTTP suspectes",
		Description:     "Poids des méthodes HTTP inhabituelles ou dangereuses (CONNECT, PROPFIND, etc.).",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validateNonNegativeFloat,
	},
	{
		Key:             "THREAT_THRESHOLD_MEDIUM",
		SettingKey:      "threat_threshold_medium",
		EnvVar:          "THREAT_THRESHOLD_MEDIUM",
		Type:            TypeFloat,
		DefaultValue:    "15",
		Category:        "threats",
		Label:           "Seuil de menace : Modéré",
		Description:     "Score total à partir duquel une adresse IP est classée comme menace modérée.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveFloat,
	},
	{
		Key:             "THREAT_THRESHOLD_HIGH",
		SettingKey:      "threat_threshold_high",
		EnvVar:          "THREAT_THRESHOLD_HIGH",
		Type:            TypeFloat,
		DefaultValue:    "50",
		Category:        "threats",
		Label:           "Seuil de menace : Élevé",
		Description:     "Score total à partir duquel une adresse IP est classée comme menace élevée.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveFloat,
	},
	{
		Key:             "THREAT_THRESHOLD_CRITICAL",
		SettingKey:      "threat_threshold_critical",
		EnvVar:          "THREAT_THRESHOLD_CRITICAL",
		Type:            TypeFloat,
		DefaultValue:    "150",
		Category:        "threats",
		Label:           "Seuil de menace : Critique",
		Description:     "Score total à partir duquel une adresse IP est classée comme menace critique.",
		IsSecret:        false,
		IsEditable:      true,
		RequiresRestart: false,
		Validate:        validatePositiveFloat,
	},
}

// ParamsByKey indexes AllParams by its canonical Key for fast lookup.
var ParamsByKey = func() map[string]ConfigParamDef {
	m := make(map[string]ConfigParamDef, len(AllParams))
	for _, p := range AllParams {
		m[p.Key] = p
		// Also allow lookup by SettingKey
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

package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
)

// Repository defines data access operations required for system configuration.
type Repository interface {
	GetAllSettings(ctx context.Context) (map[string]string, error)
	SetSetting(ctx context.Context, key, value string) error
	DeleteSetting(ctx context.Context, key string) error
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
}

// Service manages configuration reading, persistence, and audit logging.
type Service struct {
	repo Repository
	cfg  *config.Config
}

func NewService(repo Repository, cfg *config.Config) *Service {
	return &Service{
		repo: repo,
		cfg:  cfg,
	}
}

// GetEffectiveConfig returns the resolved multi-source configuration.
//
// OverrideFromDB is called here (idempotently — it just re-applies whatever
// is already in the DB to s.cfg) purely to obtain its returned set of
// setting keys a DB value actually takes precedence over ENV for. Without
// it, ResolveEffectiveConfig would have no way to know that a long-standing
// set of keys (SMTP, ntfy, GitHub, retention days, JWT/refresh durations,
// every OIDC_*/threat_* setting) already lets the DB win unconditionally,
// and would report "env" as their source whenever both are set — telling
// the admin their save was ignored when it was actually applied.
func (s *Service) GetEffectiveConfig(ctx context.Context, revealSecrets bool) (config.ConfigSummary, error) {
	settings, err := s.repo.GetAllSettings(ctx)
	if err != nil {
		return config.ConfigSummary{}, err
	}
	dbOverridable := s.cfg.OverrideFromDB(s.repo)
	return config.ResolveEffectiveConfig(settings, dbOverridable, revealSecrets), nil
}

// UpdateParam updates a single parameter in the persistent settings store.
// If an environment variable is present on the container, the update is stored
// but a warning is returned since the environment variable remains authoritative.
func (s *Service) UpdateParam(ctx context.Context, key, value, username, clientIP string) (*config.ConfigEntry, string, error) {
	param, ok := config.FindParam(key)
	if !ok {
		return nil, "", apperr.NotFound(fmt.Sprintf("paramètre de configuration inconnu : %s", key))
	}

	if !param.IsEditable {
		return nil, "", apperr.Validation(fmt.Sprintf("le paramètre %s est géré uniquement via l'environnement Docker", param.Key))
	}

	trimmedVal := strings.TrimSpace(value)

	// Avoid wiping existing secret if placeholder sentinel is sent
	if param.IsSecret && trimmedVal == config.MaskedSecretSentinel {
		summary, _ := s.GetEffectiveConfig(ctx, false)
		for _, e := range summary.Entries {
			if e.Key == param.Key {
				return &e, "", nil
			}
		}
	}

	if param.Validate != nil && trimmedVal != "" {
		if err := param.Validate(trimmedVal); err != nil {
			return nil, "", apperr.Validation(fmt.Sprintf("valeur invalide pour %s : %v", param.Key, err))
		}
	}

	// Persist to database
	if err := s.repo.SetSetting(ctx, param.SettingKey, trimmedVal); err != nil {
		return nil, "", apperr.Failed(fmt.Sprintf("erreur lors de l'enregistrement de %s : %v", param.Key, err))
	}

	// Reload config in memory
	s.cfg.OverrideFromDB(s.repo)

	// Prepare audit details with masked secrets
	var auditVal string
	if param.IsSecret {
		auditVal = "[REDACTED]"
	} else {
		auditVal = trimmedVal
	}
	details := fmt.Sprintf("Modification de configuration %s = %s", param.Key, auditVal)
	_, _ = s.repo.CreateAuditLog(ctx, username, "update_config", "", clientIP, details, "success")

	summary, err := s.GetEffectiveConfig(ctx, false)
	if err != nil {
		return nil, "", nil
	}

	for _, e := range summary.Entries {
		if e.Key == param.Key {
			return &e, configConflictWarning(e), nil
		}
	}

	return nil, "", nil
}

// configConflictWarning explains a save that didn't take effect the way the
// admin would expect, derived from the entry ResolveEffectiveConfig already
// computed rather than re-deriving env/DB precedence a second, independent
// way (see GetEffectiveConfig's doc comment for the bug that caused).
func configConflictWarning(e config.ConfigEntry) string {
	if !e.HasConflict {
		return ""
	}
	if e.Source == "env" {
		return fmt.Sprintf("La valeur a été enregistrée en base, mais la variable d'environnement Docker %s reste active et prioritaire.", e.EnvVar)
	}
	// Source == "ui": OverrideFromDB lets this key's DB value win over ENV —
	// the save took effect, but the env var is still set to a different
	// value and will look like the wrong one to whoever configured it.
	return fmt.Sprintf("La valeur enregistrée est active, mais diffère de la variable d'environnement Docker %s toujours définie sur le conteneur.", e.EnvVar)
}

// ResetParam removes the parameter's entry from persistent storage so it
// reverts to ENV or default, and reports (as a warning, not an error) when
// that revert won't be visible until the server restarts.
//
// OverrideFromDB only ever moves a field forward when it finds a DB value —
// it has no way to "unset" one it already applied on a previous call, since
// deleting the settings row just makes the DB lookup miss next time, it
// doesn't retroactively undo the assignment already made to *Config. So for
// a key OverrideFromDB was actively applying (checked here before the
// delete, via the same applied-keys map UpdateParam's conflict detection
// uses), the row is gone and the effective-config API will correctly show
// "env"/"default" going forward, but the already-running process keeps
// serving the old value from memory until restarted.
func (s *Service) ResetParam(ctx context.Context, key, username, clientIP string) (*config.ConfigEntry, string, error) {
	param, ok := config.FindParam(key)
	if !ok {
		return nil, "", apperr.NotFound(fmt.Sprintf("paramètre inconnu : %s", key))
	}

	if !param.IsEditable {
		return nil, "", apperr.Validation(fmt.Sprintf("le paramètre %s n'est pas modifiable", param.Key))
	}

	wasLiveViaDB := s.cfg.OverrideFromDB(s.repo)[param.SettingKey]

	if err := s.repo.DeleteSetting(ctx, param.SettingKey); err != nil {
		return nil, "", apperr.Failed(fmt.Sprintf("erreur lors de la réinitialisation de %s : %v", param.Key, err))
	}

	// Reload config in memory
	s.cfg.OverrideFromDB(s.repo)

	details := fmt.Sprintf("Réinitialisation du paramètre %s à sa valeur par défaut/ENV", param.Key)
	_, _ = s.repo.CreateAuditLog(ctx, username, "reset_config", "", clientIP, details, "success")

	var warning string
	if wasLiveViaDB {
		warning = fmt.Sprintf("%s a été réinitialisé en base, mais le processus en cours continue d'utiliser l'ancienne valeur enregistrée jusqu'au prochain redémarrage du serveur.", param.Key)
	}

	summary, err := s.GetEffectiveConfig(ctx, false)
	if err != nil {
		return nil, warning, nil
	}

	for _, e := range summary.Entries {
		if e.Key == param.Key {
			return &e, warning, nil
		}
	}

	return nil, warning, nil
}

// UpdateBulk updates multiple parameters in one request.
func (s *Service) UpdateBulk(ctx context.Context, updates map[string]string, username, clientIP string) (config.ConfigSummary, []string, error) {
	var warnings []string

	for key, val := range updates {
		_, warn, err := s.UpdateParam(ctx, key, val, username, clientIP)
		if err != nil {
			return config.ConfigSummary{}, nil, err
		}
		if warn != "" {
			warnings = append(warnings, warn)
		}
	}

	summary, err := s.GetEffectiveConfig(ctx, false)
	return summary, warnings, err
}

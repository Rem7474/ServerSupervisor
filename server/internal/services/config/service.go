package config

import (
	"context"
	"fmt"
	"os"
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
func (s *Service) GetEffectiveConfig(ctx context.Context, revealSecrets bool) (config.ConfigSummary, error) {
	settings, err := s.repo.GetAllSettings(ctx)
	if err != nil {
		return config.ConfigSummary{}, err
	}
	return config.ResolveEffectiveConfig(settings, revealSecrets), nil
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

	// Check if overridden by environment
	var warning string
	if envVal, envOk := os.LookupEnv(param.EnvVar); envOk && strings.TrimSpace(envVal) != "" {
		warning = fmt.Sprintf("La valeur a été enregistrée en base, mais la variable d'environnement Docker %s reste active et prioritaire.", param.EnvVar)
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

// ResetParam removes the parameter's entry from persistent storage so it reverts to ENV or default.
func (s *Service) ResetParam(ctx context.Context, key, username, clientIP string) (*config.ConfigEntry, error) {
	param, ok := config.FindParam(key)
	if !ok {
		return nil, apperr.NotFound(fmt.Sprintf("paramètre inconnu : %s", key))
	}

	if !param.IsEditable {
		return nil, apperr.Validation(fmt.Sprintf("le paramètre %s n'est pas modifiable", param.Key))
	}

	if err := s.repo.DeleteSetting(ctx, param.SettingKey); err != nil {
		return nil, apperr.Failed(fmt.Sprintf("erreur lors de la réinitialisation de %s : %v", param.Key, err))
	}

	// Reload config in memory
	s.cfg.OverrideFromDB(s.repo)

	details := fmt.Sprintf("Réinitialisation du paramètre %s à sa valeur par défaut/ENV", param.Key)
	_, _ = s.repo.CreateAuditLog(ctx, username, "reset_config", "", clientIP, details, "success")

	summary, err := s.GetEffectiveConfig(ctx, false)
	if err != nil {
		return nil, nil
	}

	for _, e := range summary.Entries {
		if e.Key == param.Key {
			return &e, nil
		}
	}

	return nil, nil
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

package config

import (
	"os"
	"strings"
)

// ConfigEntry represents the resolved, multi-source state of a configuration parameter.
type ConfigEntry struct {
	Key             string    `json:"key"`
	SettingKey      string    `json:"setting_key"`
	EnvVar          string    `json:"env_var"`
	Label           string    `json:"label"`
	Description     string    `json:"description"`
	Category        string    `json:"category"`
	Type            ParamType `json:"type"`
	Options         []string  `json:"options,omitempty"`
	DefaultValue    string    `json:"default_value"`
	EffectiveValue  string    `json:"effective_value"`
	Source          string    `json:"source"` // "env", "ui", "default"
	IsSecret        bool      `json:"is_secret"`
	IsEditable      bool      `json:"is_editable"`
	RequiresRestart bool      `json:"requires_restart"`
	HasEnvOverride  bool      `json:"has_env_override"`
	EnvValue        string    `json:"env_value,omitempty"`
	UIValue         string    `json:"ui_value,omitempty"`
	HasConflict     bool      `json:"has_conflict"`
}

// ConfigSummary encapsulates the complete configuration status and aggregation metrics.
type ConfigSummary struct {
	Entries       []ConfigEntry `json:"entries"`
	TotalParams   int           `json:"total_params"`
	EnvCount      int           `json:"env_count"`
	UICount       int           `json:"ui_count"`
	DefaultCount  int           `json:"default_count"`
	ConflictCount int           `json:"conflict_count"`
	Categories    []string      `json:"categories"`
}

const MaskedSecretSentinel = "••••••••"

// MaskValue masks secret values unless they are empty.
func MaskValue(val string) string {
	if strings.TrimSpace(val) == "" {
		return ""
	}
	return MaskedSecretSentinel
}

// ResolveEffectiveConfig computes the effective value and origin for every registered parameter.
// Priority rules:
//  1. ENV (if present in os.LookupEnv and non-empty) -> Source = "env"
//  2. UI / DB (if present in dbSettings and non-empty) -> Source = "ui"
//  3. Default (hardcoded fallback) -> Source = "default"
//
// A conflict is detected when BOTH ENV and UI have non-empty values that differ.
func ResolveEffectiveConfig(dbSettings map[string]string, revealSecrets bool) ConfigSummary {
	entries := make([]ConfigEntry, 0, len(AllParams))
	categorySet := make(map[string]bool)
	var envCount, uiCount, defaultCount, conflictCount int

	for _, param := range AllParams {
		categorySet[param.Category] = true

		// 1. Check container environment
		envRaw, envOk := os.LookupEnv(param.EnvVar)
		hasEnv := envOk && strings.TrimSpace(envRaw) != ""

		// 2. Check UI / DB persisted settings
		var uiRaw string
		var hasUI bool
		if dbSettings != nil {
			// Look up by setting_key (lowercase) then by param.Key
			if v, ok := dbSettings[param.SettingKey]; ok && strings.TrimSpace(v) != "" {
				uiRaw = v
				hasUI = true
			} else if v, ok := dbSettings[strings.ToLower(param.Key)]; ok && strings.TrimSpace(v) != "" {
				uiRaw = v
				hasUI = true
			}
		}

		// 3. Determine effective value and source
		var source string
		var rawEffective string
		var hasConflict bool

		if hasEnv {
			source = "env"
			rawEffective = envRaw
			envCount++
			if hasUI && strings.TrimSpace(uiRaw) != strings.TrimSpace(envRaw) {
				hasConflict = true
				conflictCount++
			}
		} else if hasUI {
			source = "ui"
			rawEffective = uiRaw
			uiCount++
		} else {
			source = "default"
			rawEffective = param.DefaultValue
			defaultCount++
		}

		// Masking logic
		effectiveVal := rawEffective
		envVal := envRaw
		uiVal := uiRaw
		defaultVal := param.DefaultValue

		if param.IsSecret && !revealSecrets {
			effectiveVal = MaskValue(rawEffective)
			if hasEnv {
				envVal = MaskValue(envRaw)
			}
			if hasUI {
				uiVal = MaskValue(uiRaw)
			}
			defaultVal = MaskValue(param.DefaultValue)
		}

		entry := ConfigEntry{
			Key:             param.Key,
			SettingKey:      param.SettingKey,
			EnvVar:          param.EnvVar,
			Label:           param.Label,
			Description:     param.Description,
			Category:        param.Category,
			Type:            param.Type,
			Options:         param.Options,
			DefaultValue:    defaultVal,
			EffectiveValue:  effectiveVal,
			Source:          source,
			IsSecret:        param.IsSecret,
			IsEditable:      param.IsEditable,
			RequiresRestart: param.RequiresRestart,
			HasEnvOverride:  hasEnv,
			EnvValue:        envVal,
			UIValue:         uiVal,
			HasConflict:     hasConflict,
		}

		entries = append(entries, entry)
	}

	categories := []string{
		"server",
		"logging",
		"network",
		"database",
		"auth",
		"oidc",
		"notifications",
		"integrations",
		"retention",
		"threats",
	}

	return ConfigSummary{
		Entries:       entries,
		TotalParams:   len(entries),
		EnvCount:      envCount,
		UICount:       uiCount,
		DefaultCount:  defaultCount,
		ConflictCount: conflictCount,
		Categories:    categories,
	}
}

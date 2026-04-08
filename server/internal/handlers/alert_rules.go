package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AlertRulesHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAlertRulesHandler(db *database.DB, cfg *config.Config) *AlertRulesHandler {
	return &AlertRulesHandler{db: db, cfg: cfg}
}

// alertRuleFieldLabel maps Go struct field names to human-readable French labels.
var alertRuleFieldLabel = map[string]string{
	"Name":      "Nom",
	"Metric":    "Metrique",
	"Operator":  "Operateur",
	"Threshold": "Seuil",
	"Duration":  "Duree",
	"Enabled":   "Active",
	"HostID":    "Hote",
}

var validAlertOperators = map[string]bool{">": true, "<": true, ">=": true, "<=": true}

var validAlertChannels = map[string]bool{
	"smtp":    true,
	"ntfy":    true,
	"browser": true,
	"notify":  true,
}

var commandModuleActions = map[string][]string{
	"docker":    {"logs", "restart", "start", "stop", "compose_up", "compose_down", "compose_pull", "compose_logs", "compose_restart"},
	"journal":   {"read"},
	"apt":       {"update", "upgrade", "full-upgrade", "autoremove"},
	"systemd":   {"status", "start", "stop", "restart", "list"},
	"processes": {"list"},
	"custom":    {"run"},
}

var commandModuleRequiresTarget = map[string]bool{
	"docker":  true,
	"journal": true,
	"systemd": true,
	"custom":  true,
}

var validAlertMetrics = map[string]bool{
	"cpu": true, "memory": true, "disk": true, "load": true, "heartbeat_timeout": true,
	"status_offline":  true,
	"cpu_temperature": true, "disk_smart_status": true, "disk_temperature": true, "proxmox_storage_percent": true,
	"proxmox_node_cpu_percent": true, "proxmox_node_memory_percent": true,
}

type alertMetricCapability struct {
	Metric             string `json:"metric"`
	Label              string `json:"label"`
	Unit               string `json:"unit"`
	Icon               string `json:"icon"`
	BadgeClass         string `json:"badge_class"`
	SupportsThreshold  bool   `json:"supports_threshold"`
	SupportsDuration   bool   `json:"supports_duration"`
	SupportsHostFilter bool   `json:"supports_host_filter"`
}

type alertScopeOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type alertHostCapabilitiesResponse struct {
	HostID   string                  `json:"host_id"`
	HostName string                  `json:"host_name"`
	Metrics  []alertMetricCapability `json:"metrics"`
}

type alertSplitCapabilitiesResponse struct {
	AgentMetrics   []alertMetricCapability `json:"agent_metrics"`
	ProxmoxMetrics []alertMetricCapability `json:"proxmox_metrics"`
	ProxmoxScope   struct {
		Modes       []string           `json:"modes"`
		Connections []alertScopeOption `json:"connections"`
		Nodes       []alertScopeOption `json:"nodes"`
		Storages    []alertScopeOption `json:"storages"`
	} `json:"proxmox_scope"`
}

func validateAlertRuleMetricOperator(metric, operator string) error {
	if !validAlertOperators[operator] {
		return errors.New("Operateur invalide.")
	}
	if !validAlertMetrics[metric] {
		return errors.New("Metrique invalide.")
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validateAlertActions(actions *models.AlertActions) error {
	if actions == nil {
		return nil
	}
	if actions.Cooldown < 0 {
		return errors.New("La periode de silence doit etre positive ou nulle.")
	}
	for _, channel := range actions.Channels {
		if !validAlertChannels[channel] {
			return fmt.Errorf("Canal de notification invalide: %s", channel)
		}
	}

	if actions.CommandTrigger != nil {
		ct := actions.CommandTrigger
		ct.Module = strings.TrimSpace(ct.Module)
		ct.Action = strings.TrimSpace(ct.Action)
		ct.Target = strings.TrimSpace(ct.Target)

		if ct.Module == "" || ct.Action == "" {
			return errors.New("Le declencheur de commande doit definir un module et une action.")
		}
		allowedActions, ok := commandModuleActions[ct.Module]
		if !ok {
			return fmt.Errorf("Module de commande invalide: %s", ct.Module)
		}
		if !containsString(allowedActions, ct.Action) {
			return fmt.Errorf("Action invalide pour le module %s: %s", ct.Module, ct.Action)
		}
		if commandModuleRequiresTarget[ct.Module] && ct.Target == "" {
			return fmt.Errorf("Le module %s requiert une cible.", ct.Module)
		}
		if !commandModuleRequiresTarget[ct.Module] {
			ct.Target = ""
		}
	}
	return nil
}

func validateProxmoxScopeExists(db *database.DB, scope *models.ProxmoxMetricScope) error {
	if scope == nil {
		return errors.New("Le scope Proxmox est requis.")
	}

	if scope.ScopeMode == "connection" {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM proxmox_connections WHERE id = $1)`, scope.ConnectionID).Scan(&exists); err != nil || !exists {
			return errors.New("Connexion Proxmox introuvable pour ce scope.")
		}
	}

	if scope.ScopeMode == "node" {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM proxmox_nodes WHERE id = $1)`, scope.NodeID).Scan(&exists); err != nil || !exists {
			return errors.New("Noeud Proxmox introuvable pour ce scope.")
		}
	}

	if scope.ScopeMode == "storage" {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM proxmox_storages WHERE id = $1)`, scope.StorageID).Scan(&exists); err != nil || !exists {
			return errors.New("Stockage Proxmox introuvable pour ce scope.")
		}
	}

	return nil
}

func normalizeRuleSourceType(source models.AlertSourceType, metric string) models.AlertSourceType {
	if source == "" {
		return models.InferAlertSourceType(metric)
	}
	return source
}

func (h *AlertRulesHandler) loadProxmoxScopeOptions() (modes []string, connections, nodes, storages []alertScopeOption) {
	modes = []string{"global", "connection", "node", "storage"}
	connections = []alertScopeOption{}
	nodes = []alertScopeOption{}
	storages = []alertScopeOption{}

	if rows, err := h.db.Query(`SELECT id, name FROM proxmox_connections ORDER BY name`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, name string
			if scanErr := rows.Scan(&id, &name); scanErr == nil {
				connections = append(connections, alertScopeOption{ID: id, Label: name})
			}
		}
	}

	if rows, err := h.db.Query(`
		SELECT n.id, COALESCE(c.name,'?') || ' / ' || n.node_name
		FROM proxmox_nodes n
		LEFT JOIN proxmox_connections c ON c.id = n.connection_id
		ORDER BY c.name, n.node_name`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, label string
			if scanErr := rows.Scan(&id, &label); scanErr == nil {
				nodes = append(nodes, alertScopeOption{ID: id, Label: label})
			}
		}
	}

	if rows, err := h.db.Query(`
		SELECT s.id, COALESCE(c.name,'?') || ' / ' || s.node_name || ' / ' || s.storage_name
		FROM proxmox_storages s
		LEFT JOIN proxmox_connections c ON c.id = s.connection_id
		ORDER BY c.name, s.node_name, s.storage_name`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, label string
			if scanErr := rows.Scan(&id, &label); scanErr == nil {
				storages = append(storages, alertScopeOption{ID: id, Label: label})
			}
		}
	}

	return modes, connections, nodes, storages
}

func (h *AlertRulesHandler) GetAgentAlertRuleCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"metrics": allAgentAlertMetrics()})
}

func (h *AlertRulesHandler) GetProxmoxAlertRuleCapabilities(c *gin.Context) {
	modes, connections, nodes, storages := h.loadProxmoxScopeOptions()
	response := alertSplitCapabilitiesResponse{AgentMetrics: []alertMetricCapability{}, ProxmoxMetrics: allProxmoxAlertMetrics()}
	response.ProxmoxScope.Modes = modes
	response.ProxmoxScope.Connections = connections
	response.ProxmoxScope.Nodes = nodes
	response.ProxmoxScope.Storages = storages
	c.JSON(http.StatusOK, response)
}

// allAlertMetrics returns the complete list of all available alert metrics.
func allAgentAlertMetrics() []alertMetricCapability {
	return []alertMetricCapability{
		{Metric: "cpu", Label: "CPU", Unit: "%", Icon: "\u26a1", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "cpu_temperature", Label: "Temp. CPU", Unit: "\u00b0C", Icon: "\U0001f321", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "memory", Label: "RAM", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "disk", Label: "Disque", Unit: "%", Icon: "\U0001f4be", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "load", Label: "Load avg", Unit: "", Icon: "\U0001f4c8", BadgeClass: "bg-purple-lt text-purple", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "heartbeat_timeout", Label: "Heartbeat", Unit: "s", Icon: "\U0001fac0", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "status_offline", Label: "Hote hors ligne", Unit: "", Icon: "\U0001f50c", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "disk_smart_status", Label: "SMART disque", Unit: "", Icon: "\U0001f6e1", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "disk_temperature", Label: "Temp. disque", Unit: "\u00b0C", Icon: "\U0001f321", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
	}
}

func allProxmoxAlertMetrics() []alertMetricCapability {
	return []alertMetricCapability{
		{Metric: "proxmox_storage_percent", Label: "Proxmox stockage", Unit: "%", Icon: "\U0001f5a5", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_cpu_percent", Label: "Proxmox CPU noeud", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_memory_percent", Label: "Proxmox RAM noeud", Unit: "%", Icon: "\U0001f4ca", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_guest_cpu_percent", Label: "CPU VM/LXC Proxmox", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_guest_memory_percent", Label: "RAM VM/LXC Proxmox", Unit: "%", Icon: "\U0001f4ca", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_pending_updates", Label: "Paquets APT en attente", Unit: "", Icon: "\U0001f504", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_security_updates", Label: "Mises à jour sécurité APT", Unit: "", Icon: "\U0001f512", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_recent_failed_tasks_24h", Label: "Tâches Proxmox échouées (24h)", Unit: "", Icon: "\U0001f552", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_disk_failed_count", Label: "Disques physiques en échec", Unit: "", Icon: "\U0001f4a5", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_disk_min_wearout_percent", Label: "Usure disque min", Unit: "%", Icon: "\U0001f6e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
	}
}

// filterMetricsByCollectors returns only metrics that are available on the host based on its enabled collectors.
// Collectors map example: {"docker": true, "smart": false, "cpu_temp": true, ...}
func filterMetricsByCollectors(allMetrics []alertMetricCapability, collectors map[string]bool) []alertMetricCapability {
	// These metrics are always available (base system metrics)
	alwaysAvailable := map[string]bool{
		"cpu":               true,
		"memory":            true,
		"disk":              true,
		"load":              true,
		"heartbeat_timeout": true,
		"status_offline":    true,
	}

	// These metrics require specific collectors
	requiresCollector := map[string]string{
		"cpu_temperature":   "cpu_temp",
		"disk_smart_status": "smart",
		"disk_temperature":  "smart",
	}

	var filtered []alertMetricCapability
	for _, metric := range allMetrics {
		// Always include base metrics
		if alwaysAvailable[metric.Metric] {
			filtered = append(filtered, metric)
			continue
		}

		// Check if metric requires a specific collector
		if requiredCollector, ok := requiresCollector[metric.Metric]; ok {
			// Check if required collector is enabled
			if collectors[requiredCollector] {
				filtered = append(filtered, metric)
			}
		}
	}

	return filtered
}

// GetHostAlertMetrics returns alert metrics available for a specific host based on its enabled collectors.
func (h *AlertRulesHandler) GetHostAlertMetrics(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostId parameter is required"})
		return
	}

	// Fetch the host to get collectors
	host, err := h.db.GetHost(hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}

	// Build response with filtered metrics
	response := alertHostCapabilitiesResponse{
		HostID:   host.ID,
		HostName: host.Name,
		Metrics:  filterMetricsByCollectors(allAgentAlertMetrics(), host.Collectors),
	}

	c.JSON(http.StatusOK, response)
}

// alertRuleTagMessage returns a human-readable message for a validator tag.
func alertRuleTagMessage(field, tag string) string {
	label, ok := alertRuleFieldLabel[field]
	if !ok {
		label = field
	}
	switch tag {
	case "required":
		return fmt.Sprintf("Le champ %s est obligatoire.", label)
	case "min":
		return fmt.Sprintf("Le champ %s est trop court.", label)
	case "max":
		return fmt.Sprintf("Le champ %s est trop long.", label)
	case "email":
		return fmt.Sprintf("Le champ %s doit etre une adresse e-mail valide.", label)
	default:
		return fmt.Sprintf("Le champ %s est invalide.", label)
	}
}

// humanizeValidationError converts a go-playground/validator error into a
// single readable string. Falls back to the raw error for non-validation errors.
func humanizeValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	if len(ve) == 1 {
		return alertRuleTagMessage(ve[0].Field(), ve[0].Tag())
	}
	msg := "Plusieurs champs sont invalides :"
	for _, fe := range ve {
		msg += " " + alertRuleTagMessage(fe.Field(), fe.Tag()) + ";"
	}
	return msg
}

// scanAlertRule scans a single alert rule row from the DB.
// Expected column order: id, name, enabled, source_type, host_id, proxmox_scope,
// metric, operator, threshold, duration_seconds, actions, last_fired, created_at, updated_at
func scanAlertRule(row interface {
	Scan(dest ...interface{}) error
}) (models.AlertRule, error) {
	var rule models.AlertRule
	var name, hostID, sourceType sql.NullString
	var threshold sql.NullFloat64
	var actionsJSON, proxmoxScopeJSON []byte
	var lastFired, updatedAt sql.NullTime

	err := row.Scan(
		&rule.ID, &name, &rule.Enabled, &sourceType, &hostID, &proxmoxScopeJSON, &rule.Metric,
		&rule.Operator, &threshold, &rule.DurationSeconds,
		&actionsJSON, &lastFired, &rule.CreatedAt, &updatedAt,
	)
	if err != nil {
		return rule, err
	}

	if name.Valid {
		rule.Name = &name.String
	}
	if hostID.Valid {
		rule.HostID = &hostID.String
	}
	if sourceType.Valid {
		rule.SourceType = models.AlertSourceType(sourceType.String)
	}
	if threshold.Valid {
		rule.Threshold = &threshold.Float64
	}
	if lastFired.Valid {
		rule.LastFired = &lastFired.Time
	}
	if updatedAt.Valid {
		rule.UpdatedAt = &updatedAt.Time
	}
	if len(actionsJSON) > 0 {
		_ = json.Unmarshal(actionsJSON, &rule.Actions)
	}
	if len(proxmoxScopeJSON) > 0 {
		_ = json.Unmarshal(proxmoxScopeJSON, &rule.ProxmoxScope)
	}
	if rule.Actions.Channels == nil {
		rule.Actions.Channels = []string{}
	}
	rule.NormalizeCompatibility()
	return rule, nil
}

const alertRuleSelectCols = `
id, name, enabled, source_type, host_id, proxmox_scope, metric, operator, threshold, duration_seconds,
actions, last_fired, created_at, updated_at`

// ListAlertRules returns all alert rules
func (h *AlertRulesHandler) ListAlertRules(c *gin.Context) {
	rows, err := h.db.Query(`SELECT` + alertRuleSelectCols + `
FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer func() { _ = rows.Close() }()

	rules := []models.AlertRule{}
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rules = append(rules, rule)
	}
	c.JSON(http.StatusOK, rules)
}

// GetAlertRule returns a single alert rule by ID
func (h *AlertRulesHandler) GetAlertRule(c *gin.Context) {
	id := c.Param("id")
	row := h.db.QueryRow(`SELECT`+alertRuleSelectCols+`
FROM alert_rules WHERE id = $1`, id)

	rule, err := scanAlertRule(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// CreateAlertRule creates a new alert rule
func (h *AlertRulesHandler) CreateAlertRule(c *gin.Context) {
	var req models.AlertRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}

	req.SourceType = normalizeRuleSourceType(req.SourceType, req.Metric)

	if err := validateAlertRuleMetricOperator(req.Metric, req.Operator); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateAlertActions(&req.Actions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Actions.Channels == nil {
		req.Actions.Channels = []string{}
	}

	var rule models.AlertRule
	name := req.Name
	rule.Name = &name
	rule.Enabled = req.Enabled
	rule.SourceType = req.SourceType
	rule.HostID = req.HostID
	rule.ProxmoxScope = req.ProxmoxScope
	rule.Metric = req.Metric
	rule.Operator = req.Operator
	threshold := req.Threshold
	rule.Threshold = &threshold
	rule.DurationSeconds = req.Duration
	rule.Actions = req.Actions

	if err := rule.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rule.SourceType == models.AlertSourceProxmox {
		if err := validateProxmoxScopeExists(h.db, rule.ProxmoxScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	actionsJSON, _ := json.Marshal(rule.Actions)
	proxmoxScopeJSON, _ := json.Marshal(rule.ProxmoxScope)

	err := h.db.QueryRow(`
INSERT INTO alert_rules (name, enabled, source_type, host_id, proxmox_scope, metric, operator, threshold, duration_seconds, actions)
VALUES ($1, $2, $3, $4, CAST($5 AS JSONB), $6, $7, $8, $9, CAST($10 AS JSONB))
RETURNING id, created_at, updated_at`,
		req.Name, req.Enabled, rule.SourceType, rule.HostID, string(proxmoxScopeJSON), rule.Metric, rule.Operator,
		req.Threshold, req.Duration, string(actionsJSON),
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// UpdateAlertRule updates an existing alert rule
func (h *AlertRulesHandler) UpdateAlertRule(c *gin.Context) {
	id := c.Param("id")

	var req models.AlertRuleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}

	row := h.db.QueryRow(`SELECT`+alertRuleSelectCols+`
FROM alert_rules WHERE id = $1`, id)
	existing, err := scanAlertRule(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Regle d'alerte introuvable."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.SourceType != nil && *req.SourceType != existing.SourceType {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Le changement de source_type n'est pas autorise."})
		return
	}

	next := existing
	if req.Name != nil {
		next.Name = req.Name
	}
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.HostID != nil {
		next.HostID = req.HostID
	}
	if req.Metric != nil {
		next.Metric = *req.Metric
	}
	if req.Operator != nil {
		next.Operator = *req.Operator
	}
	if req.Threshold != nil {
		next.Threshold = req.Threshold
	}
	if req.Duration != nil {
		next.DurationSeconds = *req.Duration
	}
	if req.Actions != nil {
		next.Actions = *req.Actions
	}
	if req.ProxmoxScope != nil {
		next.ProxmoxScope = req.ProxmoxScope
	}

	if err := validateAlertRuleMetricOperator(next.Metric, next.Operator); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateAlertActions(&next.Actions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := next.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if next.SourceType == models.AlertSourceProxmox {
		if err := validateProxmoxScopeExists(h.db, next.ProxmoxScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	actionsJSON, _ := json.Marshal(next.Actions)
	proxmoxScopeJSON, _ := json.Marshal(next.ProxmoxScope)

	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+strconv.Itoa(argCount))
		args = append(args, *next.Name)
		argCount++
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = $"+strconv.Itoa(argCount))
		args = append(args, next.Enabled)
		argCount++
	}
	if req.HostID != nil {
		updates = append(updates, "host_id = $"+strconv.Itoa(argCount))
		args = append(args, next.HostID)
		argCount++
	}
	if req.ProxmoxScope != nil {
		updates = append(updates, "proxmox_scope = CAST($"+strconv.Itoa(argCount)+" AS JSONB)")
		args = append(args, string(proxmoxScopeJSON))
		argCount++
	}
	if req.Metric != nil {
		if !validAlertMetrics[next.Metric] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Metrique invalide."})
			return
		}
		updates = append(updates, "metric = $"+strconv.Itoa(argCount))
		args = append(args, next.Metric)
		argCount++
	}
	if req.Operator != nil {
		if !validAlertOperators[next.Operator] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Operateur invalide."})
			return
		}
		updates = append(updates, "operator = $"+strconv.Itoa(argCount))
		args = append(args, next.Operator)
		argCount++
	}
	if req.Threshold != nil {
		updates = append(updates, "threshold = $"+strconv.Itoa(argCount))
		args = append(args, *next.Threshold)
		argCount++
	}
	if req.Duration != nil {
		updates = append(updates, "duration_seconds = $"+strconv.Itoa(argCount))
		args = append(args, next.DurationSeconds)
		argCount++
	}
	if req.Actions != nil {
		updates = append(updates, "actions = CAST($"+strconv.Itoa(argCount)+" AS JSONB)")
		args = append(args, string(actionsJSON))
		argCount++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aucun champ a mettre a jour."})
		return
	}

	updates = append(updates, "updated_at = NOW()")
	args = append(args, id)

	query := "UPDATE alert_rules SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += " WHERE id = $" + strconv.Itoa(argCount)

	result, err := h.db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Regle d'alerte introuvable."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule updated"})
}

// DeleteAlertRule deletes an alert rule
func (h *AlertRulesHandler) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM alert_rules WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Regle d'alerte introuvable."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted"})
}

// TestAlertRule evaluates a rule against current metrics without saving it.
func (h *AlertRulesHandler) TestAlertRule(c *gin.Context) {
	var req struct {
		SourceType   models.AlertSourceType     `json:"source_type"`
		HostID       *string                    `json:"host_id"`
		ProxmoxScope *models.ProxmoxMetricScope `json:"proxmox_scope"`
		Metric       string                     `json:"metric" binding:"required"`
		Operator     string                     `json:"operator" binding:"required"`
		Threshold    float64                    `json:"threshold" binding:"required"`
		Duration     int                        `json:"duration"`
		Actions      models.AlertActions        `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}
	if err := validateAlertRuleMetricOperator(req.Metric, req.Operator); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SourceType == "" {
		req.SourceType = models.InferAlertSourceType(req.Metric)
	}

	if err := validateAlertActions(&req.Actions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	threshold := req.Threshold
	rule := models.AlertRule{
		SourceType:      req.SourceType,
		HostID:          req.HostID,
		ProxmoxScope:    req.ProxmoxScope,
		Metric:          req.Metric,
		Operator:        req.Operator,
		Threshold:       &threshold,
		DurationSeconds: req.Duration,
		Actions:         req.Actions,
		Enabled:         true,
	}
	if err := rule.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rule.SourceType == models.AlertSourceProxmox {
		if err := validateProxmoxScopeExists(h.db, rule.ProxmoxScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	hosts, err := h.db.GetAllHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hosts"})
		return
	}

	type TestResult struct {
		HostID       string  `json:"host_id"`
		HostName     string  `json:"host_name"`
		CurrentValue float64 `json:"current_value"`
		WouldFire    bool    `json:"would_fire"`
		HasData      bool    `json:"has_data"`
	}

	var results []TestResult
	anyFires := false

	ruleNoStaleness := rule
	ruleNoStaleness.DurationSeconds = 0

	for _, host := range hosts {
		if rule.HostID != nil && *rule.HostID != host.ID {
			continue
		}
		value, ok := alerts.GetMetricValue(h.db, host, ruleNoStaleness)
		_, freshOk := alerts.GetMetricValue(h.db, host, rule)
		wouldFire := ok && freshOk && alerts.MatchRule(rule, host, value)
		if wouldFire {
			anyFires = true
		}
		results = append(results, TestResult{
			HostID:       host.ID,
			HostName:     host.Name,
			CurrentValue: value,
			WouldFire:    wouldFire,
			HasData:      ok,
		})
	}

	if results == nil {
		results = []TestResult{}
	}
	c.JSON(http.StatusOK, gin.H{
		"any_fires":    anyFires,
		"evaluated_at": time.Now(),
		"results":      results,
	})
}

// ListIncidents returns all alert incidents with pagination
func (h *AlertRulesHandler) ListIncidents(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}

	offset := (page - 1) * limit
	incidents, err := h.db.GetAlertIncidents(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch incidents"})
		return
	}
	if incidents == nil {
		incidents = []models.AlertIncident{}
	}
	c.JSON(http.StatusOK, incidents)
}

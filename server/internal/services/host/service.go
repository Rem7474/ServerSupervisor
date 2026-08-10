// Package host is the application/service layer for host management. It owns the
// host business logic (API-key generation, IP validation, agent-update conflict
// rules + dispatch, the page-load aggregations) behind a Repository and a
// Dispatcher port, so it is unit-testable without a database and the HTTP handler
// only does role authz + request/response translation.
package host

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/safego"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	RegisterHost(ctx context.Context, host *models.Host) error
	GetAllHosts(ctx context.Context) ([]models.Host, error)
	GetHost(ctx context.Context, id string) (*models.Host, error)
	UpdateHost(ctx context.Context, id string, req *models.HostUpdate) error
	DeleteHost(ctx context.Context, id string) error
	UpdateHostAPIKey(ctx context.Context, id, hashedKey string) error
	GetRemoteCommandsByHostAndModule(ctx context.Context, hostID, module string, limit int) ([]models.RemoteCommand, error)
	GetLatestMetrics(ctx context.Context, hostID string) (*models.SystemMetrics, error)
	GetEffectiveHostCPUTemperature(ctx context.Context, hostID string, fallbackLocal float64) (float64, bool)
	GetDockerContainers(ctx context.Context, hostID string) ([]models.DockerContainer, error)
	GetAptStatus(ctx context.Context, hostID string) (*models.AptStatus, error)
	GetLatestDiskMetrics(ctx context.Context, hostID string) ([]models.DiskMetrics, error)
	GetLatestDiskHealth(ctx context.Context, hostID string) ([]models.DiskHealth, error)
	GetDiskMetricsHistory(ctx context.Context, hostID, mountPoint string, limit int) ([]models.DiskMetrics, error)
	GetDiskMetricsAggregated(ctx context.Context, hostID, mountPoint string, hours int) ([]models.DiskMetrics, string, error)
	GetRecentCommandsByHost(ctx context.Context, hostID string, limit int) ([]models.RemoteCommand, error)
	GetHostExposure(ctx context.Context, ip string, since time.Time) (*models.HostExposure, error)
	GetLatestNetworkFlowMetrics(ctx context.Context, hostID string) ([]models.NetworkFlowMetric, error)
	GetNetworkFlowsHistory(ctx context.Context, hostID, remoteIP string, remotePort int, protocol string, hours int) ([]models.NetworkFlowSummaryPoint, error)
	GetNetworkFlowsSummary(ctx context.Context, hostID string, hours int) ([]models.NetworkFlowSummaryPoint, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the host use-cases.
type Service struct {
	repo          Repository
	dispatcher    Dispatcher
	latestVersion func() string
	bus           *events.Bus
}

// NewService wires the service. latestVersion resolves the latest agent release
// (injected so the service stays decoupled from the gitprovider/cache). bus is the
// event bus used to wake live snapshots after host changes (nil-safe).
func NewService(repo Repository, dispatcher Dispatcher, latestVersion func() string, bus *events.Bus) *Service {
	return &Service{repo: repo, dispatcher: dispatcher, latestVersion: latestVersion, bus: bus}
}

// publishHostList wakes the snapshots whose host lists change on register/delete.
func (s *Service) publishHostList() {
	s.bus.PublishAll(events.TopicDashboard, events.TopicNetwork, events.TopicApt)
}

// generateAPIKey creates a new API key pair for a host. The plain key (returned
// to the caller once) is "{hostID}.{secret}"; the stored key is a bcrypt hash.
func generateAPIKey(hostID string) (plainKey, hashedKey string, err error) {
	secret := uuid.New().String()
	hashedKey, err = database.HashAPIKey(secret)
	if err != nil {
		return "", "", err
	}
	return hostID + "." + secret, hashedKey, nil
}

// Register validates the request, generates an API key and stores the host.
// Returns the new host id and the plain API key (shown once).
func (s *Service) Register(ctx context.Context, req models.HostRegistration) (id, plainKey string, err error) {
	if net.ParseIP(req.IPAddress) == nil {
		return "", "", apperr.Validation("invalid IP address format")
	}
	hostID := uuid.New().String()
	plain, hashed, err := generateAPIKey(hostID)
	if err != nil {
		return "", "", err
	}
	host := &models.Host{
		ID:        hostID,
		Name:      req.Name,
		IPAddress: req.IPAddress,
		APIKey:    hashed,
		Tags:      req.Tags,
		Status:    "offline",
	}
	if err := s.repo.RegisterHost(ctx, host); err != nil {
		return "", "", err
	}
	s.publishHostList()
	return hostID, plain, nil
}

// BulkResult reports one entry's outcome in a bulk registration.
type BulkResult struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Created   bool   `json:"created"`
	HostID    string `json:"host_id,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RegisterBulk registers many hosts in one call (e.g. after a subnet
// discovery scan); each entry is validated and registered independently so
// one bad entry doesn't fail the whole batch.
func (s *Service) RegisterBulk(ctx context.Context, reqs []models.HostRegistration) (int, []BulkResult, error) {
	if len(reqs) == 0 {
		return 0, nil, apperr.Validation("hosts array is required")
	}
	if len(reqs) > 254 {
		return 0, nil, apperr.Validation("too many hosts (max 254)")
	}
	results := make([]BulkResult, 0, len(reqs))
	created := 0
	for _, req := range reqs {
		id, plainKey, err := s.Register(ctx, req)
		if err != nil {
			results = append(results, BulkResult{Name: req.Name, IPAddress: req.IPAddress, Error: err.Error()})
			continue
		}
		created++
		results = append(results, BulkResult{Name: req.Name, IPAddress: req.IPAddress, Created: true, HostID: id, APIKey: plainKey})
	}
	return created, results, nil
}

// List returns all hosts (never nil).
func (s *Service) List(ctx context.Context) ([]models.Host, error) {
	hosts, err := s.repo.GetAllHosts(ctx)
	if err != nil {
		return nil, err
	}
	if hosts == nil {
		hosts = []models.Host{}
	}
	return hosts, nil
}

// Get returns a host, or apperr.NotFound when absent.
func (s *Service) Get(ctx context.Context, id string) (*models.Host, error) {
	host, err := s.repo.GetHost(ctx, id)
	if err != nil {
		return nil, apperr.NotFound("host not found")
	}
	return host, nil
}

// Update applies the editable fields and returns the stored host.
func (s *Service) Update(ctx context.Context, id string, req models.HostUpdate) (*models.Host, error) {
	if req.Name == nil && req.Hostname == nil && req.IPAddress == nil && req.OS == nil && req.Tags == nil {
		return nil, apperr.Validation("no fields to update")
	}
	if err := s.repo.UpdateHost(ctx, id, &req); err != nil {
		return nil, err
	}
	s.bus.PublishAll(events.TopicDashboard, events.TopicNetwork, events.TopicApt)
	s.bus.Publish(events.HostTopic(id))
	return s.repo.GetHost(ctx, id)
}

// Delete removes a host.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.DeleteHost(ctx, id); err != nil {
		return err
	}
	s.publishHostList()
	s.bus.Publish(events.HostTopic(id))
	return nil
}

// RotateKey regenerates the host's API key and returns the new plain key.
func (s *Service) RotateKey(ctx context.Context, id string) (string, error) {
	plain, hashed, err := generateAPIKey(id)
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdateHostAPIKey(ctx, id, hashed); err != nil {
		return "", err
	}
	return plain, nil
}

// TriggerAgentUpdate queues an agent self-update, rejecting when the agent is
// already current or an update is already in flight. Returns the command id and
// the target version.
func (s *Service) TriggerAgentUpdate(ctx context.Context, id, username, clientIP string) (commandID, targetVersion string, err error) {
	host, err := s.Get(ctx, id)
	if err != nil {
		return "", "", err
	}
	target := s.latestVersion()
	if host.AgentVersion != "" && host.AgentVersion == target {
		return "", "", apperr.Conflict("agent is already up to date")
	}
	if cmds, e := s.repo.GetRemoteCommandsByHostAndModule(ctx, id, "agent", 20); e == nil {
		for _, cmd := range cmds {
			if cmd.Action == "update" && (cmd.Status == "pending" || cmd.Status == "running") {
				return "", "", apperr.Conflict("an agent update is already in progress for this host")
			}
		}
	}
	payload, err := json.Marshal(map[string]string{"version": target})
	if err != nil {
		return "", "", err
	}
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      id,
		Module:      "agent",
		Action:      "update",
		Payload:     string(payload),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "agent_update",
			HostID:    id,
			IPAddress: clientIP,
			Details:   "agent update to v" + target,
		},
	})
	if err != nil {
		return "", "", err
	}
	return result.Command.ID, target, nil
}

// HostComplete is the comprehensive host snapshot for initial page load.
type HostComplete struct {
	Host               *models.Host               `json:"host"`
	Metrics            *models.SystemMetrics      `json:"metrics"`
	Containers         []models.DockerContainer   `json:"containers"`
	AptStatus          *models.AptStatus          `json:"apt_status"`
	DiskMetrics        []models.DiskMetrics       `json:"disk_metrics"`
	DiskHealth         []models.DiskHealth        `json:"disk_health"`
	NetworkFlows       []models.NetworkFlowMetric `json:"network_flows"`
	CommandHistory     []models.RemoteCommand     `json:"command_history"`
	LatestAgentVersion string                     `json:"latest_agent_version"`
}

// Complete returns the page-load snapshot, running the independent reads in
// parallel.
func (s *Service) Complete(ctx context.Context, id string) (*HostComplete, error) {
	host, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var (
		metrics      *models.SystemMetrics
		containers   []models.DockerContainer
		aptStatus    *models.AptStatus
		diskMetrics  []models.DiskMetrics
		diskHealth   []models.DiskHealth
		networkFlows []models.NetworkFlowMetric
		cmdHistory   []models.RemoteCommand
	)
	var wg sync.WaitGroup
	wg.Add(7)
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.metrics")
		metrics, _ = s.repo.GetLatestMetrics(ctx, id)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.containers")
		containers, _ = s.repo.GetDockerContainers(ctx, id)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.aptStatus")
		aptStatus, _ = s.repo.GetAptStatus(ctx, id)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.diskMetrics")
		diskMetrics, _ = s.repo.GetLatestDiskMetrics(ctx, id)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.diskHealth")
		diskHealth, _ = s.repo.GetLatestDiskHealth(ctx, id)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.networkFlows")
		networkFlows, _ = s.repo.GetLatestNetworkFlowMetrics(ctx, id)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "host.Complete.cmdHistory")
		cmdHistory, _ = s.repo.GetRecentCommandsByHost(ctx, id, 20)
	}()
	wg.Wait()

	s.resolveTemp(ctx, id, metrics)
	return &HostComplete{
		Host:               host,
		Metrics:            metrics,
		Containers:         nonNilContainers(containers),
		AptStatus:          aptStatus,
		DiskMetrics:        nonNilDiskMetrics(diskMetrics),
		DiskHealth:         nonNilDiskHealth(diskHealth),
		NetworkFlows:       nonNilNetworkFlows(networkFlows),
		CommandHistory:     nonNilCommands(cmdHistory),
		LatestAgentVersion: s.latestVersion(),
	}, nil
}

// HostDashboard is the lighter host overview (metrics + docker + apt).
type HostDashboard struct {
	Host       *models.Host             `json:"host"`
	Metrics    *models.SystemMetrics    `json:"metrics"`
	Containers []models.DockerContainer `json:"containers"`
	AptStatus  *models.AptStatus        `json:"apt_status"`
}

// Dashboard returns the lighter host overview.
func (s *Service) Dashboard(ctx context.Context, id string) (*HostDashboard, error) {
	host, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	metrics, _ := s.repo.GetLatestMetrics(ctx, id)
	s.resolveTemp(ctx, id, metrics)
	containers, _ := s.repo.GetDockerContainers(ctx, id)
	aptStatus, _ := s.repo.GetAptStatus(ctx, id)
	return &HostDashboard{Host: host, Metrics: metrics, Containers: containers, AptStatus: aptStatus}, nil
}

// Exposure returns the NPM-domain correlation for a host: every proxy host
// that forwards to its IP address, enriched with aggregated web-log traffic
// for those domains over the given period. The domains routing to a host are
// invisible from the host's own detail page today (that data is collected and
// scoped under the reverse-proxy host, not this one) — this stitches it back
// together by IP, since that is the only link NPM's own config gives us.
func (s *Service) Exposure(ctx context.Context, id string, period time.Duration) (*models.HostExposure, error) {
	h, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	exposure, err := s.repo.GetHostExposure(ctx, h.IPAddress, time.Now().Add(-period))
	if err != nil {
		return nil, err
	}
	exposure.HostID = id
	return exposure, nil
}

// DiskMetrics returns the latest disk metrics for a host (never nil), each
// enriched with a best-effort saturation forecast (see attachDiskForecast).
func (s *Service) DiskMetrics(ctx context.Context, id string) ([]models.DiskMetrics, error) {
	m, err := s.repo.GetLatestDiskMetrics(ctx, id)
	if err != nil {
		return nil, err
	}
	metrics := nonNilDiskMetrics(m)
	for i := range metrics {
		s.attachDiskForecast(ctx, id, &metrics[i])
	}
	return metrics, nil
}

// attachDiskForecast fills ForecastDaysUntilFull from the mount point's last
// 30 days of usage trend. Best-effort: a failure to fetch trend data just
// leaves the forecast unset, it never fails the whole disk-metrics response.
func (s *Service) attachDiskForecast(ctx context.Context, hostID string, m *models.DiskMetrics) {
	trend, _, err := s.repo.GetDiskMetricsAggregated(ctx, hostID, m.MountPoint, 24*30)
	if err != nil {
		return
	}
	if days, ok := forecastDiskDaysUntilFull(trend); ok {
		m.ForecastDaysUntilFull = &days
	}
}

// DiskMetricsHistory returns a mount point's recent samples (never nil).
func (s *Service) DiskMetricsHistory(ctx context.Context, id, mountPoint string, limit int) ([]models.DiskMetrics, error) {
	h, err := s.repo.GetDiskMetricsHistory(ctx, id, mountPoint, limit)
	if err != nil {
		return nil, err
	}
	return nonNilDiskMetrics(h), nil
}

// DiskMetricsAggregated returns a mount point's adaptively bucketed history and
// the chosen aggregation type (points never nil).
func (s *Service) DiskMetricsAggregated(ctx context.Context, id, mountPoint string, hours int) ([]models.DiskMetrics, string, error) {
	points, aggType, err := s.repo.GetDiskMetricsAggregated(ctx, id, mountPoint, hours)
	if err != nil {
		return nil, "", err
	}
	return nonNilDiskMetrics(points), aggType, nil
}

// DiskHealth returns the SMART health of a host's disks (never nil).
func (s *Service) DiskHealth(ctx context.Context, id string) ([]models.DiskHealth, error) {
	h, err := s.repo.GetLatestDiskHealth(ctx, id)
	if err != nil {
		return nil, err
	}
	return nonNilDiskHealth(h), nil
}

// NetworkFlows returns the host's most recent report cycle's top talkers
// (never nil) — the "others" rollup row, if any, sorted last.
func (s *Service) NetworkFlows(ctx context.Context, id string) ([]models.NetworkFlowMetric, error) {
	m, err := s.repo.GetLatestNetworkFlowMetrics(ctx, id)
	if err != nil {
		return nil, err
	}
	return nonNilNetworkFlows(m), nil
}

// NetworkFlowsHistory returns one talker's bandwidth over time (never nil),
// for the per-talker drill-down chart.
func (s *Service) NetworkFlowsHistory(ctx context.Context, id, remoteIP string, remotePort int, protocol string, hours int) ([]models.NetworkFlowSummaryPoint, error) {
	points, err := s.repo.GetNetworkFlowsHistory(ctx, id, remoteIP, remotePort, protocol, hours)
	if err != nil {
		return nil, err
	}
	if points == nil {
		points = []models.NetworkFlowSummaryPoint{}
	}
	return points, nil
}

// NetworkFlowsSummary returns a host's total tracked bandwidth over time
// (never nil), for the overview chart.
func (s *Service) NetworkFlowsSummary(ctx context.Context, id string, hours int) ([]models.NetworkFlowSummaryPoint, error) {
	points, err := s.repo.GetNetworkFlowsSummary(ctx, id, hours)
	if err != nil {
		return nil, err
	}
	if points == nil {
		points = []models.NetworkFlowSummaryPoint{}
	}
	return points, nil
}

// resolveTemp overrides the agent-reported CPU temperature with the effective
// (sensor-source) one when available.
func (s *Service) resolveTemp(ctx context.Context, id string, metrics *models.SystemMetrics) {
	if metrics == nil {
		return
	}
	if temp, ok := s.repo.GetEffectiveHostCPUTemperature(ctx, id, metrics.CPUTemperature); ok {
		metrics.CPUTemperature = temp
	}
}

func nonNilContainers(v []models.DockerContainer) []models.DockerContainer {
	if v == nil {
		return []models.DockerContainer{}
	}
	return v
}
func nonNilDiskMetrics(v []models.DiskMetrics) []models.DiskMetrics {
	if v == nil {
		return []models.DiskMetrics{}
	}
	return v
}
func nonNilDiskHealth(v []models.DiskHealth) []models.DiskHealth {
	if v == nil {
		return []models.DiskHealth{}
	}
	return v
}
func nonNilNetworkFlows(v []models.NetworkFlowMetric) []models.NetworkFlowMetric {
	if v == nil {
		return []models.NetworkFlowMetric{}
	}
	return v
}
func nonNilCommands(v []models.RemoteCommand) []models.RemoteCommand {
	if v == nil {
		return []models.RemoteCommand{}
	}
	return v
}

package proxmox

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// Repository is the data-access port. *database.DB satisfies it structurally
// (GetEnabledProxmoxConnections returns the database-package ProxmoxConnectionFull,
// which is why the database type appears here).
type Repository interface {
	ListProxmoxConnections(ctx context.Context) ([]models.ProxmoxConnection, error)
	CreateProxmoxConnection(ctx context.Context, name, apiURL, tokenID, tokenSecret string, insecureSkipVerify, enabled bool, pollIntervalSec int) (string, error)
	GetProxmoxConnectionByID(ctx context.Context, id string) (*models.ProxmoxConnection, error)
	UpdateProxmoxConnection(ctx context.Context, id, name, apiURL, tokenID, tokenSecret string, insecureSkipVerify, enabled bool, pollIntervalSec int) error
	DeleteProxmoxConnection(ctx context.Context, id string) error
	GetEnabledProxmoxConnections(ctx context.Context) ([]database.ProxmoxConnectionFull, error)
	GetProxmoxTokenSecret(ctx context.Context, id string) (string, error)
	GetProxmoxSummary(ctx context.Context) (models.ProxmoxSummary, error)

	ListProxmoxGuests(ctx context.Context, connectionID, guestType, status string) ([]models.ProxmoxGuest, error)
	ListProxmoxGuestsByNode(ctx context.Context, connectionID, nodeName string) ([]models.ProxmoxGuest, error)
	GetProxmoxGuestMetricsSummary(ctx context.Context, guestID string, hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error)

	ListProxmoxGuestLinks(ctx context.Context, status string) ([]models.ProxmoxGuestLink, error)
	UpsertProxmoxGuestLink(ctx context.Context, guestID, hostID, status, metricsSource string) (*models.ProxmoxGuestLink, error)
	GetProxmoxGuestLink(ctx context.Context, id string) (*models.ProxmoxGuestLink, error)
	UpdateProxmoxGuestLink(ctx context.Context, id string, status, metricsSource *string) (*models.ProxmoxGuestLink, error)
	DeleteProxmoxGuestLink(ctx context.Context, id string) error
	GetProxmoxGuestLinkByGuest(ctx context.Context, guestID string) (*models.ProxmoxGuestLink, error)
	GetProxmoxGuestLinkByHost(ctx context.Context, hostID string) (*models.ProxmoxGuestLink, error)
	ListProxmoxLinkCandidates(ctx context.Context, hostID string) ([]models.ProxmoxGuest, error)

	ListProxmoxNodes(ctx context.Context) ([]models.ProxmoxNode, error)
	ListProxmoxNodesByConnection(ctx context.Context, connectionID string) ([]models.ProxmoxNode, error)
	GetProxmoxNode(ctx context.Context, id string) (*models.ProxmoxNode, error)
	GetProxmoxNodeMetricsSummary(ctx context.Context, hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error)
	GetProxmoxNodeCPUTemperatureHistory(ctx context.Context, nodeID string, hours int) ([]models.SystemMetrics, error)
	GetProxmoxNodeFanRPMHistory(ctx context.Context, nodeID string, hours int) ([]models.SystemMetrics, error)
	ListProxmoxNodeCPUTempSourceCandidates(ctx context.Context, connectionID, nodeName string) ([]models.Host, error)
	SetProxmoxNodeSensorSource(ctx context.Context, nodeID, hostID string) error
	BackfillProxmoxNodeSensorSources(ctx context.Context) error
	GetHost(ctx context.Context, id string) (*models.Host, error)

	ListProxmoxDisksByNode(ctx context.Context, connectionID, nodeName string) ([]models.ProxmoxDisk, error)
	ListProxmoxDisksByHost(ctx context.Context, hostID string) ([]models.ProxmoxDisk, error)

	ListProxmoxTasks(ctx context.Context, connectionID string, limit int) ([]models.ProxmoxTask, error)
	ListProxmoxTasksByNode(ctx context.Context, connectionID, nodeName string, limit int) ([]models.ProxmoxTask, error)
	ListProxmoxBackupJobs(ctx context.Context, connectionID string) ([]models.ProxmoxBackupJob, error)
	ListProxmoxBackupRuns(ctx context.Context, connectionID string) ([]models.ProxmoxBackupRun, error)
}

// Service holds the Proxmox HTTP use-cases + owns the background poller.
type Service struct {
	repo   Repository
	cfg    *config.Config
	poller *Poller
	bus    *events.Bus
}

func NewService(db *database.DB, cfg *config.Config, bus *events.Bus) *Service {
	return &Service{repo: db, cfg: cfg, poller: NewPoller(db, cfg), bus: bus}
}

// ===== background poll =====

// PollAll refreshes all connections then wakes the dashboard subscribers (the
// dashboard renders Proxmox nodes/links). Nil-safe when no bus is wired.
func (s *Service) PollAll(ctx context.Context) {
	s.poller.PollAll(ctx)
	s.bus.Publish(events.TopicDashboard)
}

// TriggerPollByID launches an immediate poll of one enabled connection on the
// supplied (long-lived) ctx. Returns apperr.NotFound when the id is not enabled.
func (s *Service) TriggerPollByID(reqCtx, pollCtx context.Context, id string) error {
	conns, err := s.repo.GetEnabledProxmoxConnections(reqCtx)
	if err != nil {
		return err
	}
	for _, conn := range conns {
		if conn.ID == id {
			go s.poller.PollOne(pollCtx, conn)
			return nil
		}
	}
	return apperr.NotFound("enabled connection not found")
}

// ===== connections CRUD =====

func (s *Service) ListConnections(ctx context.Context) ([]models.ProxmoxConnection, error) {
	return s.repo.ListProxmoxConnections(ctx)
}

func (s *Service) CreateConnection(ctx context.Context, req models.ProxmoxConnectionRequest) (*models.ProxmoxConnection, error) {
	if req.TokenSecret == "" {
		return nil, apperr.Validation("token_secret is required when creating a connection")
	}
	id, err := s.repo.CreateProxmoxConnection(ctx, req.Name, req.APIURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify, req.Enabled, req.PollIntervalSec)
	if err != nil {
		return nil, err
	}
	conn, _ := s.repo.GetProxmoxConnectionByID(ctx, id)
	return conn, nil
}

func (s *Service) GetConnection(ctx context.Context, id string) (*models.ProxmoxConnection, error) {
	conn, err := s.repo.GetProxmoxConnectionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, apperr.NotFound("connection not found")
	}
	return conn, nil
}

func (s *Service) UpdateConnection(ctx context.Context, id string, req models.ProxmoxConnectionRequest) (*models.ProxmoxConnection, error) {
	if _, err := s.GetConnection(ctx, id); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateProxmoxConnection(ctx, id, req.Name, req.APIURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify, req.Enabled, req.PollIntervalSec); err != nil {
		return nil, err
	}
	conn, _ := s.repo.GetProxmoxConnectionByID(ctx, id)
	return conn, nil
}

func (s *Service) DeleteConnection(ctx context.Context, id string) error {
	if _, err := s.GetConnection(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteProxmoxConnection(ctx, id)
}

// TestConnection tests ad-hoc credentials (no persistence). The bool/string pair
// mirrors the original 200 {success,error} response.
func (s *Service) TestConnection(apiURL, tokenID, secret string, insecure bool) (bool, string) {
	if err := proxmoxclient.New(apiURL, tokenID, secret, insecure).TestConnection(); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// TestConnectionByID tests a stored connection using its stored secret.
func (s *Service) TestConnectionByID(ctx context.Context, id string) (bool, string, error) {
	conn, err := s.repo.GetProxmoxConnectionByID(ctx, id)
	if err != nil || conn == nil {
		return false, "", apperr.NotFound("connection not found")
	}
	secret, err := s.repo.GetProxmoxTokenSecret(ctx, id)
	if err != nil {
		return false, "", err
	}
	if err := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify).TestConnection(); err != nil {
		return false, err.Error(), nil
	}
	return true, "", nil
}

// ===== summary / guests =====

func (s *Service) Summary(ctx context.Context) (models.ProxmoxSummary, error) {
	return s.repo.GetProxmoxSummary(ctx)
}

func (s *Service) ListGuests(ctx context.Context, connectionID, guestType, status string) ([]models.ProxmoxGuest, error) {
	return s.repo.ListProxmoxGuests(ctx, connectionID, guestType, status)
}

func (s *Service) GuestMetricsSummary(ctx context.Context, guestID string, hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error) {
	summary, err := s.repo.GetProxmoxGuestMetricsSummary(ctx, guestID, hours, bucketMinutes)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = []models.ProxmoxNodeMetricsSummary{}
	}
	return summary, nil
}

// ===== guest ↔ host links =====

func (s *Service) ListLinks(ctx context.Context, status string) ([]models.ProxmoxGuestLink, error) {
	return s.repo.ListProxmoxGuestLinks(ctx, status)
}

func (s *Service) CreateLink(ctx context.Context, req models.ProxmoxGuestLinkRequest) (*models.ProxmoxGuestLink, error) {
	if req.Status == "" {
		req.Status = "confirmed"
	}
	if req.MetricsSource == "" {
		req.MetricsSource = "auto"
	}
	link, err := s.repo.UpsertProxmoxGuestLink(ctx, req.GuestID, req.HostID, req.Status, req.MetricsSource)
	if err == nil {
		s.publishLink(req.HostID)
	}
	return link, err
}

// publishLink wakes the dashboard + the affected host-detail subscribers after a
// guest↔host link change (both views render the link). Nil-safe.
func (s *Service) publishLink(hostID string) {
	s.bus.Publish(events.TopicDashboard)
	if hostID != "" {
		s.bus.Publish(events.HostTopic(hostID))
	}
}

func (s *Service) GetLink(ctx context.Context, id string) (*models.ProxmoxGuestLink, error) {
	link, err := s.repo.GetProxmoxGuestLink(ctx, id)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, apperr.NotFound("link not found")
	}
	return link, nil
}

func (s *Service) UpdateLink(ctx context.Context, id string, req models.ProxmoxGuestLinkUpdate) (*models.ProxmoxGuestLink, error) {
	if _, err := s.GetLink(ctx, id); err != nil {
		return nil, err
	}
	validStatuses := map[string]bool{"suggested": true, "confirmed": true, "ignored": true}
	validMetricsSources := map[string]bool{"auto": true, "agent": true, "proxmox": true}
	if req.Status != nil && !validStatuses[*req.Status] {
		return nil, apperr.Validation("status invalide : doit être suggested, confirmed ou ignored")
	}
	if req.MetricsSource != nil && !validMetricsSources[*req.MetricsSource] {
		return nil, apperr.Validation("metrics_source invalide : doit être auto, agent ou proxmox")
	}
	link, err := s.repo.UpdateProxmoxGuestLink(ctx, id, req.Status, req.MetricsSource)
	if err == nil && link != nil {
		s.publishLink(link.HostID)
	}
	return link, err
}

func (s *Service) DeleteLink(ctx context.Context, id string) error {
	link, err := s.GetLink(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteProxmoxGuestLink(ctx, id); err != nil {
		return err
	}
	s.publishLink(link.HostID)
	return nil
}

func (s *Service) LinkByGuest(ctx context.Context, guestID string) (*models.ProxmoxGuestLink, error) {
	return s.repo.GetProxmoxGuestLinkByGuest(ctx, guestID)
}

func (s *Service) LinkByHost(ctx context.Context, hostID string) (*models.ProxmoxGuestLink, error) {
	return s.repo.GetProxmoxGuestLinkByHost(ctx, hostID)
}

func (s *Service) LinkCandidates(ctx context.Context, hostID string) ([]models.ProxmoxGuest, error) {
	return s.repo.ListProxmoxLinkCandidates(ctx, hostID)
}

func (s *Service) HostProxmoxDisks(ctx context.Context, hostID string) ([]models.ProxmoxDisk, error) {
	return s.repo.ListProxmoxDisksByHost(ctx, hostID)
}

// ===== nodes (stored read models) =====

func (s *Service) ListNodes(ctx context.Context, connectionID string) ([]models.ProxmoxNode, error) {
	_ = s.repo.BackfillProxmoxNodeSensorSources(ctx)
	if connectionID != "" {
		return s.repo.ListProxmoxNodesByConnection(ctx, connectionID)
	}
	return s.repo.ListProxmoxNodes(ctx)
}

func (s *Service) GetNode(ctx context.Context, id string) (*models.ProxmoxNode, error) {
	_ = s.repo.BackfillProxmoxNodeSensorSources(ctx)
	return s.node(ctx, id)
}

// node fetches a node or returns apperr.NotFound (shared by the live-proxy paths).
func (s *Service) node(ctx context.Context, id string) (*models.ProxmoxNode, error) {
	node, err := s.repo.GetProxmoxNode(ctx, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperr.NotFound("node not found")
	}
	return node, nil
}

func (s *Service) NodeMetricsSummary(ctx context.Context, hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error) {
	summary, err := s.repo.GetProxmoxNodeMetricsSummary(ctx, hours, bucketMinutes)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = []models.ProxmoxNodeMetricsSummary{}
	}
	return summary, nil
}

func (s *Service) NodeCPUTemperatureHistory(ctx context.Context, nodeID string, hours int) ([]models.SystemMetrics, error) {
	_ = s.repo.BackfillProxmoxNodeSensorSources(ctx)
	h, err := s.repo.GetProxmoxNodeCPUTemperatureHistory(ctx, nodeID, hours)
	if err != nil {
		return nil, err
	}
	return nonNilMetrics(h), nil
}

func (s *Service) NodeFanRPMHistory(ctx context.Context, nodeID string, hours int) ([]models.SystemMetrics, error) {
	_ = s.repo.BackfillProxmoxNodeSensorSources(ctx)
	h, err := s.repo.GetProxmoxNodeFanRPMHistory(ctx, nodeID, hours)
	if err != nil {
		return nil, err
	}
	return nonNilMetrics(h), nil
}

func (s *Service) NodeSensorSourceCandidates(ctx context.Context, nodeID string) ([]models.Host, error) {
	node, err := s.node(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	hosts, err := s.repo.ListProxmoxNodeCPUTempSourceCandidates(ctx, node.ConnectionID, node.NodeName)
	if err != nil {
		return nil, err
	}
	if hosts == nil {
		hosts = []models.Host{}
	}
	return hosts, nil
}

func (s *Service) UpdateNodeSensorSource(ctx context.Context, nodeID, hostID string) (*models.ProxmoxNode, error) {
	if _, err := s.node(ctx, nodeID); err != nil {
		return nil, err
	}
	if hostID != "" {
		host, err := s.repo.GetHost(ctx, hostID)
		if err != nil || host == nil {
			return nil, apperr.Validation("invalid host_id")
		}
	}
	if err := s.repo.SetProxmoxNodeSensorSource(ctx, nodeID, hostID); err != nil {
		return nil, err
	}
	return s.repo.GetProxmoxNode(ctx, nodeID)
}

func (s *Service) NodeDisks(ctx context.Context, nodeID string) ([]models.ProxmoxDisk, error) {
	node, err := s.node(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListProxmoxDisksByNode(ctx, node.ConnectionID, node.NodeName)
}

// ===== tasks / backups =====

func (s *Service) ListTasks(ctx context.Context, connectionID string, limit int) ([]models.ProxmoxTask, error) {
	return s.repo.ListProxmoxTasks(ctx, connectionID, limit)
}

func (s *Service) ListNodeTasks(ctx context.Context, nodeID string, limit int) ([]models.ProxmoxTask, error) {
	node, err := s.node(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListProxmoxTasksByNode(ctx, node.ConnectionID, node.NodeName, limit)
}

func (s *Service) ListBackupJobs(ctx context.Context, connectionID string) ([]models.ProxmoxBackupJob, error) {
	return s.repo.ListProxmoxBackupJobs(ctx, connectionID)
}

func (s *Service) ListBackupRuns(ctx context.Context, connectionID string) ([]models.ProxmoxBackupRun, error) {
	return s.repo.ListProxmoxBackupRuns(ctx, connectionID)
}

// ===== live PVE proxy =====

// resolveSecret returns the token secret + connection for a connection id.
func (s *Service) resolveSecret(ctx context.Context, connectionID string) (string, *models.ProxmoxConnection, error) {
	conns, err := s.repo.GetEnabledProxmoxConnections(ctx)
	if err != nil {
		return "", nil, err
	}
	var secret string
	for _, co := range conns {
		if co.ID == connectionID {
			secret = co.TokenSecret
			break
		}
	}
	if secret == "" {
		return "", nil, apperr.BadGateway("connection not found or disabled")
	}
	conn, err := s.repo.GetProxmoxConnectionByID(ctx, connectionID)
	if err != nil || conn == nil {
		return "", nil, apperr.BadGateway("failed to load connection")
	}
	return secret, conn, nil
}

// nodeClient resolves a node id to (node, PVE client) or an apperr.
func (s *Service) nodeClient(ctx context.Context, nodeID string) (*models.ProxmoxNode, *proxmoxclient.Client, error) {
	node, err := s.node(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	secret, conn, err := s.resolveSecret(ctx, node.ConnectionID)
	if err != nil {
		return nil, nil, err
	}
	return node, proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify), nil
}

func (s *Service) NodeStatus(ctx context.Context, nodeID string) (any, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	status, err := client.GetNodeStatus(node.NodeName)
	if err != nil {
		return nil, apperr.BadGateway(err.Error())
	}
	return status, nil
}

func (s *Service) NodeRRD(ctx context.Context, nodeID, timeframe string) (any, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	points, err := client.GetNodeRRDData(node.NodeName, timeframe)
	if err != nil {
		return nil, apperr.BadGateway(err.Error())
	}
	return points, nil
}

func (s *Service) NodeServices(ctx context.Context, nodeID string) (any, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	services, err := client.GetNodeServices(node.NodeName)
	if err != nil {
		return nil, apperr.BadGateway(err.Error())
	}
	return services, nil
}

func (s *Service) TaskLog(ctx context.Context, nodeID, upid string) (any, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	lines, err := client.GetNodeTaskLog(node.NodeName, upid)
	if err != nil {
		return nil, apperr.BadGateway(err.Error())
	}
	return lines, nil
}

func (s *Service) NodeSyslog(ctx context.Context, nodeID string, limit int, service, search string) ([]proxmoxclient.PVESyslogLine, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	lines, err := client.GetNodeSyslog(node.NodeName, limit, service)
	if err != nil {
		return nil, apperr.BadGateway(err.Error())
	}
	if search != "" {
		needle := strings.ToLower(search)
		filtered := make([]proxmoxclient.PVESyslogLine, 0, len(lines))
		for _, line := range lines {
			haystack := strings.ToLower(strings.Join([]string{line.T, line.Msg, line.Tag, line.Level, line.Node, line.PID}, " "))
			if strings.Contains(haystack, needle) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}
	return lines, nil
}

func (s *Service) RefreshNodeApt(ctx context.Context, nodeID string) (string, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return "", err
	}
	upid, err := client.TriggerNodeAptUpdate(node.NodeName)
	if err != nil {
		return "", apperr.BadGateway(err.Error())
	}
	return upid, nil
}

func (s *Service) NodeGuestNetworks(ctx context.Context, nodeID string) (map[int][]proxmoxclient.GuestNetworkIface, error) {
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	guests, err := s.repo.ListProxmoxGuestsByNode(ctx, node.ConnectionID, node.NodeName)
	if err != nil {
		return nil, err
	}
	result := make(map[int][]proxmoxclient.GuestNetworkIface)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, g := range guests {
		if g.Status != "running" {
			continue
		}
		wg.Add(1)
		go func(guest models.ProxmoxGuest) {
			defer wg.Done()
			var ifaces []proxmoxclient.GuestNetworkIface
			var ferr error
			if guest.GuestType == "vm" {
				ifaces, ferr = client.GetVMNetworkInterfaces(node.NodeName, guest.VMID)
			} else {
				ifaces, ferr = client.GetLXCInterfaces(node.NodeName, guest.VMID)
			}
			if ferr != nil || len(ifaces) == 0 {
				return
			}
			mu.Lock()
			result[guest.VMID] = ifaces
			mu.Unlock()
		}(g)
	}
	wg.Wait()
	return result, nil
}

// MigrateGuest migrates a guest to target; guestType defaults to "vm".
func (s *Service) MigrateGuest(ctx context.Context, nodeID string, vmid int, guestType, target string, online bool) (string, error) {
	if guestType == "" {
		guestType = "vm"
	}
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return "", err
	}
	upid, err := client.MigrateGuest(node.NodeName, vmid, guestType, target, online)
	if err != nil {
		return "", apperr.BadGateway(err.Error())
	}
	return upid, nil
}

var validServiceAction = map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}

func (s *Service) NodeServiceAction(ctx context.Context, nodeID, service, action string) (string, error) {
	if !validServiceAction[action] {
		return "", apperr.Validation(fmt.Sprintf("invalid action %q; allowed: start stop restart reload", action))
	}
	node, client, err := s.nodeClient(ctx, nodeID)
	if err != nil {
		return "", err
	}
	upid, err := client.NodeServiceAction(node.NodeName, service, action)
	if err != nil {
		return "", apperr.BadGateway(err.Error())
	}
	return upid, nil
}

func nonNilMetrics(v []models.SystemMetrics) []models.SystemMetrics {
	if v == nil {
		return []models.SystemMetrics{}
	}
	return v
}

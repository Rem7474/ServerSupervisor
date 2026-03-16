package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// taskLimit is the number of recent tasks fetched per node per poll cycle.
const taskLimit = 50

// ProxmoxHandler manages Proxmox connections, exposes read-only data,
// and runs the background polling loop.
type ProxmoxHandler struct {
	db   *database.DB
	cfg  *config.Config
	stop chan struct{}
}

func NewProxmoxHandler(db *database.DB, cfg *config.Config) *ProxmoxHandler {
	return &ProxmoxHandler{
		db:   db,
		cfg:  cfg,
		stop: make(chan struct{}),
	}
}

// ─── Poller ───────────────────────────────────────────────────────────────────

// StartPoller begins periodic collection for all enabled Proxmox connections.
// It runs an immediate first pass, then repeats at the minimum configured interval.
func (h *ProxmoxHandler) StartPoller() {
	go h.pollAll() // immediate first pass

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				h.pollAll()
			case <-h.stop:
				ticker.Stop()
				return
			}
		}
	}()
	log.Println("Proxmox poller started (tick: 30s, respects per-connection poll_interval_sec)")
}

func (h *ProxmoxHandler) StopPoller() {
	close(h.stop)
}

// pollAll iterates all enabled connections and polls each one.
func (h *ProxmoxHandler) pollAll() {
	conns, err := h.db.GetEnabledProxmoxConnections()
	if err != nil {
		log.Printf("proxmox poller: failed to fetch connections: %v", err)
		return
	}
	for _, c := range conns {
		h.pollOne(c)
	}
}

func (h *ProxmoxHandler) pollOne(conn database.ProxmoxConnectionFull) {
	// Respect per-connection interval: skip if last success was recent enough.
	interval := time.Duration(conn.PollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if conn.LastSuccessAt != nil && time.Since(*conn.LastSuccessAt) < interval {
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, conn.TokenSecret, conn.InsecureSkipVerify)

	// Fetch cluster name (best-effort — standalone nodes may not have this endpoint).
	clusterStatuses, _ := client.GetClusterStatus()
	clusterName := proxmoxclient.ClusterName(clusterStatuses)

	// Fetch nodes.
	nodes, err := client.GetNodes()
	if err != nil {
		log.Printf("proxmox poller [%s]: failed to get nodes: %v", conn.Name, err)
		_ = h.db.UpdateProxmoxConnectionError(conn.ID, err.Error())
		return
	}

	cutoff := time.Now().Add(-3 * interval) // mark stale after 3 missed polls

	for _, n := range nodes {
		// Fetch version for online nodes.
		pveVersion := n.PVEVersion
		if n.Status == "online" && pveVersion == "" {
			if v, err := client.GetNodeVersion(n.Node); err == nil {
				pveVersion = v
			}
		}

		if err := h.db.UpsertProxmoxNode(
			conn.ID, n.Node, n.Status, n.MaxCPU, n.CPU,
			n.MaxMem, n.Mem, n.Uptime, pveVersion, clusterName, n.IP,
		); err != nil {
			log.Printf("proxmox poller [%s]: upsert node %s: %v", conn.Name, n.Node, err)
		}

		if n.Status != "online" {
			continue
		}

		// VMs
		vms, err := client.GetNodeQemu(n.Node)
		if err != nil {
			log.Printf("proxmox poller [%s/%s]: get qemu: %v", conn.Name, n.Node, err)
		} else {
			for _, vm := range vms {
				if err := h.db.UpsertProxmoxGuest(
					conn.ID, n.Node, "vm", vm.VMID, vm.Name, vm.Status,
					vm.CPUs, vm.CPU, vm.MaxMem, vm.Mem, vm.MaxDisk, vm.Uptime, vm.Tags,
				); err != nil {
					log.Printf("proxmox poller [%s/%s]: upsert vm %d: %v", conn.Name, n.Node, vm.VMID, err)
					continue
				}
				// Auto-suggest a host link by name matching (best-effort).
				if guestID, err := h.db.GetProxmoxGuestIDByVMID(conn.ID, n.Node, vm.VMID); err == nil && guestID != "" {
					_ = h.db.AutoSuggestProxmoxLink(guestID, vm.Name)
				}
			}
		}

		// LXC containers
		lxcs, err := client.GetNodeLXC(n.Node)
		if err != nil {
			log.Printf("proxmox poller [%s/%s]: get lxc: %v", conn.Name, n.Node, err)
		} else {
			for _, lxc := range lxcs {
				if err := h.db.UpsertProxmoxGuest(
					conn.ID, n.Node, "lxc", lxc.VMID, lxc.Name, lxc.Status,
					lxc.CPUs, lxc.CPU, lxc.MaxMem, lxc.Mem, lxc.MaxDisk, lxc.Uptime, lxc.Tags,
				); err != nil {
					log.Printf("proxmox poller [%s/%s]: upsert lxc %d: %v", conn.Name, n.Node, lxc.VMID, err)
					continue
				}
				// Auto-suggest a host link by name matching (best-effort).
				if guestID, err := h.db.GetProxmoxGuestIDByVMID(conn.ID, n.Node, lxc.VMID); err == nil && guestID != "" {
					_ = h.db.AutoSuggestProxmoxLink(guestID, lxc.Name)
				}
			}
		}

		// Storage
		storages, err := client.GetNodeStorage(n.Node)
		if err != nil {
			log.Printf("proxmox poller [%s/%s]: get storage: %v", conn.Name, n.Node, err)
		} else {
			for _, s := range storages {
				if err := h.db.UpsertProxmoxStorage(
					conn.ID, n.Node, s.Storage, s.Type,
					s.Total, s.Used, s.Avail,
					s.Enabled != 0, s.Active != 0, s.Shared != 0,
				); err != nil {
					log.Printf("proxmox poller [%s/%s]: upsert storage %s: %v", conn.Name, n.Node, s.Storage, err)
				}
			}
		}

		// Tasks (recent history)
		tasks, err := client.GetNodeTasks(n.Node, taskLimit)
		if err != nil {
			log.Printf("proxmox poller [%s/%s]: get tasks: %v", conn.Name, n.Node, err)
		} else {
			for _, t := range tasks {
				var startTime, endTime *time.Time
				if t.StartTime > 0 {
					v := time.Unix(t.StartTime, 0).UTC()
					startTime = &v
				}
				if t.EndTime > 0 {
					v := time.Unix(t.EndTime, 0).UTC()
					endTime = &v
				}
				if err := h.db.UpsertProxmoxTask(
					conn.ID, n.Node, t.UPID, t.Type, t.Status, t.User,
					startTime, endTime, t.ExitStatus, t.ID,
				); err != nil {
					log.Printf("proxmox poller [%s/%s]: upsert task %s: %v", conn.Name, n.Node, t.UPID, err)
				}
				// Track latest vzdump run per VM for backup overview.
				if t.Type == "vzdump" && t.Status == "stopped" && t.ID != "" {
					if vmid := parseVMID(t.ID); vmid > 0 {
						_ = h.db.UpsertProxmoxBackupRun(
							conn.ID, n.Node, vmid, t.UPID, t.ExitStatus,
							startTime, endTime, t.ExitStatus,
						)
					}
				}
			}
		}

		// Physical disks
		disks, diskErr := client.GetNodeDisksList(n.Node)
		if diskErr != nil {
			log.Printf("proxmox poller [%s/%s]: get disks FAILED (check Sys.Audit privilege on API token): %v", conn.Name, n.Node, diskErr)
		} else {
			log.Printf("proxmox poller [%s/%s]: got %d disk(s)", conn.Name, n.Node, len(disks))
			for _, d := range disks {
				health := d.Health
				if health == "" {
					health = "UNKNOWN"
				}
				wearout := d.Wearout
				if d.Type != "ssd" && d.Type != "nvme" {
					wearout = -1
				}
				if err := h.db.UpsertProxmoxDisk(
					conn.ID, n.Node, d.DevPath, d.Model, d.Serial,
					d.Size, d.Type, health, wearout,
				); err != nil {
					log.Printf("proxmox poller [%s/%s]: upsert disk %s: %v", conn.Name, n.Node, d.DevPath, err)
				}
			}
		}

		// Pending apt updates (graceful — may be denied by some PVE configurations)
		pkgs, aptErr := client.GetNodeAptUpdate(n.Node)
		if aptErr != nil {
			log.Printf("proxmox poller [%s/%s]: get apt/update FAILED (check Sys.Modify or PVEAuditor permissions): %v", conn.Name, n.Node, aptErr)
		} else {
			log.Printf("proxmox poller [%s/%s]: got %d pending apt package(s)", conn.Name, n.Node, len(pkgs))
		}
		pending, security := 0, 0
		for _, p := range pkgs {
			pending++
			if isSecurityPackage(p.Origin, p.Section) {
				security++
			}
		}
		if err := h.db.UpdateProxmoxNodeUpdates(conn.ID, n.Node, pending, security); err != nil {
			log.Printf("proxmox poller [%s/%s]: update node updates: %v", conn.Name, n.Node, err)
		}
	}

	// Backup job configurations (once per connection)
	backupJobs, err := client.GetClusterBackup()
	if err != nil {
		log.Printf("proxmox poller [%s]: get backup jobs: %v", conn.Name, err)
	} else {
		for _, j := range backupJobs {
			if err := h.db.UpsertProxmoxBackupJob(
				conn.ID, j.ID, j.Enabled != 0,
				j.Schedule, j.Storage, j.Mode, j.Compress, j.VMIDs, j.MailTo,
			); err != nil {
				log.Printf("proxmox poller [%s]: upsert backup job %s: %v", conn.Name, j.ID, err)
			}
		}
		_ = h.db.DeleteStaleProxmoxBackupJobs(conn.ID, cutoff)
	}

	// Cleanup stale records.
	_ = h.db.DeleteStaleProxmoxGuests(conn.ID, cutoff)
	_ = h.db.DeleteStaleProxmoxNodes(conn.ID, cutoff)
	_ = h.db.DeleteStaleProxmoxTasks(conn.ID, cutoff)
	_ = h.db.DeleteStaleProxmoxDisks(conn.ID, cutoff)

	_ = h.db.UpdateProxmoxConnectionSuccess(conn.ID)
	log.Printf("proxmox poller [%s]: poll complete (%d node(s))", conn.Name, len(nodes))
}

// ─── CRUD: Connections ────────────────────────────────────────────────────────

// ListConnections returns all connections (no secrets).
func (h *ProxmoxHandler) ListConnections(c *gin.Context) {
	conns, err := h.db.ListProxmoxConnections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conns)
}

// CreateConnection adds a new Proxmox connection.
func (h *ProxmoxHandler) CreateConnection(c *gin.Context) {
	var req models.ProxmoxConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TokenSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_secret is required when creating a connection"})
		return
	}

	id, err := h.db.CreateProxmoxConnection(
		req.Name, req.APIURL, req.TokenID, req.TokenSecret,
		req.InsecureSkipVerify, req.Enabled, req.PollIntervalSec,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	conn, _ := h.db.GetProxmoxConnectionByID(id)
	c.JSON(http.StatusCreated, conn)
}

// GetConnection returns one connection (no secret).
func (h *ProxmoxHandler) GetConnection(c *gin.Context) {
	conn, err := h.db.GetProxmoxConnectionByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	c.JSON(http.StatusOK, conn)
}

// UpdateConnection updates a connection. Empty token_secret keeps the existing one.
func (h *ProxmoxHandler) UpdateConnection(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxConnectionByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}

	var req models.ProxmoxConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateProxmoxConnection(
		id, req.Name, req.APIURL, req.TokenID, req.TokenSecret,
		req.InsecureSkipVerify, req.Enabled, req.PollIntervalSec,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	conn, _ := h.db.GetProxmoxConnectionByID(id)
	c.JSON(http.StatusOK, conn)
}

// DeleteConnection removes a connection (and cascade-deletes its nodes/guests/storages).
func (h *ProxmoxHandler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxConnectionByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	if err := h.db.DeleteProxmoxConnection(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "connection deleted"})
}

// TestConnection tests connectivity and token validity without saving anything.
func (h *ProxmoxHandler) TestConnection(c *gin.Context) {
	var req struct {
		APIURL             string `json:"api_url" binding:"required"`
		TokenID            string `json:"token_id" binding:"required"`
		TokenSecret        string `json:"token_secret" binding:"required"`
		InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(req.APIURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify)
	if err := client.TestConnection(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestConnectionByID tests an existing saved connection (uses stored secret).
func (h *ProxmoxHandler) TestConnectionByID(c *gin.Context) {
	id := c.Param("id")
	conn, err := h.db.GetProxmoxConnectionByID(id)
	if err != nil || conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	secret, err := h.db.GetProxmoxTokenSecret(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	if err := client.TestConnection(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PollNow triggers an immediate poll for one connection.
func (h *ProxmoxHandler) PollNow(c *gin.Context) {
	id := c.Param("id")
	conns, err := h.db.GetEnabledProxmoxConnections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, conn := range conns {
		if conn.ID == id {
			go h.pollOne(conn)
			c.JSON(http.StatusOK, gin.H{"message": "poll triggered"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "enabled connection not found"})
}

// ─── Read-only data endpoints ─────────────────────────────────────────────────

// GetSummary returns aggregate stats (connection/node/guest/storage counts).
func (h *ProxmoxHandler) GetSummary(c *gin.Context) {
	summary, err := h.db.GetProxmoxSummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ListNodes returns all nodes, optionally filtered by connection_id query param.
func (h *ProxmoxHandler) ListNodes(c *gin.Context) {
	connID := c.Query("connection_id")
	var (
		nodes []models.ProxmoxNode
		err   error
	)
	if connID != "" {
		nodes, err = h.db.ListProxmoxNodesByConnection(connID)
	} else {
		nodes, err = h.db.ListProxmoxNodes()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// GetNode returns a single node with its guests and storages.
func (h *ProxmoxHandler) GetNode(c *gin.Context) {
	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	c.JSON(http.StatusOK, node)
}

// ListGuests returns all guests with optional filters: connection_id, type (vm|lxc), status.
func (h *ProxmoxHandler) ListGuests(c *gin.Context) {
	guests, err := h.db.ListProxmoxGuests(
		c.Query("connection_id"),
		c.Query("type"),
		c.Query("status"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, guests)
}

// ─── Guest ↔ Host links ───────────────────────────────────────────────────────

// ListLinks returns all guest-host links, optionally filtered by ?status=suggested|confirmed|ignored.
func (h *ProxmoxHandler) ListLinks(c *gin.Context) {
	links, err := h.db.ListProxmoxGuestLinks(c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, links)
}

// CreateLink creates or replaces a guest-host link (upserts on guest_id).
func (h *ProxmoxHandler) CreateLink(c *gin.Context) {
	var req models.ProxmoxGuestLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Status == "" {
		req.Status = "confirmed"
	}
	if req.MetricsSource == "" {
		req.MetricsSource = "auto"
	}
	link, err := h.db.UpsertProxmoxGuestLink(req.GuestID, req.HostID, req.Status, req.MetricsSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, link)
}

// GetLink returns a single link by its ID.
func (h *ProxmoxHandler) GetLink(c *gin.Context) {
	link, err := h.db.GetProxmoxGuestLink(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if link == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	c.JSON(http.StatusOK, link)
}

// UpdateLink updates status and/or metrics_source for a link.
func (h *ProxmoxHandler) UpdateLink(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxGuestLink(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}

	var req models.ProxmoxGuestLinkUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	link, err := h.db.UpdateProxmoxGuestLink(id, req.Status, req.MetricsSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link)
}

// DeleteLink removes a guest-host link.
func (h *ProxmoxHandler) DeleteLink(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxGuestLink(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err := h.db.DeleteProxmoxGuestLink(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link deleted"})
}

// GetLinkByGuest returns the link for a specific Proxmox guest, or null when none exists.
// Returns 200 in both cases to avoid spurious 404s in the browser console.
func (h *ProxmoxHandler) GetLinkByGuest(c *gin.Context) {
	link, err := h.db.GetProxmoxGuestLinkByGuest(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link) // nil marshals to JSON null
}

// GetLinkByHost returns the confirmed/suggested Proxmox link for a host, or null when none exists.
// Returns 200 in both cases to avoid spurious 404s in the browser console.
func (h *ProxmoxHandler) GetLinkByHost(c *gin.Context) {
	link, err := h.db.GetProxmoxGuestLinkByHost(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link) // nil marshals to JSON null
}

// ListLinkCandidates returns Proxmox guests that could be linked to a host,
// ordered by name similarity. Used for the manual-link dropdown.
func (h *ProxmoxHandler) ListLinkCandidates(c *gin.Context) {
	hostID := c.Param("id")
	candidates, err := h.db.ListProxmoxLinkCandidates(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, candidates)
}

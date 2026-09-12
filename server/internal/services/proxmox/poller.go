// Package proxmox is the application/service layer for Proxmox VE supervision:
// connection CRUD, the stored read models, the live PVE proxy endpoints and the
// background polling loop. The HTTP use-cases sit behind a Repository port; the
// poller is background sync and uses the concrete *database.DB (like the other
// background jobs).
package proxmox

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/proxmoxclient"
	"github.com/serversupervisor/server/internal/safego"
)

// taskLimit is the number of recent tasks fetched per node per poll cycle.
const taskLimit = 50

// parseVMID converts a Proxmox task object ID string to an integer VMID.
// Returns 0 if the string is not a valid positive integer.
func parseVMID(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// taskFinished reports whether a PVE task has stopped.
//
// The task *list* endpoint does not report the lifecycle the way the per-task
// status endpoint does: it puts the outcome straight into `status` ("OK",
// "job errors", …) and leaves `exitstatus` empty, so testing `status ==
// "stopped"` never matches a finished task and every backup result was
// discarded. Treating anything that isn't "running" as finished works against
// both shapes, and an empty status (no data at all) is deliberately not
// finished — better to skip a task than to record a result PVE never gave.
func taskFinished(t proxmoxclient.PVETask) bool {
	if t.Status == "" && t.ExitStatus == "" {
		return false
	}
	return t.Status != "running"
}

// taskOutcome returns a finished task's result, preferring the dedicated
// exitstatus field when the endpoint provides one and falling back to the
// status field that the list endpoint overloads with it.
func taskOutcome(t proxmoxclient.PVETask) string {
	if t.ExitStatus != "" {
		return t.ExitStatus
	}
	if t.Status != "running" {
		return t.Status
	}
	return ""
}

// Backup run statuses. proxmox_backup_runs.status is a small vocabulary the UI
// can colour and translate; the raw PVE outcome is kept in exit_status.
//
// Storing the raw string in both meant a failure rendered as an untranslated,
// unbounded badge ("could not activate storage 'pbs-immich': …") in neutral
// grey, while the same task showed a red "failed" in the log panel.
const (
	BackupStatusOK       = "OK"
	BackupStatusFailed   = "failed"
	BackupStatusWarnings = "warnings"
	BackupStatusRunning  = "running"
)

// normalizeBackupStatus maps a PVE task outcome onto that vocabulary.
//
// vzdump reports "OK", "WARNINGS: n" when guests were skipped but the job
// completed, or an arbitrary error string. Only the last is a failure —
// collapsing warnings into it would report a backup that ran as broken.
func normalizeBackupStatus(outcome string) string {
	switch trimmed := strings.TrimSpace(outcome); {
	case trimmed == "":
		return BackupStatusRunning
	case strings.EqualFold(trimmed, "OK"):
		return BackupStatusOK
	case strings.HasPrefix(strings.ToUpper(trimmed), "WARNINGS"):
		return BackupStatusWarnings
	default:
		return BackupStatusFailed
	}
}

// mergeTasksByUPID appends extra tasks not already present in base (keyed by
// UPID, PVE's unique task identifier) — used to fold a type-filtered task
// fetch (e.g. "vzdump") into the plain top-N task window without duplicating
// a task both calls happened to return.
func mergeTasksByUPID(base, extra []proxmoxclient.PVETask) []proxmoxclient.PVETask {
	seen := make(map[string]bool, len(base))
	for _, t := range base {
		seen[t.UPID] = true
	}
	for _, t := range extra {
		if !seen[t.UPID] {
			base = append(base, t)
			seen[t.UPID] = true
		}
	}
	return base
}

// Poller runs the background Proxmox collection loop. It is background sync, so it
// uses the concrete *database.DB directly (many write paths) rather than a port.
type Poller struct {
	db  *database.DB
	cfg *config.Config
}

func NewPoller(db *database.DB, cfg *config.Config) *Poller {
	return &Poller{db: db, cfg: cfg}
}

// PollAll iterates all enabled connections and polls each one.
func (s *Poller) PollAll(ctx context.Context) {
	conns, err := s.db.GetEnabledProxmoxConnections(ctx)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller: failed to fetch connections: %v", err))
		return
	}
	for _, c := range conns {
		if ctx.Err() != nil {
			return
		}
		s.PollOne(ctx, c)
	}
}

func (s *Poller) PollOne(ctx context.Context, conn database.ProxmoxConnectionFull) {
	interval := time.Duration(conn.PollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if conn.LastSuccessAt != nil && time.Since(*conn.LastSuccessAt) < interval {
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, conn.TokenSecret, conn.InsecureSkipVerify)

	clusterStatuses, _ := client.GetClusterStatus()
	clusterName := proxmoxclient.ClusterName(clusterStatuses)

	nodes, err := client.GetNodes()
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s]: failed to get nodes: %v", conn.Name, err))
		_ = s.db.UpdateProxmoxConnectionError(ctx, conn.ID, err.Error())
		return
	}

	cutoff := time.Now().Add(-3 * interval)
	deg := &degraded{}

	for _, n := range nodes {
		pveVersion := n.PVEVersion
		if n.Status == "online" && pveVersion == "" {
			if v, err := client.GetNodeVersion(n.Node); err == nil {
				pveVersion = v
			}
		}

		if err := s.db.UpsertProxmoxNode(ctx,
			conn.ID, n.Node, n.Status, n.MaxCPU, n.CPU,
			n.MaxMem, n.Mem, n.Uptime, pveVersion, clusterName, n.IP,
		); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s]: upsert node %s: %v", conn.Name, n.Node, err))
		} else if n.Status == "online" {
			if nodeID, err := s.db.GetProxmoxNodeID(ctx, conn.ID, n.Node); err == nil {
				if err := s.db.InsertProxmoxNodeMetric(ctx, nodeID, conn.ID, n.Node, n.CPU, n.MaxMem, n.Mem); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: insert node metric: %v", conn.Name, n.Node, err))
				}
			}
		}

		if n.Status != "online" {
			continue
		}

		vms, err := client.GetNodeQemu(n.Node)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get qemu: %v", conn.Name, n.Node, err))
		} else {
			for _, vm := range vms {
				if err := s.db.UpsertProxmoxGuest(ctx,
					conn.ID, n.Node, "vm", vm.VMID, vm.Name, vm.Status,
					vm.CPUs, vm.CPU, vm.MaxMem, vm.Mem, vm.MaxDisk, vm.Disk, vm.Uptime, vm.Tags,
				); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert vm %d: %v", conn.Name, n.Node, vm.VMID, err))
					continue
				}
				if guestID, err := s.db.GetProxmoxGuestIDByVMID(ctx, conn.ID, n.Node, vm.VMID); err == nil && guestID != "" {
					_ = s.db.AutoSuggestProxmoxLink(ctx, guestID, vm.Name)
					if vm.Status == "running" {
						if err := s.db.InsertProxmoxGuestMetric(ctx, guestID, vm.CPU, vm.MaxMem, vm.Mem); err != nil {
							slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: insert vm metric %d: %v", conn.Name, n.Node, vm.VMID, err))
						}
					}
				}
			}
		}

		lxcs, err := client.GetNodeLXC(n.Node)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get lxc: %v", conn.Name, n.Node, err))
		} else {
			for _, lxc := range lxcs {
				if err := s.db.UpsertProxmoxGuest(ctx,
					conn.ID, n.Node, "lxc", lxc.VMID, lxc.Name, lxc.Status,
					lxc.CPUs, lxc.CPU, lxc.MaxMem, lxc.Mem, lxc.MaxDisk, lxc.Disk, lxc.Uptime, lxc.Tags,
				); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert lxc %d: %v", conn.Name, n.Node, lxc.VMID, err))
					continue
				}
				if guestID, err := s.db.GetProxmoxGuestIDByVMID(ctx, conn.ID, n.Node, lxc.VMID); err == nil && guestID != "" {
					_ = s.db.AutoSuggestProxmoxLink(ctx, guestID, lxc.Name)
					if lxc.Status == "running" {
						if err := s.db.InsertProxmoxGuestMetric(ctx, guestID, lxc.CPU, lxc.MaxMem, lxc.Mem); err != nil {
							slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: insert lxc metric %d: %v", conn.Name, n.Node, lxc.VMID, err))
						}
					}
				}
			}
		}

		storages, err := client.GetNodeStorage(n.Node)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get storage: %v", conn.Name, n.Node, err))
			deg.note("storage", err)
		} else {
			for _, st := range storages {
				if err := s.db.UpsertProxmoxStorage(ctx,
					conn.ID, n.Node, st.Storage, st.Type,
					st.Total, st.Used, st.Avail,
					st.Enabled != 0, st.Active != 0, st.Shared != 0,
				); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert storage %s: %v", conn.Name, n.Node, st.Storage, err))
				}
			}
			s.pollBackupVolumes(ctx, client, conn, n.Node, storages, deg)
		}

		// The plain top-taskLimit window and the vzdump-filtered window are
		// independent PVE API calls (different filter params, neither reads
		// the other's result) — fetched concurrently rather than back to
		// back so this step's wall-clock cost is one round-trip, not two,
		// per node per poll cycle.
		var tasks, vzdumpTasks []proxmoxclient.PVETask
		var tasksErr, vzdumpErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer safego.Recover(ctx, "proxmox.poller.getNodeTasks")
			tasks, tasksErr = client.GetNodeTasks(n.Node, taskLimit, "")
		}()
		go func() {
			defer wg.Done()
			defer safego.Recover(ctx, "proxmox.poller.getNodeVzdumpTasks")
			vzdumpTasks, vzdumpErr = client.GetNodeTasks(n.Node, taskLimit, "vzdump")
		}()
		wg.Wait()

		if tasksErr != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get tasks FAILED (needs Sys.Audit on /nodes/%s — without it PVE returns only the token's own tasks): %v", conn.Name, n.Node, n.Node, tasksErr))
			deg.note("tasks", tasksErr)
		} else {
			// The plain top-taskLimit window above can silently push an
			// older vzdump (backup) task out on a node busy with other
			// activity (replication, service actions, ...) — backups are
			// operationally the task type users most need visible (and
			// UpsertProxmoxBackupRun below depends on seeing it), so fetch
			// them again with PVE's own server-side type filter and merge,
			// deduped by UPID, rather than trust they survived the general
			// window.
			if vzdumpErr != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get vzdump tasks: %v", conn.Name, n.Node, vzdumpErr))
			} else {
				tasks = mergeTasksByUPID(tasks, vzdumpTasks)
			}

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
				if err := s.db.UpsertProxmoxTask(ctx,
					conn.ID, n.Node, t.UPID, t.Type, t.Status, t.User,
					startTime, endTime, t.ExitStatus, t.ID,
				); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert task %s: %v", conn.Name, n.Node, t.UPID, err))
				}
				if t.Type == "vzdump" && taskFinished(t) {
					if vmid := parseVMID(t.ID); vmid > 0 {
						outcome := taskOutcome(t)
						if err := s.db.UpsertProxmoxBackupRun(ctx,
							conn.ID, n.Node, vmid, t.UPID, normalizeBackupStatus(outcome),
							startTime, endTime, outcome,
						); err != nil {
							slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert backup run vmid=%d: %v", conn.Name, n.Node, vmid, err))
						}
					}
				}
			}
		}

		disks, diskErr := client.GetNodeDisksList(n.Node)
		if diskErr != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get disks FAILED (check Sys.Audit privilege on API token): %v", conn.Name, n.Node, diskErr))
			deg.note("disks", diskErr)
		} else {
			slog.InfoContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: got %d disk(s)", conn.Name, n.Node, len(disks)))
			for _, d := range disks {
				health := d.Health
				if health == "" {
					health = "UNKNOWN"
				}
				wearout := int(d.Wearout)
				if d.Type != "ssd" && d.Type != "nvme" {
					wearout = -1
				}
				if err := s.db.UpsertProxmoxDisk(ctx,
					conn.ID, n.Node, d.DevPath, d.Model, d.Serial,
					d.Size, d.Type, health, wearout,
				); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert disk %s: %v", conn.Name, n.Node, d.DevPath, err))
				}
			}
		}

		pkgs, aptErr := client.GetNodeAptUpdate(n.Node)
		if aptErr != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get apt/update FAILED (requires Sys.Modify — PVEAuditor is insufficient; create a custom role or add Sys.Modify to your token): %v", conn.Name, n.Node, aptErr))
			deg.note("apt updates", aptErr)
		} else {
			slog.InfoContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: got %d pending apt package(s)", conn.Name, n.Node, len(pkgs)))
		}
		pending := 0
		for range pkgs {
			pending++
		}
		if err := s.db.UpdateProxmoxNodeUpdates(ctx, conn.ID, n.Node, pending, 0); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: update node updates: %v", conn.Name, n.Node, err))
		}
	}

	backupJobs, err := client.GetClusterBackup()
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s]: get backup jobs: %v", conn.Name, err))
		deg.note("backup jobs", err)
	} else {
		for _, j := range backupJobs {
			if err := s.db.UpsertProxmoxBackupJob(ctx,
				conn.ID, j.ID, j.Enabled != 0,
				j.Schedule, j.Storage, j.Mode, j.Compress, j.VMIDs, j.MailTo,
			); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s]: upsert backup job %s: %v", conn.Name, j.ID, err))
			}
		}
		_ = s.db.DeleteStaleProxmoxBackupJobs(ctx, conn.ID, cutoff)
	}

	_ = s.db.DeleteStaleProxmoxGuests(ctx, conn.ID, cutoff)
	_ = s.db.DeleteStaleProxmoxNodes(ctx, conn.ID, cutoff)
	_ = s.db.DeleteStaleProxmoxTasks(ctx, conn.ID, cutoff)
	_ = s.db.DeleteStaleProxmoxDisks(ctx, conn.ID, cutoff)

	// A cycle that reached PVE but could not make every call is neither a
	// success nor an outage: last_success_at stays at the last fully clean poll
	// while last_error names what is missing.
	if msg := deg.summary(); msg != "" {
		_ = s.db.UpdateProxmoxConnectionError(ctx, conn.ID, msg)
	} else {
		_ = s.db.UpdateProxmoxConnectionSuccess(ctx, conn.ID)
	}
	slog.InfoContext(ctx, fmt.Sprintf("proxmox poller [%s]: poll complete (%d node(s))", conn.Name, len(nodes)))
}

// pollBackupVolumes derives a last-backup-per-guest result from the files on
// each backup-capable storage.
//
// The task list alone cannot supply this: a vzdump *job* spanning several
// guests runs as one PVE task with no vmid attached, so its per-guest results
// exist only inside that task's log. The backup volumes, by contrast, are one
// file per guest and carry a creation time. Whatever the task list did manage
// to report stays authoritative — UpsertProxmoxBackupRunFromStorage refuses to
// overwrite a newer row — so this only fills the gaps.
func (s *Poller) pollBackupVolumes(
	ctx context.Context,
	client *proxmoxclient.Client,
	conn database.ProxmoxConnectionFull,
	node string,
	storages []proxmoxclient.PVEStorage,
	deg *degraded,
) {
	// Newest volume wins per guest; a storage holds every retained backup, not
	// just the latest.
	newest := map[int]int64{}
	for _, st := range storages {
		if st.Active == 0 || !st.SupportsBackup() {
			continue
		}
		entries, err := client.GetStorageBackups(node, st.Storage)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: list backups on %s: %v", conn.Name, node, st.Storage, err))
			deg.note("backup volumes", err)
			continue
		}
		for _, e := range entries {
			vmid := e.GuestID()
			if vmid == 0 || e.CTime <= 0 {
				continue
			}
			if e.CTime > newest[vmid] {
				newest[vmid] = e.CTime
			}
		}
	}

	for vmid, ctime := range newest {
		// A retained volume is proof the backup completed; a failed run leaves
		// no file (or a .tmp PVE does not list as backup content).
		if err := s.db.UpsertProxmoxBackupRunFromStorage(
			ctx, conn.ID, node, vmid, BackupStatusOK, time.Unix(ctime, 0).UTC(),
		); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert backup volume vmid=%d: %v", conn.Name, node, vmid, err))
		}
	}
}

// degraded collects the PVE endpoints that failed during one poll cycle.
//
// Until now only a total failure to list nodes reached last_error; every other
// call — tasks, disks, apt, storage, backup volumes — was logged and nothing
// more, so a token missing one privilege produced a silently empty tab with no
// hint anywhere in the interface.
//
// It accumulates rather than writing on each failure because the cycle ends by
// marking the connection successful, which would otherwise erase whatever the
// same cycle had just recorded. Endpoints are deduplicated: the same call
// failing on every node of a cluster is one problem, not N.
type degraded struct {
	seen  map[string]bool
	order []string
}

func (d *degraded) note(what string, cause error) {
	if d.seen == nil {
		d.seen = map[string]bool{}
	}
	msg := fmt.Sprintf("%s: %v", what, cause)
	if d.seen[what] {
		return
	}
	d.seen[what] = true
	d.order = append(d.order, msg)
}

// summary renders the accumulated failures, or "" when the cycle was clean.
func (d *degraded) summary() string {
	if len(d.order) == 0 {
		return ""
	}
	out := strings.Join(d.order, " | ")
	// last_error is displayed in a badge tooltip and a banner; an unbounded
	// upstream message would make both unusable.
	if len(out) > 500 {
		out = out[:500] + "…"
	}
	return out
}

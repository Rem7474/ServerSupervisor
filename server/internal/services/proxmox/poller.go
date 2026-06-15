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
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/proxmoxclient"
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
					vm.CPUs, vm.CPU, vm.MaxMem, vm.Mem, vm.MaxDisk, vm.Uptime, vm.Tags,
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
					lxc.CPUs, lxc.CPU, lxc.MaxMem, lxc.Mem, lxc.MaxDisk, lxc.Uptime, lxc.Tags,
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
		}

		tasks, err := client.GetNodeTasks(n.Node, taskLimit)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get tasks: %v", conn.Name, n.Node, err))
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
				if err := s.db.UpsertProxmoxTask(ctx,
					conn.ID, n.Node, t.UPID, t.Type, t.Status, t.User,
					startTime, endTime, t.ExitStatus, t.ID,
				); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: upsert task %s: %v", conn.Name, n.Node, t.UPID, err))
				}
				if t.Type == "vzdump" && t.Status == "stopped" && t.ID != "" {
					if vmid := parseVMID(t.ID); vmid > 0 {
						_ = s.db.UpsertProxmoxBackupRun(ctx,
							conn.ID, n.Node, vmid, t.UPID, t.ExitStatus,
							startTime, endTime, t.ExitStatus,
						)
					}
				}
			}
		}

		disks, diskErr := client.GetNodeDisksList(n.Node)
		if diskErr != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("proxmox poller [%s/%s]: get disks FAILED (check Sys.Audit privilege on API token): %v", conn.Name, n.Node, diskErr))
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

	_ = s.db.UpdateProxmoxConnectionSuccess(ctx, conn.ID)
	slog.InfoContext(ctx, fmt.Sprintf("proxmox poller [%s]: poll complete (%d node(s))", conn.Name, len(nodes)))
}

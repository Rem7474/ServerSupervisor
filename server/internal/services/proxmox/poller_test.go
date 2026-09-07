package proxmox

import (
	"testing"

	"github.com/serversupervisor/server/internal/proxmoxclient"
)

func TestMergeTasksByUPID(t *testing.T) {
	base := []proxmoxclient.PVETask{
		{UPID: "UPID:pve1:A", Type: "qmstart"},
		{UPID: "UPID:pve1:B", Type: "vzdump"},
	}
	extra := []proxmoxclient.PVETask{
		{UPID: "UPID:pve1:B", Type: "vzdump"}, // already in base — must not duplicate
		{UPID: "UPID:pve1:C", Type: "vzdump"}, // pushed out of the top-N window — must be added
	}

	got := mergeTasksByUPID(base, extra)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3: %+v", len(got), got)
	}
	upids := make(map[string]bool, len(got))
	for _, task := range got {
		if upids[task.UPID] {
			t.Fatalf("duplicate UPID %q in result: %+v", task.UPID, got)
		}
		upids[task.UPID] = true
	}
	if !upids["UPID:pve1:C"] {
		t.Errorf("missing UPID:pve1:C (the task outside the general window) in %+v", got)
	}
}

func TestMergeTasksByUPIDNoExtra(t *testing.T) {
	base := []proxmoxclient.PVETask{{UPID: "UPID:pve1:A"}}
	got := mergeTasksByUPID(base, nil)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
}

func TestTaskFinished(t *testing.T) {
	// The task *list* endpoint reports the outcome in `status` and leaves
	// `exitstatus` empty; the per-task status endpoint uses "stopped" plus a
	// separate `exitstatus`. Both shapes have to be recognised, or backup runs
	// are silently dropped — which is exactly what testing status == "stopped"
	// alone did.
	tests := []struct {
		name string
		task proxmoxclient.PVETask
		want bool
	}{
		{"list shape, success", proxmoxclient.PVETask{Status: "OK"}, true},
		{"list shape, failure", proxmoxclient.PVETask{Status: "job errors"}, true},
		{"status-endpoint shape", proxmoxclient.PVETask{Status: "stopped", ExitStatus: "OK"}, true},
		{"still running", proxmoxclient.PVETask{Status: "running"}, false},
		{"no data at all", proxmoxclient.PVETask{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := taskFinished(tc.task); got != tc.want {
				t.Errorf("taskFinished(%+v) = %v, want %v", tc.task, got, tc.want)
			}
		})
	}
}

func TestTaskOutcome(t *testing.T) {
	tests := []struct {
		name string
		task proxmoxclient.PVETask
		want string
	}{
		{"prefers the dedicated field", proxmoxclient.PVETask{Status: "stopped", ExitStatus: "OK"}, "OK"},
		{"falls back to the overloaded status", proxmoxclient.PVETask{Status: "job errors"}, "job errors"},
		{"running has no outcome yet", proxmoxclient.PVETask{Status: "running"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := taskOutcome(tc.task); got != tc.want {
				t.Errorf("taskOutcome(%+v) = %q, want %q", tc.task, got, tc.want)
			}
		})
	}
}

func TestNormalizeBackupStatus(t *testing.T) {
	// proxmox_backup_runs.status feeds a coloured, translated badge; the raw
	// PVE string belongs in exit_status. Storing the raw string in both made a
	// failure render as an unbounded grey badge quoting a storage error.
	tests := []struct {
		outcome string
		want    string
	}{
		{"OK", BackupStatusOK},
		{"ok", BackupStatusOK},
		{"", BackupStatusRunning},
		{"   ", BackupStatusRunning},
		{"job errors", BackupStatusFailed},
		{"could not activate storage 'pbs-immich': error fetching datastores - 500", BackupStatusFailed},
		// A job that skipped a guest still ran; reporting it as failed would be
		// wrong.
		{"WARNINGS: 1", BackupStatusWarnings},
		{"warnings: 3", BackupStatusWarnings},
	}
	for _, tc := range tests {
		if got := normalizeBackupStatus(tc.outcome); got != tc.want {
			t.Errorf("normalizeBackupStatus(%q) = %q, want %q", tc.outcome, got, tc.want)
		}
	}
}

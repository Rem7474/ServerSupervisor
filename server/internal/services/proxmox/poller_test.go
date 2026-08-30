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

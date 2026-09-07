package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// fakePVEPollServer answers every REST call PollOne makes for one online
// node ("pve1") with minimal/empty data, except /nodes/pve1/tasks: it
// returns a different task depending on PVE's typefilter query param, so the
// vzdump merge added alongside backup-run visibility (see poller.go's
// mergeTasksByUPID doc comment) can be exercised end-to-end against a real
// Postgres database.
func fakePVEPollServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"node":"pve1","status":"online","cpu":0.1,"maxcpu":4,"mem":1000,"maxmem":2000,"uptime":100,"pveversion":"pve-manager/8.0"}]}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/storage", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("typefilter") == "vzdump" {
			// This vzdump task is deliberately absent from the plain window
			// below, simulating it having been pushed out by other node
			// activity — mergeTasksByUPID must fold it back in.
			_, _ = w.Write([]byte(`{"data":[{"upid":"UPID:pve1:VZDUMP","type":"vzdump","status":"stopped","user":"root@pam","starttime":1000,"endtime":1100,"id":"101","exitstatus":"OK"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"upid":"UPID:pve1:OTHER","type":"qmstart","status":"stopped","user":"root@pam","starttime":900,"endtime":950,"id":"101","exitstatus":"OK"}]}`))
	})
	mux.HandleFunc("/nodes/pve1/disks/list", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/apt/update", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/cluster/backup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	return httptest.NewServer(mux)
}

// TestPollOne_MergesVzdumpTasksAndRecordsBackupRun exercises PollOne
// end-to-end against a real Postgres database and a fake PVE server: the
// general task window and the vzdump-filtered fetch return different tasks,
// so both must land in proxmox_tasks (merged, deduped) and the vzdump one
// must produce a proxmox_backup_runs row.
func TestPollOne_MergesVzdumpTasksAndRecordsBackupRun(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	pve := fakePVEPollServer(t)
	t.Cleanup(pve.Close)

	connID, err := db.CreateProxmoxConnection(ctx, models.ProxmoxConnectionRequest{
		Name: "poll-test", APIURL: pve.URL, TokenID: "user@pve!token", TokenSecret: "secret",
		Enabled: true, PollIntervalSec: 60,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	conns, err := db.GetEnabledProxmoxConnections(ctx)
	if err != nil || len(conns) != 1 {
		t.Fatalf("get enabled connections: %v %+v", err, conns)
	}

	p := NewPoller(db, nil)
	p.PollOne(ctx, conns[0])

	tasks, err := db.ListProxmoxTasksByNode(ctx, connID, "pve1", 100)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	upids := make(map[string]bool, len(tasks))
	for _, tsk := range tasks {
		upids[tsk.UPID] = true
	}
	if !upids["UPID:pve1:OTHER"] || !upids["UPID:pve1:VZDUMP"] {
		t.Fatalf("expected both the general and the vzdump-filtered task to be stored, got %+v", fmt.Sprint(tasks))
	}

	runs, err := db.ListProxmoxBackupRuns(ctx, connID)
	if err != nil {
		t.Fatalf("list backup runs: %v", err)
	}
	found := false
	for _, run := range runs {
		if run.TaskUPID == "UPID:pve1:VZDUMP" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a backup run for the vzdump task, got %+v", runs)
	}
}

// fakePVEListShapeServer reproduces what a real PVE returned in the field: the
// task *list* puts the outcome straight into `status` ("OK", "job errors") and
// sends no `exitstatus`, and a multi-guest backup job runs as a single task
// carrying no vmid at all. Backups therefore come from two places — the tasks
// that do name a guest, and the volumes on the backup storage for the ones
// swallowed by the job-level task.
func fakePVEListShapeServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"node":"pve1","status":"online","cpu":0.1,"maxcpu":4,"mem":1000,"maxmem":2000,"uptime":100}]}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/storage", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"storage":"backups","type":"dir","active":1,"enabled":1,"content":"backup,iso"}]}`))
	})
	mux.HandleFunc("/nodes/pve1/storage/backups/content", func(w http.ResponseWriter, r *http.Request) {
		// vmid 300 is only ever covered by the job-level task, so the volume is
		// the sole evidence it was backed up. 200 also has a volume, older than
		// its own task result.
		_, _ = w.Write([]byte(`{"data":[
			{"volid":"backups:backup/vzdump-qemu-300-2026_09_07.vma.zst","vmid":300,"ctime":1757203200,"format":"vma.zst"},
			{"volid":"backups:backup/vzdump-qemu-200-2026_09_01.vma.zst","vmid":200,"ctime":1000,"format":"vma.zst"}
		]}`))
	})
	mux.HandleFunc("/nodes/pve1/tasks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"upid":"UPID:pve1:VM200","type":"vzdump","status":"job errors","user":"root@pam","starttime":1757200000,"endtime":1757205000,"id":"200"},
			{"upid":"UPID:pve1:JOB","type":"vzdump","status":"job errors","user":"root@pam","starttime":1757200000,"endtime":1757201800,"id":""}
		]}`))
	})
	mux.HandleFunc("/nodes/pve1/disks/list", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/nodes/pve1/apt/update", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/cluster/backup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	return httptest.NewServer(mux)
}

func TestPollOne_RecordsBackupRunsFromListShapeAndStorage(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	pve := fakePVEListShapeServer(t)
	t.Cleanup(pve.Close)

	connID, err := db.CreateProxmoxConnection(ctx, models.ProxmoxConnectionRequest{
		Name: "list-shape", APIURL: pve.URL, TokenID: "user@pve!token", TokenSecret: "secret",
		Enabled: true, PollIntervalSec: 60,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	conns, err := db.GetEnabledProxmoxConnections(ctx)
	if err != nil || len(conns) != 1 {
		t.Fatalf("get enabled connections: %v %+v", err, conns)
	}

	NewPoller(db, nil).PollOne(ctx, conns[0])

	runs, err := db.ListProxmoxBackupRuns(ctx, connID)
	if err != nil {
		t.Fatalf("list backup runs: %v", err)
	}
	byVMID := make(map[int]models.ProxmoxBackupRun, len(runs))
	for _, r := range runs {
		byVMID[r.VMID] = r
	}

	// The task names vmid 200 and reports a failure in `status`. Testing
	// status == "stopped" dropped it, leaving the tab permanently empty.
	run200, ok := byVMID[200]
	if !ok {
		t.Fatalf("no run recorded for vmid 200, got %+v", runs)
	}
	if run200.Status != "job errors" {
		t.Errorf("vmid 200 status = %q, want %q", run200.Status, "job errors")
	}
	if run200.TaskUPID != "UPID:pve1:VM200" {
		t.Errorf("vmid 200 lost its task link: %q", run200.TaskUPID)
	}

	// vmid 300 appears in no task — only the job-level one, which carries no
	// vmid — so its volume on the backup storage is the only evidence.
	run300, ok := byVMID[300]
	if !ok {
		t.Fatalf("no run recorded for vmid 300 from storage, got %+v", runs)
	}
	if run300.TaskUPID != "" {
		t.Errorf("vmid 300 should have no task link, got %q", run300.TaskUPID)
	}
	if run300.EndTime == nil || run300.EndTime.Unix() != 1757203200 {
		t.Errorf("vmid 300 end_time = %v, want the volume ctime", run300.EndTime)
	}

	// The job-level task must not invent a run of its own.
	if _, ok := byVMID[0]; ok {
		t.Error("the vmid-less job task should not produce a run")
	}
}

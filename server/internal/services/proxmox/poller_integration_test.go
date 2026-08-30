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

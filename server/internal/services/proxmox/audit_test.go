package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

var testActor = Actor{Username: "alice", ClientIP: "10.0.0.7"}

func requireOneAudit(t *testing.T, repo *fakeRepo, action, status string) auditEntry {
	t.Helper()
	if len(repo.audits) != 1 {
		t.Fatalf("expected exactly 1 audit entry, got %d: %+v", len(repo.audits), repo.audits)
	}
	a := repo.audits[0]
	if a.action != action || a.status != status || a.username != "alice" || a.ip != "10.0.0.7" {
		t.Fatalf("unexpected audit entry %+v (want action=%s status=%s, alice@10.0.0.7)", a, action, status)
	}
	return a
}

func newPVEActionStub(t *testing.T, code int) (*httptest.Server, *fakeRepo) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code == http.StatusOK {
			_, _ = w.Write([]byte(`{"data":"UPID:pve1:0001"}`))
		} else {
			_, _ = w.Write([]byte(`{"data":null}`))
		}
	}))
	t.Cleanup(srv.Close)

	repo := &fakeRepo{
		enabledConns: []database.ProxmoxConnectionFull{{
			ProxmoxConnection: models.ProxmoxConnection{ID: "c1", APIURL: srv.URL, TokenID: "root@pam!tok"},
			TokenSecret:       "secret",
		}},
		connByID: map[string]*models.ProxmoxConnection{
			"c1": {ID: "c1", APIURL: srv.URL, TokenID: "root@pam!tok"},
		},
		nodeByID: map[string]*models.ProxmoxNode{
			"n1": {ID: "n1", ConnectionID: "c1", NodeName: "pve1"},
		},
		guestByID: map[string]*models.ProxmoxGuest{
			"g1": {ID: "g1", ConnectionID: "c1", NodeName: "pve1", VMID: 100, GuestType: "lxc"},
		},
	}
	return srv, repo
}

func TestCreateLink_WritesAudit(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newSvc(repo).CreateLink(context.Background(), models.ProxmoxGuestLinkRequest{GuestID: "g1", HostID: "h1"}, testActor); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	requireOneAudit(t, repo, "proxmox_link_create", "success")
}

func TestUpdateLink_WritesAudit(t *testing.T) {
	repo := &fakeRepo{link: &models.ProxmoxGuestLink{ID: "l1", HostID: "h1"}}
	ok := "confirmed"
	if _, err := newSvc(repo).UpdateLink(context.Background(), "l1", models.ProxmoxGuestLinkUpdate{Status: &ok}, testActor); err != nil {
		t.Fatalf("UpdateLink: %v", err)
	}
	requireOneAudit(t, repo, "proxmox_link_update", "success")
}

func TestUpdateLink_ValidationFailureIsNotAudited(t *testing.T) {
	repo := &fakeRepo{link: &models.ProxmoxGuestLink{}}
	bad := "bogus"
	if _, err := newSvc(repo).UpdateLink(context.Background(), "l1", models.ProxmoxGuestLinkUpdate{Status: &bad}, testActor); err == nil {
		t.Fatal("expected a validation error")
	}
	if len(repo.audits) != 0 {
		t.Fatalf("rejected input must not be audited as a mutation, got %+v", repo.audits)
	}
}

func TestDeleteLink_WritesAudit(t *testing.T) {
	repo := &fakeRepo{link: &models.ProxmoxGuestLink{ID: "l1", HostID: "h1"}}
	if err := newSvc(repo).DeleteLink(context.Background(), "l1", testActor); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	requireOneAudit(t, repo, "proxmox_link_delete", "success")
}

func TestDeleteLink_NotFoundIsNotAudited(t *testing.T) {
	repo := &fakeRepo{}
	if err := newSvc(repo).DeleteLink(context.Background(), "missing", testActor); err == nil {
		t.Fatal("expected not found")
	}
	if len(repo.audits) != 0 {
		t.Fatalf("no mutation happened, nothing to audit: %+v", repo.audits)
	}
}

func TestGuestAction_WritesAudit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusOK)
		upid, err := newSvc(repo).GuestAction(context.Background(), "g1", "reboot", testActor)
		if err != nil || upid != "UPID:pve1:0001" {
			t.Fatalf("GuestAction = (%q, %v)", upid, err)
		}
		a := requireOneAudit(t, repo, "proxmox_guest_action", "success")
		if !strings.Contains(a.details, "action=reboot") || !strings.Contains(a.details, "vmid=100") {
			t.Errorf("unexpected details: %q", a.details)
		}
	})

	t.Run("pve failure", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusInternalServerError)
		if _, err := newSvc(repo).GuestAction(context.Background(), "g1", "shutdown", testActor); err == nil {
			t.Fatal("expected PVE error")
		}
		requireOneAudit(t, repo, "proxmox_guest_action", "failed")
	})

	t.Run("invalid action not audited", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusOK)
		if _, err := newSvc(repo).GuestAction(context.Background(), "g1", "stop", testActor); err == nil {
			t.Fatal("expected validation error")
		}
		if len(repo.audits) != 0 {
			t.Fatalf("rejected action must not be audited, got %+v", repo.audits)
		}
	})
}

func TestMigrateGuest_WritesAudit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusOK)
		upid, err := newSvc(repo).MigrateGuest(context.Background(), "n1", 100, "lxc", "pve2", true, testActor)
		if err != nil || upid != "UPID:pve1:0001" {
			t.Fatalf("MigrateGuest = (%q, %v)", upid, err)
		}
		a := requireOneAudit(t, repo, "proxmox_guest_migrate", "success")
		if !strings.Contains(a.details, "target=pve2") {
			t.Errorf("unexpected details: %q", a.details)
		}
	})

	t.Run("pve failure", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusInternalServerError)
		if _, err := newSvc(repo).MigrateGuest(context.Background(), "n1", 100, "vm", "pve2", false, testActor); err == nil {
			t.Fatal("expected PVE error")
		}
		requireOneAudit(t, repo, "proxmox_guest_migrate", "failed")
	})
}

func TestNodeServiceAction_WritesAudit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusOK)
		upid, err := newSvc(repo).NodeServiceAction(context.Background(), "n1", "pveproxy", "restart", testActor)
		if err != nil || upid != "UPID:pve1:0001" {
			t.Fatalf("NodeServiceAction = (%q, %v)", upid, err)
		}
		a := requireOneAudit(t, repo, "proxmox_node_service_action", "success")
		if !strings.Contains(a.details, "service=pveproxy") || !strings.Contains(a.details, "action=restart") {
			t.Errorf("unexpected details: %q", a.details)
		}
	})

	t.Run("pve failure", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusInternalServerError)
		if _, err := newSvc(repo).NodeServiceAction(context.Background(), "n1", "pveproxy", "restart", testActor); err == nil {
			t.Fatal("expected PVE error")
		}
		requireOneAudit(t, repo, "proxmox_node_service_action", "failed")
	})
}

func TestRefreshNodeApt_WritesAudit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusOK)
		upid, err := newSvc(repo).RefreshNodeApt(context.Background(), "n1", testActor)
		if err != nil || upid != "UPID:pve1:0001" {
			t.Fatalf("RefreshNodeApt = (%q, %v)", upid, err)
		}
		requireOneAudit(t, repo, "proxmox_node_apt_refresh", "success")
	})

	t.Run("pve failure", func(t *testing.T) {
		_, repo := newPVEActionStub(t, http.StatusInternalServerError)
		if _, err := newSvc(repo).RefreshNodeApt(context.Background(), "n1", testActor); err == nil {
			t.Fatal("expected PVE error")
		}
		requireOneAudit(t, repo, "proxmox_node_apt_refresh", "failed")
	})
}

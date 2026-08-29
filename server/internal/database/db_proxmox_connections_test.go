package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func baseConnRequest(name string) models.ProxmoxConnectionRequest {
	return models.ProxmoxConnectionRequest{
		Name:               name,
		APIURL:             "https://pve.example.com:8006",
		TokenID:            "root@pam!monitoring",
		TokenSecret:        "top-secret-token",
		InsecureSkipVerify: true,
		Enabled:            true,
	}
}

// TestProxmoxConnectionsCRUD exercises the full lifecycle including the
// console-credential columns (pve_username/pve_password) added alongside the
// interactive LXC console feature — see root CLAUDE.md's "Proxmox
// integration" section.
func TestProxmoxConnectionsCRUD(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	req := baseConnRequest("pve-crud-test")
	id, err := db.CreateProxmoxConnection(ctx, req)
	if err != nil || id == "" {
		t.Fatalf("create: id=%q err=%v", id, err)
	}

	t.Run("GetProxmoxConnectionByID", func(t *testing.T) {
		got, err := db.GetProxmoxConnectionByID(ctx, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got == nil || got.Name != req.Name || got.APIURL != req.APIURL {
			t.Fatalf("got = %+v", got)
		}
		if got.ConsoleConfigured {
			t.Error("ConsoleConfigured should be false with no PVE console credentials set")
		}
	})

	t.Run("GetProxmoxConnectionByID missing", func(t *testing.T) {
		got, err := db.GetProxmoxConnectionByID(ctx, "00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for a missing connection, got %+v", got)
		}
	})

	t.Run("ListProxmoxConnections", func(t *testing.T) {
		conns, err := db.ListProxmoxConnections(ctx)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		found := false
		for _, c := range conns {
			if c.ID == id {
				found = true
				if c.ConsoleConfigured {
					t.Error("ConsoleConfigured should be false before console credentials are set")
				}
			}
		}
		if !found {
			t.Fatalf("created connection %q not found in list", id)
		}
	})

	t.Run("GetProxmoxTokenSecret", func(t *testing.T) {
		secret, err := db.GetProxmoxTokenSecret(ctx, id)
		if err != nil || secret != req.TokenSecret {
			t.Fatalf("secret = %q, err = %v, want %q", secret, err, req.TokenSecret)
		}
	})

	t.Run("GetProxmoxConsoleCredentials before configured", func(t *testing.T) {
		user, pass, err := db.GetProxmoxConsoleCredentials(ctx, id)
		if err != nil || user != "" || pass != "" {
			t.Fatalf("got user=%q pass=%q err=%v, want empty/nil", user, pass, err)
		}
	})

	t.Run("UpdateProxmoxConnection sets console credentials and keeps token secret", func(t *testing.T) {
		update := req
		update.Name = "pve-crud-test-renamed"
		update.TokenSecret = "" // keep existing
		update.PVEUsername = "root@pam"
		update.PVEPassword = "hunter2"
		if err := db.UpdateProxmoxConnection(ctx, id, update); err != nil {
			t.Fatalf("update: %v", err)
		}

		got, err := db.GetProxmoxConnectionByID(ctx, id)
		if err != nil || got == nil {
			t.Fatalf("get after update: %v", err)
		}
		if got.Name != "pve-crud-test-renamed" {
			t.Errorf("name not updated, got %q", got.Name)
		}
		if !got.ConsoleConfigured {
			t.Error("ConsoleConfigured should be true once pve_username/pve_password are set")
		}
		if got.PVEUsername != "root@pam" {
			t.Errorf("PVEUsername = %q, want root@pam", got.PVEUsername)
		}
		secret, err := db.GetProxmoxTokenSecret(ctx, id)
		if err != nil || secret != req.TokenSecret {
			t.Fatalf("token secret should be preserved on empty update, got %q err=%v", secret, err)
		}
		user, pass, err := db.GetProxmoxConsoleCredentials(ctx, id)
		if err != nil || user != "root@pam" || pass != "hunter2" {
			t.Fatalf("console creds = (%q,%q) err=%v, want (root@pam,hunter2)", user, pass, err)
		}
	})

	t.Run("UpdateProxmoxConnection rotates token secret and keeps console password", func(t *testing.T) {
		update := req
		update.Name = "pve-crud-test-renamed"
		update.TokenSecret = "rotated-secret"
		update.PVEUsername = "root@pam"
		update.PVEPassword = "" // keep existing console password
		if err := db.UpdateProxmoxConnection(ctx, id, update); err != nil {
			t.Fatalf("update: %v", err)
		}
		secret, err := db.GetProxmoxTokenSecret(ctx, id)
		if err != nil || secret != "rotated-secret" {
			t.Fatalf("token secret = %q err=%v, want rotated-secret", secret, err)
		}
		user, pass, err := db.GetProxmoxConsoleCredentials(ctx, id)
		if err != nil || user != "root@pam" || pass != "hunter2" {
			t.Fatalf("console password should be preserved, got (%q,%q) err=%v", user, pass, err)
		}
	})

	t.Run("UpdateProxmoxConnection rotates both secrets at once", func(t *testing.T) {
		update := req
		update.TokenSecret = "double-rotated"
		update.PVEUsername = "root@pam"
		update.PVEPassword = "new-console-pass"
		if err := db.UpdateProxmoxConnection(ctx, id, update); err != nil {
			t.Fatalf("update: %v", err)
		}
		secret, _ := db.GetProxmoxTokenSecret(ctx, id)
		if secret != "double-rotated" {
			t.Errorf("token secret = %q, want double-rotated", secret)
		}
		user, pass, _ := db.GetProxmoxConsoleCredentials(ctx, id)
		if user != "root@pam" || pass != "new-console-pass" {
			t.Errorf("console creds = (%q,%q), want (root@pam,new-console-pass)", user, pass)
		}
	})

	t.Run("GetEnabledProxmoxConnections", func(t *testing.T) {
		conns, err := db.GetEnabledProxmoxConnections(ctx)
		if err != nil {
			t.Fatalf("get enabled: %v", err)
		}
		found := false
		for _, c := range conns {
			if c.ID == id {
				found = true
				if c.TokenSecret != "double-rotated" {
					t.Errorf("TokenSecret = %q, want double-rotated", c.TokenSecret)
				}
			}
		}
		if !found {
			t.Fatalf("enabled connection %q not found", id)
		}
	})

	t.Run("DeleteProxmoxConnection", func(t *testing.T) {
		if err := db.DeleteProxmoxConnection(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		got, err := db.GetProxmoxConnectionByID(ctx, id)
		if err != nil {
			t.Fatalf("get after delete: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil after delete, got %+v", got)
		}
	})
}

// TestCreateProxmoxConnection_DefaultPollInterval covers the "poll_interval_sec
// <= 0 defaults to 60" branch shared by Create and Update.
func TestCreateProxmoxConnection_DefaultPollInterval(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	req := baseConnRequest("pve-default-interval")
	req.PollIntervalSec = 0
	id, err := db.CreateProxmoxConnection(ctx, req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := db.GetProxmoxConnectionByID(ctx, id)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.PollIntervalSec != 60 {
		t.Errorf("PollIntervalSec = %d, want default 60", got.PollIntervalSec)
	}
}

// TestGetProxmoxConsoleCredentials_UnknownConnection covers the sql.ErrNoRows
// -> ("", "", nil) branch, distinct from a configured-but-empty connection.
func TestGetProxmoxConsoleCredentials_UnknownConnection(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	user, pass, err := db.GetProxmoxConsoleCredentials(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("unexpected error for unknown connection: %v", err)
	}
	if user != "" || pass != "" {
		t.Errorf("got user=%q pass=%q, want empty", user, pass)
	}
}

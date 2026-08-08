package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestCreateAuditLog_AssignsCategory is the regression test for ROADMAP.md
// item #13: category is computed once at write time (models.CategorizeAuditAction),
// not left at the column default for every row.
func TestCreateAuditLog_AssignsCategory(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	id, err := db.CreateAuditLog(ctx, "alert-engine", "alert_fired", "", "", "{}", "success")
	if err != nil {
		t.Fatalf("CreateAuditLog: %v", err)
	}

	logs, err := db.GetAuditLogs(ctx, 10, 0, database.AuditLogFilter{})
	if err != nil {
		t.Fatalf("GetAuditLogs: %v", err)
	}
	var found *models.AuditLog
	for i := range logs {
		if logs[i].ID == id {
			found = &logs[i]
		}
	}
	if found == nil {
		t.Fatalf("created audit log %d not found", id)
	}
	if found.Category != models.AuditCategoryAlert {
		t.Errorf("category = %q, want %q", found.Category, models.AuditCategoryAlert)
	}
}

// TestGetAuditLogs_FiltersByCategoryAndDateRange covers the filtered read
// path behind GET /audit/logs and the CSV export.
func TestGetAuditLogs_FiltersByCategoryAndDateRange(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if _, err := db.CreateAuditLog(ctx, "admin", "update_settings", "", "", "", "success"); err != nil {
		t.Fatalf("seed settings log: %v", err)
	}
	if _, err := db.CreateAuditLog(ctx, "alert-engine", "alert_fired", "", "", "", "success"); err != nil {
		t.Fatalf("seed alert log: %v", err)
	}

	t.Run("category filter", func(t *testing.T) {
		logs, err := db.GetAuditLogs(ctx, 10, 0, database.AuditLogFilter{Category: models.AuditCategorySettings})
		if err != nil {
			t.Fatalf("GetAuditLogs: %v", err)
		}
		if len(logs) != 1 || logs[0].Action != "update_settings" {
			t.Fatalf("category filter = %+v, want exactly the settings log", logs)
		}
	})

	t.Run("date range excludes everything when From is in the future", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		logs, err := db.GetAuditLogs(ctx, 10, 0, database.AuditLogFilter{From: &future})
		if err != nil {
			t.Fatalf("GetAuditLogs: %v", err)
		}
		if len(logs) != 0 {
			t.Fatalf("expected no logs after a future From, got %d", len(logs))
		}
	})

	t.Run("date range includes everything when From is in the past", func(t *testing.T) {
		past := time.Now().Add(-24 * time.Hour)
		logs, err := db.GetAuditLogs(ctx, 10, 0, database.AuditLogFilter{From: &past})
		if err != nil {
			t.Fatalf("GetAuditLogs: %v", err)
		}
		if len(logs) != 2 {
			t.Fatalf("expected both seeded logs, got %d", len(logs))
		}
	})
}

// TestCleanOldAuditLogsByCategory_OnlyTouchesItsOwnCategory ensures a
// per-category retention purge doesn't also delete a different category's
// logs that happen to be equally old — the whole point of splitting
// retention by category instead of one global DELETE.
func TestCleanOldAuditLogsByCategory_OnlyTouchesItsOwnCategory(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if _, err := db.CreateAuditLog(ctx, "admin", "update_settings", "", "", "", "success"); err != nil {
		t.Fatalf("seed settings log: %v", err)
	}
	if _, err := db.CreateAuditLog(ctx, "alert-engine", "alert_fired", "", "", "", "success"); err != nil {
		t.Fatalf("seed alert log: %v", err)
	}

	// retentionDays=0 means "older than right now" — both rows are old enough.
	deleted, err := db.CleanOldAuditLogsByCategory(ctx, models.AuditCategorySettings, 0)
	if err != nil {
		t.Fatalf("CleanOldAuditLogsByCategory: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only the settings-category row)", deleted)
	}

	remaining, err := db.GetAuditLogs(ctx, 10, 0, database.AuditLogFilter{})
	if err != nil {
		t.Fatalf("GetAuditLogs: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Action != "alert_fired" {
		t.Fatalf("remaining logs = %+v, want only the alert log untouched", remaining)
	}
}

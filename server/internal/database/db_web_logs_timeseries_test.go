package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func TestGetWebLogsTimeseries(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	for _, id := range []string{"ts-host-a", "ts-host-b"} {
		if err := db.RegisterHost(ctx, &models.Host{ID: id, Name: id, Hostname: id + ".local", APIKey: "x", Status: "online"}); err != nil {
			t.Fatalf("register host %s: %v", id, err)
		}
	}

	base := time.Date(2026, 1, 10, 10, 5, 0, 0, time.UTC)
	req := func(offset time.Duration, ip string, status int) models.WebRequest {
		return models.WebRequest{
			Timestamp: base.Add(offset).Format(time.RFC3339),
			IP:        ip, Method: "GET", Path: "/", Status: status, Domain: "example.com",
		}
	}
	insert := func(hostID string, reqs ...models.WebRequest) {
		t.Helper()
		if err := db.InsertWebLogSnapshot(ctx, hostID, &models.WebLogReport{
			Source: "nginx", CollectedAt: base.Add(2 * time.Hour), Requests: reqs,
		}); err != nil {
			t.Fatalf("insert snapshot for %s: %v", hostID, err)
		}
	}
	insert("ts-host-a",
		req(0, "10.0.0.1", 200),
		req(time.Minute, "10.0.0.2", 404),
		req(time.Hour, "10.0.0.3", 500),
	)
	insert("ts-host-b", req(0, "10.0.0.4", 200))

	since := base.Add(-time.Hour)
	until := base.Add(3 * time.Hour)

	t.Run("hour buckets", func(t *testing.T) {
		rows, err := db.GetWebLogsTimeseries(ctx, since, until, "", "", "hour")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("got %d buckets, want 2: %v", len(rows), rows)
		}
		if got := rows[0]["total"]; got != int64(3) {
			t.Errorf("first bucket total = %v, want 3", got)
		}
		if got := rows[1]["status_5xx"]; got != int64(1) {
			t.Errorf("second bucket 5xx = %v, want 1", got)
		}
	})

	t.Run("minute buckets", func(t *testing.T) {
		rows, err := db.GetWebLogsTimeseries(ctx, since, until, "", "", "minute")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("got %d buckets, want 3: %v", len(rows), rows)
		}
	})

	t.Run("host filter keeps the bucket parameter aligned after the filter arguments", func(t *testing.T) {
		rows, err := db.GetWebLogsTimeseries(ctx, since, until, "ts-host-b", "nginx", "hour")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0]["total"] != int64(1) {
			t.Fatalf("got %v, want a single bucket with total 1", rows)
		}
	})

	t.Run("an unknown bucket falls back to hour instead of reaching the query", func(t *testing.T) {
		rows, err := db.GetWebLogsTimeseries(ctx, since, until, "", "", "hour'); DROP TABLE web_log_requests; --")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("got %d buckets, want the 2 hourly ones", len(rows))
		}
	})
}

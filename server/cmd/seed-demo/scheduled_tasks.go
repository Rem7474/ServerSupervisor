package main

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type demoTask struct {
	Name           string
	HostID         string
	Module         string
	Action         string
	Target         string
	CronExpression string
	Enabled        bool
	LastRunStatus  string // "" = never run
	LastRunAgo     time.Duration
}

var demoTasks = []demoTask{
	{Name: "Demo - Nightly APT upgrade", HostID: "demo-web-01", Module: "apt", Action: "upgrade",
		CronExpression: "0 3 * * *", Enabled: true, LastRunStatus: "completed", LastRunAgo: 9 * time.Hour},
	{Name: "Demo - Docker prune", HostID: "demo-app-02", Module: "docker", Action: "prune",
		CronExpression: "30 2 * * 0", Enabled: true, LastRunStatus: "failed", LastRunAgo: 33 * time.Hour},
	{Name: "Demo - Log rotation check", HostID: "demo-db-01", Module: "journal", Action: "read", Target: "postgresql",
		CronExpression: "0 * * * *", Enabled: true, LastRunStatus: "completed", LastRunAgo: 40 * time.Minute},
	{Name: "Demo - Batch cleanup (disabled)", HostID: "demo-worker-01", Module: "processes", Action: "list",
		CronExpression: "0 0 * * *", Enabled: false},
}

// seedScheduledTasks reseeds every "Demo - " task from scratch (delete by
// name prefix, then recreate) — scheduled_tasks.id is DB-generated and
// nothing else in this seed package references it, so delete+recreate is
// simpler than a find-and-update path here.
func seedScheduledTasks(ctx context.Context, db *database.DB) error {
	if _, err := db.Exec(ctx, `DELETE FROM scheduled_tasks WHERE name LIKE 'Demo - %'`); err != nil {
		return err
	}

	for _, t := range demoTasks {
		created, err := db.CreateScheduledTask(ctx, models.ScheduledTask{
			HostID: t.HostID, Name: t.Name, Module: t.Module, Action: t.Action, Target: t.Target,
			CronExpression: t.CronExpression, Enabled: t.Enabled, CreatedBy: "demo-seed",
		})
		if err != nil {
			return err
		}
		if t.LastRunStatus == "" {
			continue
		}
		lastRun := anchor.Add(-t.LastRunAgo)
		nextRun := lastRun.Add(24 * time.Hour)
		if err := db.UpdateScheduledTaskRun(ctx, created.ID, t.LastRunStatus, lastRun, nextRun); err != nil {
			return err
		}
	}
	return nil
}

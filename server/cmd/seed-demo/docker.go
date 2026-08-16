package main

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// seedDocker upserts a fixed set of containers per host via
// UpsertDockerContainers, which already does ON CONFLICT (id) DO UPDATE and
// prunes any row for that host not in the given list — fully idempotent with
// no delete step needed here. Covers running/healthy, running/unhealthy (the
// explicit edge case), restarting, and exited (both zero and non-zero exit).
func seedDocker(ctx context.Context, db *database.DB) error {
	byHost := map[string][]models.DockerContainer{
		"demo-web-01": {
			container(anchor, "demo-web-01-nginx", "demo-web-01", "nginx", "1.27-alpine", "running", "Up 3 hours", "80/tcp, 443/tcp"),
			container(anchor, "demo-web-01-frontend", "demo-web-01", "ghcr.io/acme/frontend", "v2.4.1", "running", "Up 3 hours", "3000/tcp"),
		},
		"demo-db-01": {
			container(anchor, "demo-db-01-postgres", "demo-db-01", "postgres", "16-alpine", "running", "Up 5 days", "5432/tcp"),
			// The "unhealthy container" edge case — real Docker CLI status format.
			container(anchor, "demo-db-01-redis", "demo-db-01", "redis", "7-alpine", "running", "Up 3 hours (unhealthy)", "6379/tcp"),
		},
		"demo-app-02": {
			container(anchor, "demo-app-02-api", "demo-app-02", "ghcr.io/acme/api-backend", "v1.12.0", "running", "Up 45 minutes", "8000/tcp"),
			container(anchor, "demo-app-02-worker", "demo-app-02", "ghcr.io/acme/worker-queue", "v1.12.0", "restarting", "Restarting (1) 12 seconds ago", ""),
		},
		"demo-worker-01": {
			container(anchor.Add(-6*time.Hour), "demo-worker-01-batch", "demo-worker-01", "ghcr.io/acme/batch-job", "v3.0.2", "exited", "Exited (137) 6 hours ago", ""),
			container(anchor.Add(-72*time.Hour), "demo-worker-01-migrate", "demo-worker-01", "ghcr.io/acme/migrate", "v0.9.0", "exited", "Exited (0) 3 days ago", ""),
		},
	}

	for hostID, containers := range byHost {
		if err := db.UpsertDockerContainers(ctx, hostID, containers); err != nil {
			return err
		}
	}
	return nil
}

func container(created time.Time, id, hostID, image, tag, state, status, ports string) models.DockerContainer {
	return models.DockerContainer{
		ID:          id,
		HostID:      hostID,
		Hostname:    hostID,
		ContainerID: id[:12],
		Name:        id,
		Image:       image,
		ImageTag:    tag,
		ImageID:     "sha256:" + id,
		ImageDigest: "sha256:" + id + "digest",
		State:       state,
		Status:      status,
		Created:     created,
		Ports:       ports,
		Labels:      map[string]string{},
		EnvVars:     map[string]string{},
		Volumes:     []string{},
		Networks:    []string{"bridge"},
		NetRxBytes:  1_200_000,
		NetTxBytes:  800_000,
	}
}

package database

import (
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== APT Status ==========

func (db *DB) UpsertAptStatus(status *models.AptStatus) error {
	_, err := db.conn.Exec(
		`INSERT INTO apt_status (host_id, last_update, last_upgrade, pending_packages, package_list, security_updates, cve_list, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,CAST($7 AS JSONB),NOW())
		 ON CONFLICT (host_id) DO UPDATE SET
			last_update  = GREATEST(EXCLUDED.last_update,  COALESCE(apt_status.last_update,  EXCLUDED.last_update)),
			last_upgrade = GREATEST(EXCLUDED.last_upgrade, COALESCE(apt_status.last_upgrade, EXCLUDED.last_upgrade)),
			pending_packages = EXCLUDED.pending_packages,
			package_list = EXCLUDED.package_list,
			security_updates = EXCLUDED.security_updates,
			cve_list = EXCLUDED.cve_list,
			updated_at = NOW()`,
		status.HostID, status.LastUpdate, status.LastUpgrade, status.PendingPackages, status.PackageList, status.SecurityUpdates, status.CVEList,
	)
	return err
}

func (db *DB) GetAptStatus(hostID string) (*models.AptStatus, error) {
	var s models.AptStatus
	err := db.conn.QueryRow(
		`SELECT id, host_id, last_update, last_upgrade, pending_packages, package_list, security_updates, cve_list, updated_at
		 FROM apt_status WHERE host_id = $1`, hostID,
	).Scan(&s.ID, &s.HostID, &s.LastUpdate, &s.LastUpgrade, &s.PendingPackages, &s.PackageList, &s.SecurityUpdates, &s.CVEList, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) TouchAptLastAction(hostID, command string) error {
	now := time.Now()

	if command == "update" {
		_, err := db.conn.Exec(
			`INSERT INTO apt_status (host_id, last_update, pending_packages, package_list, security_updates, updated_at)
			 VALUES ($1, $2, 0, '[]', 0, NOW())
			 ON CONFLICT (host_id) DO UPDATE SET
				last_update = $2,
				updated_at = NOW()`,
			hostID, now,
		)
		return err
	}

	if command == "upgrade" || command == "dist-upgrade" {
		_, err := db.conn.Exec(
			`INSERT INTO apt_status (host_id, last_upgrade, pending_packages, package_list, security_updates, updated_at)
			 VALUES ($1, $2, 0, '[]', 0, NOW())
			 ON CONFLICT (host_id) DO UPDATE SET
				last_upgrade = $2,
				updated_at = NOW()`,
			hostID, now,
		)
		return err
	}

	return nil
}

// GetAptHistoryWithAgentUpdates returns APT remote commands for a host.
func (db *DB) GetAptHistoryWithAgentUpdates(hostID string, limit int) ([]models.RemoteCommand, error) {
	return db.GetRemoteCommandsByHostAndModule(hostID, "apt", limit)
}

// ========== Tracked Repos ==========

func (db *DB) CreateTrackedRepo(repo *models.TrackedRepo) error {
	return db.conn.QueryRow(
		`INSERT INTO tracked_repos (owner, repo, display_name, docker_image)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		repo.Owner, repo.Repo, repo.DisplayName, repo.DockerImage,
	).Scan(&repo.ID, &repo.CreatedAt)
}

func (db *DB) GetTrackedRepos() ([]models.TrackedRepo, error) {
	rows, err := db.conn.Query(
		`SELECT id, owner, repo, display_name, latest_version, COALESCE(latest_date, NOW()),
		 release_url, docker_image, checked_at, created_at
		 FROM tracked_repos ORDER BY display_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []models.TrackedRepo
	for rows.Next() {
		var r models.TrackedRepo
		if err := rows.Scan(&r.ID, &r.Owner, &r.Repo, &r.DisplayName, &r.LatestVersion,
			&r.LatestDate, &r.ReleaseURL, &r.DockerImage, &r.CheckedAt, &r.CreatedAt); err != nil {
			continue
		}
		repos = append(repos, r)
	}
	return repos, nil
}

func (db *DB) UpdateTrackedRepo(id int64, version, url string, date time.Time) error {
	_, err := db.conn.Exec(
		`UPDATE tracked_repos SET latest_version = $1, release_url = $2, latest_date = $3, checked_at = NOW() WHERE id = $4`,
		version, url, date, id,
	)
	return err
}

func (db *DB) DeleteTrackedRepo(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM tracked_repos WHERE id = $1`, id)
	return err
}

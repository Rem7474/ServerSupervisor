package database

import (
	"database/sql"
	"encoding/json"
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

// GetAptCVESummary returns aggregated CVE severity counts across all hosts.
func (db *DB) GetAptCVESummary() (*models.AptCVESummary, error) {
	var s models.AptCVESummary
	err := db.conn.QueryRow(`
		SELECT
			COUNT(DISTINCT CASE WHEN cve->>'severity' = 'CRITICAL' THEN host_id END),
			COUNT(DISTINCT CASE WHEN cve->>'severity' = 'HIGH'     THEN host_id END),
			COUNT(CASE WHEN cve->>'severity' = 'CRITICAL' THEN 1 END),
			COUNT(CASE WHEN cve->>'severity' = 'HIGH'     THEN 1 END),
			COUNT(CASE WHEN cve->>'severity' = 'MEDIUM'   THEN 1 END),
			COUNT(*)
		FROM apt_status,
			jsonb_array_elements(
				CASE WHEN cve_list IS NOT NULL AND cve_list::text NOT IN ('null','[]','')
					THEN cve_list ELSE '[]'::jsonb END
			) AS cve
	`).Scan(&s.HostsWithCritical, &s.HostsWithHigh, &s.CriticalCount, &s.HighCount, &s.MediumCount, &s.TotalCVECount)
	if err != nil {
		return &models.AptCVESummary{}, nil
	}
	return &s, nil
}

// GetTotalAptPending returns the total number of pending APT packages across all hosts.
func (db *DB) GetTotalAptPending() int {
	var total int
	_ = db.conn.QueryRow(`SELECT COALESCE(SUM(pending_packages), 0) FROM apt_status`).Scan(&total)
	return total
}

// GetAptPendingAll returns a map of host_id → pending_packages for hosts with pending > 0.
func (db *DB) GetAptPendingAll() map[string]int {
	rows, err := db.conn.Query(`SELECT host_id, pending_packages FROM apt_status WHERE pending_packages > 0`)
	if err != nil {
		return map[string]int{}
	}
	defer func() { _ = rows.Close() }()
	result := map[string]int{}
	for rows.Next() {
		var hostID string
		var pending int
		if err := rows.Scan(&hostID, &pending); err == nil {
			result[hostID] = pending
		}
	}
	return result
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
	defer func() { _ = rows.Close() }()

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

// ========== Unattended-Upgrades ==========

func (db *DB) UpsertUUStatus(hostID string, s models.UnattendedUpgradesStatus) error {
	cfgJSON, err := json.Marshal(s.Config)
	if err != nil {
		cfgJSON = []byte("{}")
	}
	_, err = db.conn.Exec(`
		INSERT INTO unattended_upgrades_status
			(host_id, installed, enabled, reboot_required, config, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (host_id) DO UPDATE SET
			installed       = EXCLUDED.installed,
			enabled         = EXCLUDED.enabled,
			reboot_required = EXCLUDED.reboot_required,
			config          = EXCLUDED.config,
			updated_at      = NOW()`,
		hostID, s.Installed, s.Enabled, s.RebootRequired, string(cfgJSON),
	)
	return err
}

// UpdateUULastRun bumps the last_run_at / last_run_packages counters after a new run is stored.
func (db *DB) UpdateUULastRun(hostID string, runAt time.Time, pkgCount int) error {
	_, err := db.conn.Exec(`
		INSERT INTO unattended_upgrades_status (host_id, last_run_at, last_run_packages, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (host_id) DO UPDATE SET
			last_run_at       = EXCLUDED.last_run_at,
			last_run_packages = EXCLUDED.last_run_packages,
			updated_at        = NOW()`,
		hostID, runAt, pkgCount,
	)
	return err
}

// InsertUURunIfNew inserts a run record and returns true if it was actually new.
func (db *DB) InsertUURunIfNew(hostID string, run models.UURun) (bool, error) {
	pkgsJSON, err := json.Marshal(run.Packages)
	if err != nil {
		pkgsJSON = []byte("[]")
	}
	var id int64
	err = db.conn.QueryRow(`
		INSERT INTO unattended_upgrades_runs (host_id, run_at, packages, had_error, log_snippet)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (host_id, run_at) DO NOTHING
		RETURNING id`,
		hostID, run.RunAt, string(pkgsJSON), run.HadError, run.LogSnippet,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil // already existed
	}
	return err == nil, err
}

func (db *DB) GetUUStatus(hostID string) (*models.UnattendedUpgradesDB, error) {
	row := db.conn.QueryRow(`
		SELECT installed, enabled, reboot_required, last_run_at, last_run_packages, config
		FROM unattended_upgrades_status WHERE host_id = $1`, hostID)

	var s models.UnattendedUpgradesDB
	var cfgRaw []byte
	var lastRunAt sql.NullTime
	if err := row.Scan(&s.Installed, &s.Enabled, &s.RebootRequired, &lastRunAt, &s.LastRunPackages, &cfgRaw); err != nil {
		if err == sql.ErrNoRows {
			return &models.UnattendedUpgradesDB{}, nil
		}
		return nil, err
	}
	if lastRunAt.Valid {
		t := lastRunAt.Time
		s.LastRunAt = &t
	}
	if len(cfgRaw) > 0 {
		_ = json.Unmarshal(cfgRaw, &s.Config)
	}
	return &s, nil
}

func (db *DB) GetUURuns(hostID string, limit int) ([]models.UURun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.conn.Query(`
		SELECT run_at, packages, had_error, COALESCE(log_snippet, '')
		FROM unattended_upgrades_runs
		WHERE host_id = $1
		ORDER BY run_at DESC LIMIT $2`, hostID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []models.UURun
	for rows.Next() {
		var r models.UURun
		var pkgsRaw []byte
		if err := rows.Scan(&r.RunAt, &pkgsRaw, &r.HadError, &r.LogSnippet); err != nil {
			continue
		}
		_ = json.Unmarshal(pkgsRaw, &r.Packages)
		if r.Packages == nil {
			r.Packages = []string{}
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func (db *DB) DeleteTrackedRepo(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM tracked_repos WHERE id = $1`, id)
	return err
}

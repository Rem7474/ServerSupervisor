package database

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== Release Trackers ==========

func (db *DB) CreateReleaseTracker(t models.ReleaseTracker) (*models.ReleaseTracker, error) {
	channels := t.NotifyChannels
	if channels == nil {
		channels = []string{}
	}
	var result models.ReleaseTracker
	err := db.conn.QueryRow(
		`INSERT INTO release_trackers
		 (name, provider, repo_owner, repo_name, docker_image, host_id, custom_task_id,
		  notify_channels, notify_on_release, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, name, provider, repo_owner, repo_name, docker_image, host_id, custom_task_id,
		           last_release_tag, last_checked_at, last_triggered_at,
		           notify_channels, notify_on_release, enabled, created_at`,
		t.Name, t.Provider, t.RepoOwner, t.RepoName, t.DockerImage, t.HostID, t.CustomTaskID,
		pq.Array(channels), t.NotifyOnRelease, t.Enabled,
	).Scan(
		&result.ID, &result.Name, &result.Provider, &result.RepoOwner, &result.RepoName,
		&result.DockerImage, &result.HostID, &result.CustomTaskID, &result.LastReleaseTag,
		&result.LastCheckedAt, &result.LastTriggeredAt,
		pq.Array(&result.NotifyChannels), &result.NotifyOnRelease,
		&result.Enabled, &result.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if result.NotifyChannels == nil {
		result.NotifyChannels = []string{}
	}
	return &result, nil
}

func (db *DB) ListReleaseTrackers() ([]models.ReleaseTracker, error) {
	rows, err := db.conn.Query(
		`SELECT t.id, t.name, t.provider, t.repo_owner, t.repo_name, t.docker_image,
		        t.host_id, t.custom_task_id, t.last_release_tag, t.latest_image_digest,
		        t.last_checked_at, t.last_triggered_at, t.last_error,
		        t.notify_channels, t.notify_on_release, t.enabled, t.created_at,
		        COALESCE(h.name, '') AS host_name,
		        le.id, le.tag_name, le.release_url, le.release_name,
		        le.status, le.triggered_at, le.completed_at
		 FROM release_trackers t
		 LEFT JOIN hosts h ON h.id = t.host_id
		 LEFT JOIN LATERAL (
		   SELECT id, tag_name, release_url, release_name, status, triggered_at, completed_at
		   FROM release_tracker_executions
		   WHERE tracker_id = t.id
		   ORDER BY triggered_at DESC LIMIT 1
		 ) le ON TRUE
		 ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.ReleaseTracker
	for rows.Next() {
		var t models.ReleaseTracker
		var leID, leTag, leURL, leName, leStatus sql.NullString
		var leTriggered sql.NullTime
		var leCompleted sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Provider, &t.RepoOwner, &t.RepoName, &t.DockerImage,
			&t.HostID, &t.CustomTaskID, &t.LastReleaseTag, &t.LatestImageDigest,
			&t.LastCheckedAt, &t.LastTriggeredAt, &t.LastError,
			pq.Array(&t.NotifyChannels), &t.NotifyOnRelease, &t.Enabled, &t.CreatedAt,
			&t.HostName,
			&leID, &leTag, &leURL, &leName, &leStatus, &leTriggered, &leCompleted,
		); err != nil {
			return nil, err
		}
		if t.NotifyChannels == nil {
			t.NotifyChannels = []string{}
		}
		if leID.Valid {
			exec := &models.ReleaseTrackerExecution{
				ID:          leID.String,
				TrackerID:   t.ID,
				TagName:     leTag.String,
				ReleaseURL:  leURL.String,
				ReleaseName: leName.String,
				Status:      leStatus.String,
				TriggeredAt: leTriggered.Time,
			}
			if leCompleted.Valid {
				exec.CompletedAt = &leCompleted.Time
			}
			t.LastExecution = exec
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (db *DB) GetReleaseTrackerByID(id string) (*models.ReleaseTracker, error) {
	var t models.ReleaseTracker
	err := db.conn.QueryRow(
		`SELECT t.id, t.name, t.provider, t.repo_owner, t.repo_name, t.docker_image,
		        t.host_id, t.custom_task_id, t.last_release_tag, t.latest_image_digest,
		        t.last_checked_at, t.last_triggered_at, t.last_error,
		        t.notify_channels, t.notify_on_release, t.enabled, t.created_at,
		        COALESCE(h.name, '') AS host_name
		 FROM release_trackers t
		 LEFT JOIN hosts h ON h.id = t.host_id
		 WHERE t.id = $1`, id,
	).Scan(
		&t.ID, &t.Name, &t.Provider, &t.RepoOwner, &t.RepoName, &t.DockerImage,
		&t.HostID, &t.CustomTaskID, &t.LastReleaseTag, &t.LatestImageDigest,
		&t.LastCheckedAt, &t.LastTriggeredAt, &t.LastError,
		pq.Array(&t.NotifyChannels), &t.NotifyOnRelease, &t.Enabled, &t.CreatedAt,
		&t.HostName,
	)
	if err != nil {
		return nil, err
	}
	if t.NotifyChannels == nil {
		t.NotifyChannels = []string{}
	}
	return &t, nil
}

func (db *DB) UpdateReleaseTracker(id string, t models.ReleaseTracker) error {
	channels := t.NotifyChannels
	if channels == nil {
		channels = []string{}
	}
	_, err := db.conn.Exec(
		`UPDATE release_trackers SET
		   name=$1, provider=$2, repo_owner=$3, repo_name=$4, docker_image=$5,
		   host_id=$6, custom_task_id=$7,
		   notify_channels=$8, notify_on_release=$9, enabled=$10
		 WHERE id=$11`,
		t.Name, t.Provider, t.RepoOwner, t.RepoName, t.DockerImage,
		t.HostID, t.CustomTaskID,
		pq.Array(channels), t.NotifyOnRelease, t.Enabled,
		id,
	)
	return err
}

func (db *DB) DeleteReleaseTracker(id string) error {
	_, err := db.conn.Exec(`DELETE FROM release_trackers WHERE id=$1`, id)
	return err
}

// GetEnabledReleaseTrackers returns all enabled trackers for polling.
func (db *DB) GetEnabledReleaseTrackers() ([]models.ReleaseTracker, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, provider, repo_owner, repo_name, docker_image, host_id, custom_task_id,
		        last_release_tag, latest_image_digest, notify_channels, notify_on_release
		 FROM release_trackers WHERE enabled = TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.ReleaseTracker
	for rows.Next() {
		var t models.ReleaseTracker
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Provider, &t.RepoOwner, &t.RepoName, &t.DockerImage,
			&t.HostID, &t.CustomTaskID, &t.LastReleaseTag, &t.LatestImageDigest,
			pq.Array(&t.NotifyChannels), &t.NotifyOnRelease,
		); err != nil {
			return nil, err
		}
		if t.NotifyChannels == nil {
			t.NotifyChannels = []string{}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateReleaseTrackerLastSeen updates the last known tag, check timestamp, and clears any error.
// If triggered=true, also updates last_triggered_at.
func (db *DB) UpdateReleaseTrackerLastSeen(id, newTag string, triggered bool) error {
	now := time.Now()
	if triggered {
		_, err := db.conn.Exec(
			`UPDATE release_trackers SET last_release_tag=$1, last_checked_at=$2, last_triggered_at=$2, last_error='' WHERE id=$3`,
			newTag, now, id)
		return err
	}
	if newTag != "" {
		_, err := db.conn.Exec(
			`UPDATE release_trackers SET last_release_tag=$1, last_checked_at=$2, last_error='' WHERE id=$3`,
			newTag, now, id)
		return err
	}
	_, err := db.conn.Exec(
		`UPDATE release_trackers SET last_checked_at=$1, last_error='' WHERE id=$2`, now, id)
	return err
}

// UpdateReleaseTrackerDigest stores the manifest digest of the latest release image.
func (db *DB) UpdateReleaseTrackerDigest(id, digest string) error {
	_, err := db.conn.Exec(
		`UPDATE release_trackers SET latest_image_digest=$1 WHERE id=$2`, digest, id)
	return err
}

// StoreTrackerTagDigest persists a (tag, digest) pair for historical version lookup.
// Uses ON CONFLICT to update the digest if the tag was already recorded (re-tagged image).
func (db *DB) StoreTrackerTagDigest(trackerID, tag, digest string) error {
	_, err := db.conn.Exec(
		`INSERT INTO release_tracker_tag_digests (tracker_id, tag, digest)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (tracker_id, tag) DO UPDATE SET digest = EXCLUDED.digest`,
		trackerID, tag, digest)
	return err
}

// GetAllTrackerTagDigests returns all stored (trackerID, tag, digest) triples.
// Used by buildVersionComparisons to resolve a container's image digest to a version tag.
func (db *DB) GetAllTrackerTagDigests() (map[string]string, error) {
	rows, err := db.conn.Query(
		`SELECT tracker_id, tag, digest FROM release_tracker_tag_digests`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	m := make(map[string]string)
	for rows.Next() {
		var trackerID, tag, digest string
		if err := rows.Scan(&trackerID, &tag, &digest); err != nil {
			continue
		}
		m[trackerID+"|"+digest] = tag
	}
	return m, nil
}

// UpdateReleaseTrackerError stores an error from the last check attempt.
func (db *DB) UpdateReleaseTrackerError(id, errMsg string) error {
	now := time.Now()
	_, err := db.conn.Exec(
		`UPDATE release_trackers SET last_checked_at=$1, last_error=$2 WHERE id=$3`, now, errMsg, id)
	return err
}

// GetRunningExecutionForReleaseTracker returns true if a pending/running execution exists.
func (db *DB) GetRunningExecutionForReleaseTracker(trackerID string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM release_tracker_executions
		 WHERE tracker_id=$1 AND status IN ('pending','running')`, trackerID,
	).Scan(&count)
	return count > 0, err
}

func (db *DB) CreateReleaseTrackerExecution(e models.ReleaseTrackerExecution) (*models.ReleaseTrackerExecution, error) {
	var result models.ReleaseTrackerExecution
	err := db.conn.QueryRow(
		`INSERT INTO release_tracker_executions
		 (tracker_id, tag_name, release_url, release_name, status)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, tracker_id, tag_name, release_url, release_name, status, triggered_at`,
		e.TrackerID, e.TagName, e.ReleaseURL, e.ReleaseName, e.Status,
	).Scan(
		&result.ID, &result.TrackerID, &result.TagName, &result.ReleaseURL,
		&result.ReleaseName, &result.Status, &result.TriggeredAt,
	)
	return &result, err
}

func (db *DB) UpdateReleaseTrackerExecutionCommandID(execID, commandID string) error {
	_, err := db.conn.Exec(
		`UPDATE release_tracker_executions SET command_id=$1 WHERE id=$2`, commandID, execID)
	return err
}

func (db *DB) UpdateReleaseTrackerExecutionStatus(id, status string, completedAt *time.Time) error {
	_, err := db.conn.Exec(
		`UPDATE release_tracker_executions SET status=$1, completed_at=$2 WHERE id=$3`,
		status, completedAt, id)
	return err
}

// UpdateReleaseTrackerExecutionByCommandID updates execution status when a command completes.
// Returns tracker info for notification dispatch.
func (db *DB) UpdateReleaseTrackerExecutionByCommandID(commandID, status string) (
	trackerID string, notifyOnRelease bool, channels []string, err error,
) {
	now := time.Now()
	err = db.conn.QueryRow(
		`UPDATE release_tracker_executions SET status=$1, completed_at=$2
		 WHERE command_id=$3
		 RETURNING tracker_id`,
		status, now, commandID,
	).Scan(&trackerID)
	if err != nil {
		return
	}
	err = db.conn.QueryRow(
		`SELECT notify_on_release, notify_channels FROM release_trackers WHERE id=$1`, trackerID,
	).Scan(&notifyOnRelease, pq.Array(&channels))
	return
}

func (db *DB) ListReleaseTrackerExecutions(trackerID string, limit int) ([]models.ReleaseTrackerExecution, error) {
	rows, err := db.conn.Query(
		`SELECT id, tracker_id, command_id, tag_name, release_url, release_name,
		        status, triggered_at, completed_at
		 FROM release_tracker_executions
		 WHERE tracker_id=$1 ORDER BY triggered_at DESC LIMIT $2`,
		trackerID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.ReleaseTrackerExecution
	for rows.Next() {
		var e models.ReleaseTrackerExecution
		var cmdID sql.NullString
		var completed sql.NullTime
		if err := rows.Scan(
			&e.ID, &e.TrackerID, &cmdID,
			&e.TagName, &e.ReleaseURL, &e.ReleaseName,
			&e.Status, &e.TriggeredAt, &completed,
		); err != nil {
			return nil, err
		}
		if cmdID.Valid {
			e.CommandID = &cmdID.String
		}
		if completed.Valid {
			e.CompletedAt = &completed.Time
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

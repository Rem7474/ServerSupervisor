package database

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== Ambient Docker image version cache ==========
//
// Backing store of internal/services/dockerversions — see
// migrations/093_docker_image_versions.sql for why it exists.

// normalizeImageTag applies the same defaulting the Docker CLI does, so
// "nginx" and "nginx:latest" resolve to a single cache row.
func normalizeImageTag(tag string) string {
	if strings.TrimSpace(tag) == "" {
		return "latest"
	}
	return tag
}

// ListDistinctDockerImages returns every distinct (image, tag) currently
// reported by any host's containers — the work list of the refresh job.
func (db *DB) ListDistinctDockerImages(ctx context.Context) ([]models.DockerImageRef, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT DISTINCT image, COALESCE(NULLIF(image_tag, ''), 'latest')
		 FROM docker_containers
		 WHERE image <> ''`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.DockerImageRef, 0)
	for rows.Next() {
		var ref models.DockerImageRef
		if err := rows.Scan(&ref.Image, &ref.ImageTag); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// ListDockerTrackerImageRefs returns the (image, tag) pairs named by docker
// release trackers. Included in the refresh work list so a tracker whose image
// happens not to be running anywhere right now still gets a fresh digest —
// that used to be the poller's own registry call.
func (db *DB) ListDockerTrackerImageRefs(ctx context.Context) ([]models.DockerImageRef, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT DISTINCT docker_image, COALESCE(NULLIF(docker_tag, ''), 'latest')
		 FROM release_trackers
		 WHERE tracker_type = 'docker' AND enabled = TRUE AND docker_image <> ''`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.DockerImageRef, 0)
	for rows.Next() {
		var ref models.DockerImageRef
		if err := rows.Scan(&ref.Image, &ref.ImageTag); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

const dockerImageVersionCols = `image, image_tag, registry, latest_digest, latest_tag,
	COALESCE(registry_credentials_id::text, ''), checked_at, last_error`

func scanDockerImageVersion(scan func(dest ...any) error) (models.DockerImageVersion, error) {
	var v models.DockerImageVersion
	var checkedAt sql.NullTime
	err := scan(&v.Image, &v.ImageTag, &v.Registry, &v.LatestDigest, &v.LatestTag,
		&v.RegistryCredentialsID, &checkedAt, &v.LastError)
	if checkedAt.Valid {
		t := checkedAt.Time
		v.CheckedAt = &t
	}
	return v, err
}

// ListDockerImageVersions returns the whole cache (one row per image:tag).
// Read on every docker/host/dashboard WS snapshot rebuild, so it stays a
// single set-based query rather than a per-container lookup.
func (db *DB) ListDockerImageVersions(ctx context.Context) ([]models.DockerImageVersion, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT `+dockerImageVersionCols+` FROM docker_image_versions`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.DockerImageVersion, 0)
	for rows.Next() {
		v, err := scanDockerImageVersion(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetDockerImageVersion returns one cache row, or nil when the image has never
// been checked (no row yet) — not an error, the caller decides whether to
// refresh on demand.
func (db *DB) GetDockerImageVersion(ctx context.Context, image, tag string) (*models.DockerImageVersion, error) {
	v, err := scanDockerImageVersion(db.conn.QueryRowContext(ctx,
		`SELECT `+dockerImageVersionCols+` FROM docker_image_versions WHERE image=$1 AND image_tag=$2`,
		image, normalizeImageTag(tag)).Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpsertDockerImageVersion records a successful check. Always clears last_error.
func (db *DB) UpsertDockerImageVersion(ctx context.Context, v models.DockerImageVersion) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO docker_image_versions
		   (image, image_tag, registry, latest_digest, latest_tag, registry_credentials_id, checked_at, last_error)
		 VALUES ($1,$2,$3,$4,$5,$6,NOW(),'')
		 ON CONFLICT (image, image_tag) DO UPDATE SET
		   registry = EXCLUDED.registry,
		   latest_digest = EXCLUDED.latest_digest,
		   latest_tag = EXCLUDED.latest_tag,
		   registry_credentials_id = EXCLUDED.registry_credentials_id,
		   checked_at = NOW(),
		   last_error = '',
		   updated_at = NOW()`,
		v.Image, normalizeImageTag(v.ImageTag), v.Registry, v.LatestDigest, v.LatestTag,
		nullStrTracker(v.RegistryCredentialsID))
	return err
}

// SetDockerImageVersionError records a failed check without discarding the last
// known good digest — a transient registry outage degrades the row to "stale",
// it doesn't blank out every badge that depends on it.
func (db *DB) SetDockerImageVersionError(ctx context.Context, image, tag, registry, errMsg string) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO docker_image_versions (image, image_tag, registry, checked_at, last_error)
		 VALUES ($1,$2,$3,NOW(),$4)
		 ON CONFLICT (image, image_tag) DO UPDATE SET
		   registry = EXCLUDED.registry,
		   checked_at = NOW(),
		   last_error = EXCLUDED.last_error,
		   updated_at = NOW()`,
		image, normalizeImageTag(tag), registry, errMsg)
	return err
}

// PruneDockerImageVersions drops cache rows for images that are neither running
// anywhere nor named by an enabled docker tracker, so the table follows the
// fleet instead of growing forever.
func (db *DB) PruneDockerImageVersions(ctx context.Context, keep []models.DockerImageRef) (int64, error) {
	if len(keep) == 0 {
		return 0, nil
	}
	images := make([]string, 0, len(keep))
	tags := make([]string, 0, len(keep))
	for _, ref := range keep {
		images = append(images, ref.Image)
		tags = append(tags, normalizeImageTag(ref.ImageTag))
	}
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM docker_image_versions
		 WHERE (image, image_tag) NOT IN (
		   SELECT * FROM UNNEST($1::text[], $2::text[])
		 )`, pq.Array(images), pq.Array(tags))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

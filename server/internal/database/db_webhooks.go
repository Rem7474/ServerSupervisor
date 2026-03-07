package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// generateWebhookSecret creates a random 32-byte hex secret.
func generateWebhookSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== Git Webhooks ==========

func (db *DB) CreateGitWebhook(w models.GitWebhook) (*models.GitWebhook, error) {
	secret := generateWebhookSecret()
	channels := w.NotifyChannels
	if channels == nil {
		channels = []string{}
	}
	var result models.GitWebhook
	err := db.conn.QueryRow(
		`INSERT INTO git_webhooks
		 (name, secret, provider, repo_filter, branch_filter, event_filter,
		  host_id, custom_task_id, notify_channels, notify_on_success, notify_on_failure, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING id, name, secret, provider, repo_filter, branch_filter, event_filter,
		           host_id, custom_task_id, notify_channels, notify_on_success, notify_on_failure,
		           enabled, last_triggered_at, created_at`,
		w.Name, secret, w.Provider, w.RepoFilter, w.BranchFilter, w.EventFilter,
		w.HostID, w.CustomTaskID, pq.Array(channels),
		w.NotifyOnSuccess, w.NotifyOnFailure, w.Enabled,
	).Scan(
		&result.ID, &result.Name, &result.Secret, &result.Provider,
		&result.RepoFilter, &result.BranchFilter, &result.EventFilter,
		&result.HostID, &result.CustomTaskID, pq.Array(&result.NotifyChannels),
		&result.NotifyOnSuccess, &result.NotifyOnFailure,
		&result.Enabled, &result.LastTriggeredAt, &result.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (db *DB) ListGitWebhooks() ([]models.GitWebhook, error) {
	rows, err := db.conn.Query(
		`SELECT w.id, w.name, w.provider, w.repo_filter, w.branch_filter, w.event_filter,
		        w.host_id, w.custom_task_id, w.notify_channels, w.notify_on_success, w.notify_on_failure,
		        w.enabled, w.last_triggered_at, w.created_at,
		        COALESCE(h.name, '') AS host_name,
		        le.id, le.provider, le.repo_name, le.branch, le.commit_sha,
		        le.commit_message, le.pusher, le.status, le.triggered_at, le.completed_at
		 FROM git_webhooks w
		 LEFT JOIN hosts h ON h.id = w.host_id
		 LEFT JOIN LATERAL (
		     SELECT id, provider, repo_name, branch, commit_sha, commit_message, pusher, status, triggered_at, completed_at
		     FROM git_webhook_executions
		     WHERE webhook_id = w.id
		     ORDER BY triggered_at DESC LIMIT 1
		 ) le ON TRUE
		 ORDER BY w.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []models.GitWebhook
	for rows.Next() {
		var wh models.GitWebhook
		var leID, leProvider, leRepo, leBranch, leSHA, leMsg, lePusher, leStatus sql.NullString
		var leTriggered sql.NullTime
		var leCompleted sql.NullTime
		if err := rows.Scan(
			&wh.ID, &wh.Name, &wh.Provider, &wh.RepoFilter, &wh.BranchFilter, &wh.EventFilter,
			&wh.HostID, &wh.CustomTaskID, pq.Array(&wh.NotifyChannels),
			&wh.NotifyOnSuccess, &wh.NotifyOnFailure,
			&wh.Enabled, &wh.LastTriggeredAt, &wh.CreatedAt,
			&wh.HostName,
			&leID, &leProvider, &leRepo, &leBranch, &leSHA, &leMsg, &lePusher, &leStatus, &leTriggered, &leCompleted,
		); err != nil {
			return nil, err
		}
		if wh.NotifyChannels == nil {
			wh.NotifyChannels = []string{}
		}
		if leID.Valid {
			exec := &models.GitWebhookExecution{
				ID:            leID.String,
				WebhookID:     wh.ID,
				Provider:      leProvider.String,
				RepoName:      leRepo.String,
				Branch:        leBranch.String,
				CommitSHA:     leSHA.String,
				CommitMessage: leMsg.String,
				Pusher:        lePusher.String,
				Status:        leStatus.String,
				TriggeredAt:   leTriggered.Time,
			}
			if leCompleted.Valid {
				exec.CompletedAt = &leCompleted.Time
			}
			wh.LastExecution = exec
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, rows.Err()
}

func (db *DB) GetGitWebhookByID(id string) (*models.GitWebhook, error) {
	var wh models.GitWebhook
	err := db.conn.QueryRow(
		`SELECT w.id, w.name, w.secret, w.provider, w.repo_filter, w.branch_filter, w.event_filter,
		        w.host_id, w.custom_task_id, w.notify_channels, w.notify_on_success, w.notify_on_failure,
		        w.enabled, w.last_triggered_at, w.created_at,
		        COALESCE(h.name, '') AS host_name
		 FROM git_webhooks w
		 LEFT JOIN hosts h ON h.id = w.host_id
		 WHERE w.id = $1`, id,
	).Scan(
		&wh.ID, &wh.Name, &wh.Secret, &wh.Provider, &wh.RepoFilter, &wh.BranchFilter, &wh.EventFilter,
		&wh.HostID, &wh.CustomTaskID, pq.Array(&wh.NotifyChannels),
		&wh.NotifyOnSuccess, &wh.NotifyOnFailure,
		&wh.Enabled, &wh.LastTriggeredAt, &wh.CreatedAt, &wh.HostName,
	)
	if err != nil {
		return nil, err
	}
	if wh.NotifyChannels == nil {
		wh.NotifyChannels = []string{}
	}
	return &wh, nil
}

// GetGitWebhookForReceive returns minimal webhook data (including secret) for the public receiver endpoint.
func (db *DB) GetGitWebhookForReceive(id string) (*models.GitWebhook, error) {
	var wh models.GitWebhook
	err := db.conn.QueryRow(
		`SELECT id, name, secret, provider, repo_filter, branch_filter, event_filter,
		        host_id, custom_task_id, notify_channels, notify_on_success, notify_on_failure, enabled
		 FROM git_webhooks WHERE id = $1`, id,
	).Scan(
		&wh.ID, &wh.Name, &wh.Secret, &wh.Provider, &wh.RepoFilter, &wh.BranchFilter, &wh.EventFilter,
		&wh.HostID, &wh.CustomTaskID, pq.Array(&wh.NotifyChannels),
		&wh.NotifyOnSuccess, &wh.NotifyOnFailure, &wh.Enabled,
	)
	if err != nil {
		return nil, err
	}
	if wh.NotifyChannels == nil {
		wh.NotifyChannels = []string{}
	}
	return &wh, nil
}

func (db *DB) UpdateGitWebhook(id string, w models.GitWebhook) error {
	channels := w.NotifyChannels
	if channels == nil {
		channels = []string{}
	}
	_, err := db.conn.Exec(
		`UPDATE git_webhooks SET
		 name=$1, provider=$2, repo_filter=$3, branch_filter=$4, event_filter=$5,
		 host_id=$6, custom_task_id=$7, notify_channels=$8,
		 notify_on_success=$9, notify_on_failure=$10, enabled=$11
		 WHERE id=$12`,
		w.Name, w.Provider, w.RepoFilter, w.BranchFilter, w.EventFilter,
		w.HostID, w.CustomTaskID, pq.Array(channels),
		w.NotifyOnSuccess, w.NotifyOnFailure, w.Enabled, id,
	)
	return err
}

func (db *DB) DeleteGitWebhook(id string) error {
	_, err := db.conn.Exec(`DELETE FROM git_webhooks WHERE id=$1`, id)
	return err
}

func (db *DB) RegenerateWebhookSecret(id string) (string, error) {
	secret := generateWebhookSecret()
	_, err := db.conn.Exec(`UPDATE git_webhooks SET secret=$1 WHERE id=$2`, secret, id)
	if err != nil {
		return "", err
	}
	return secret, nil
}

func (db *DB) UpdateGitWebhookLastTriggered(id string) error {
	_, err := db.conn.Exec(`UPDATE git_webhooks SET last_triggered_at=NOW() WHERE id=$1`, id)
	return err
}

// ========== Git Webhook Executions ==========

func (db *DB) CreateWebhookExecution(e models.GitWebhookExecution) (*models.GitWebhookExecution, error) {
	var result models.GitWebhookExecution
	err := db.conn.QueryRow(
		`INSERT INTO git_webhook_executions
		 (webhook_id, command_id, provider, repo_name, branch, commit_sha, commit_message, pusher, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, webhook_id, command_id, provider, repo_name, branch, commit_sha, commit_message, pusher, status, triggered_at, completed_at`,
		e.WebhookID, e.CommandID, e.Provider, e.RepoName, e.Branch,
		e.CommitSHA, e.CommitMessage, e.Pusher, e.Status,
	).Scan(
		&result.ID, &result.WebhookID, &result.CommandID,
		&result.Provider, &result.RepoName, &result.Branch,
		&result.CommitSHA, &result.CommitMessage, &result.Pusher,
		&result.Status, &result.TriggeredAt, &result.CompletedAt,
	)
	return &result, err
}

func (db *DB) UpdateWebhookExecutionCommandID(execID, commandID string) error {
	_, err := db.conn.Exec(
		`UPDATE git_webhook_executions SET command_id=$1 WHERE id=$2`,
		commandID, execID,
	)
	return err
}

func (db *DB) UpdateWebhookExecutionStatus(id, status string, completedAt *time.Time) error {
	_, err := db.conn.Exec(
		`UPDATE git_webhook_executions SET status=$1, completed_at=$2 WHERE id=$3`,
		status, completedAt, id,
	)
	return err
}

// UpdateWebhookExecutionByCommandID updates the execution linked to a command when it finishes.
// Returns (webhookID, notifyOnSuccess, notifyOnFailure, notifyChannels) for notification dispatch.
func (db *DB) UpdateWebhookExecutionByCommandID(commandID, status string) (webhookID string, notifyOnSuccess bool, notifyOnFailure bool, channels []string, err error) {
	now := time.Now()
	var chArr pq.StringArray
	err = db.conn.QueryRow(
		`UPDATE git_webhook_executions e
		 SET status=$1, completed_at=$2
		 FROM git_webhooks w
		 WHERE e.command_id=$3 AND w.id=e.webhook_id
		 RETURNING w.id, w.notify_on_success, w.notify_on_failure, w.notify_channels`,
		status, now, commandID,
	).Scan(&webhookID, &notifyOnSuccess, &notifyOnFailure, &chArr)
	if err != nil {
		return "", false, false, nil, err
	}
	channels = []string(chArr)
	return
}

// GetRunningExecutionForWebhook returns true if there is a pending/running execution for the given webhook.
func (db *DB) GetRunningExecutionForWebhook(webhookID string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM git_webhook_executions
		 WHERE webhook_id=$1 AND status IN ('pending','running')`,
		webhookID,
	).Scan(&count)
	return count > 0, err
}

func (db *DB) ListWebhookExecutions(webhookID string, limit int) ([]models.GitWebhookExecution, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(
		`SELECT id, webhook_id, command_id, provider, repo_name, branch, commit_sha, commit_message, pusher, status, triggered_at, completed_at
		 FROM git_webhook_executions
		 WHERE webhook_id=$1
		 ORDER BY triggered_at DESC LIMIT $2`,
		webhookID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var execs []models.GitWebhookExecution
	for rows.Next() {
		var e models.GitWebhookExecution
		if err := rows.Scan(
			&e.ID, &e.WebhookID, &e.CommandID,
			&e.Provider, &e.RepoName, &e.Branch,
			&e.CommitSHA, &e.CommitMessage, &e.Pusher,
			&e.Status, &e.TriggeredAt, &e.CompletedAt,
		); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

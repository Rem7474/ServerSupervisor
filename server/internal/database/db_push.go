package database

import (
	"database/sql"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Push Subscriptions ==========

// SavePushSubscription inserts or updates a Web Push subscription for a user.
// The endpoint is the unique key; updating it refreshes keys and re-links to the current user.
func (db *DB) SavePushSubscription(username, endpoint, p256dh, authKey, userAgent string) error {
	_, err := db.conn.Exec(`
		INSERT INTO push_subscriptions (username, endpoint, p256dh_key, auth_key, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE SET
			username   = $1,
			p256dh_key = $3,
			auth_key   = $4,
			user_agent = $5
	`, username, endpoint, p256dh, authKey, userAgent)
	return err
}

// DeletePushSubscription removes a subscription by its push service endpoint URL.
// Called when the client unsubscribes or the push service returns 410 Gone.
func (db *DB) DeletePushSubscription(endpoint string) error {
	_, err := db.conn.Exec(`DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

// GetAllPushSubscriptions returns all stored Web Push subscriptions across all users.
// Used by the alert engine to fan-out notifications to every registered device.
func (db *DB) GetAllPushSubscriptions() ([]models.PushSubscription, error) {
	rows, err := db.conn.Query(`
		SELECT id, username, endpoint, p256dh_key, auth_key, user_agent, created_at
		FROM push_subscriptions
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subs []models.PushSubscription
	for rows.Next() {
		var s models.PushSubscription
		if err := rows.Scan(&s.ID, &s.Username, &s.Endpoint, &s.P256DHKey, &s.AuthKey, &s.UserAgent, &s.CreatedAt); err != nil {
			continue
		}
		subs = append(subs, s)
	}
	return subs, nil
}

// ========== Notification Read-At (cross-device sync) ==========

// UpsertNotificationReadAt sets the "read up to" timestamp for a user.
// All notifications triggered before readAt are considered read on every device.
func (db *DB) UpsertNotificationReadAt(username string, readAt time.Time) error {
	_, err := db.conn.Exec(`
		INSERT INTO notification_read_at (username, read_at) VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE SET read_at = $2
	`, username, readAt)
	return err
}

// GetNotificationReadAt returns the stored read-at timestamp for a user, or nil if never set.
func (db *DB) GetNotificationReadAt(username string) (*time.Time, error) {
	var readAt time.Time
	err := db.conn.QueryRow(`SELECT read_at FROM notification_read_at WHERE username = $1`, username).Scan(&readAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &readAt, nil
}

// GetPushSubscriptionsByRole returns subscriptions for users with a specific role.
func (db *DB) GetPushSubscriptionsByRole(role string) ([]models.PushSubscription, error) {
	rows, err := db.conn.Query(`
SELECT ps.id, ps.username, ps.endpoint, ps.p256dh_key, ps.auth_key, ps.user_agent, ps.created_at
FROM push_subscriptions ps
JOIN users u ON u.username = ps.username
WHERE u.role = $1
`, role)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subs []models.PushSubscription
	for rows.Next() {
		var s models.PushSubscription
		if err := rows.Scan(&s.ID, &s.Username, &s.Endpoint, &s.P256DHKey, &s.AuthKey, &s.UserAgent, &s.CreatedAt); err != nil {
			continue
		}
		subs = append(subs, s)
	}
	return subs, nil
}

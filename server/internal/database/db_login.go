package database

import (
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Login Events ==========

func (db *DB) CreateLoginEvent(username, ipAddress, userAgent string, success bool) error {
	_, err := db.conn.Exec(
		`INSERT INTO login_events (username, ip_address, user_agent, success) VALUES ($1, $2, $3, $4)`,
		username, ipAddress, userAgent, success,
	)
	return err
}

func (db *DB) GetLoginEventsByUser(username string, limit int) ([]models.LoginEvent, error) {
	rows, err := db.conn.Query(
		`SELECT id, username, ip_address, success, user_agent, created_at
		 FROM login_events WHERE username = $1 ORDER BY created_at DESC LIMIT $2`,
		username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.LoginEvent
	for rows.Next() {
		var e models.LoginEvent
		if err := rows.Scan(&e.ID, &e.Username, &e.IPAddress, &e.Success, &e.UserAgent, &e.CreatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func (db *DB) CountRecentFailedLogins(ipAddress string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM login_events WHERE ip_address = $1 AND success = FALSE AND created_at >= $2`,
		ipAddress, since,
	).Scan(&count)
	return count, err
}

// GetLoginStats returns aggregate login counts for the given time window.
func (db *DB) GetLoginStats(since time.Time) (*models.LoginStats, error) {
	var stats models.LoginStats
	err := db.conn.QueryRow(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE NOT success) AS failures,
			COUNT(DISTINCT ip_address) AS unique_ips
		FROM login_events WHERE created_at > $1
	`, since).Scan(&stats.Total, &stats.Failures, &stats.UniqueIPs)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (db *DB) GetAllLoginEvents(limit, offset int) ([]models.LoginEvent, error) {
	rows, err := db.conn.Query(
		`SELECT id, username, ip_address, success, user_agent, created_at
		 FROM login_events ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.LoginEvent
	for rows.Next() {
		var e models.LoginEvent
		if err := rows.Scan(&e.ID, &e.Username, &e.IPAddress, &e.Success, &e.UserAgent, &e.CreatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func (db *DB) CountLoginEvents() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM login_events`).Scan(&count)
	return count, err
}

// GetTopFailedIPs returns the IPs with the most failed login attempts in the given window.
func (db *DB) GetTopFailedIPs(since time.Time, limit int) ([]models.IPFailCount, error) {
	rows, err := db.conn.Query(`
		SELECT ip_address, COUNT(*) AS fail_count
		FROM login_events
		WHERE success = false AND created_at > $1
		GROUP BY ip_address
		ORDER BY fail_count DESC
		LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.IPFailCount
	for rows.Next() {
		var item models.IPFailCount
		if err := rows.Scan(&item.IPAddress, &item.FailCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

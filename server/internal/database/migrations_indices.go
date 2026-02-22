package database

import (
	"log"
)

// CreateIndices creates performance indices for frequently queried columns
func (db *DB) CreateIndices() error {
	indices := []string{
		// Alert incidents: common query (rule_id, host_id, resolved_at)
		`CREATE INDEX IF NOT EXISTS idx_alert_incidents_rule_host_resolved 
		 ON alert_incidents(rule_id, host_id, resolved_at)`,

		// Metrics: timestamp queries for history
		`CREATE INDEX IF NOT EXISTS idx_system_metrics_host_timestamp 
		 ON system_metrics(host_id, timestamp DESC)`,

		// Metrics aggregates: for historical data
		`CREATE INDEX IF NOT EXISTS idx_metrics_aggregates_host_timestamp 
		 ON metrics_aggregates(host_id, aggregation_type, timestamp DESC)`,

		// Audit logs: by host and timestamp
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_host_timestamp 
		 ON audit_logs(host_id, created_at DESC)`,

		// Audit logs: by user
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_username 
		 ON audit_logs(username, created_at DESC)`,

		// APT commands: by host
		`CREATE INDEX IF NOT EXISTS idx_apt_commands_host 
		 ON apt_commands(host_id, created_at DESC)`,

		// Docker containers: by host
		`CREATE INDEX IF NOT EXISTS idx_docker_containers_host 
		 ON docker_containers(host_id)`,

		// Hosts: by status for quick online/offline checks
		`CREATE INDEX IF NOT EXISTS idx_hosts_status 
		 ON hosts(status)`,
	}

	for _, idx := range indices {
		if _, err := db.conn.Exec(idx); err != nil {
			// Log but don't fail - indices might already exist
			log.Printf("Warning: Index creation failed (may already exist): %v", err)
		}
	}

	return nil
}

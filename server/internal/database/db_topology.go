package database

import (
	"database/sql"
	"encoding/json"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Network Topology ==========

func (db *DB) GetNetworkTopologyConfig() (*models.NetworkTopologyConfig, error) {
	var cfg models.NetworkTopologyConfig
	var excludedPortsJSON []byte
	err := db.conn.QueryRow(
		`SELECT id, root_label, root_ip, excluded_ports, service_map, host_overrides, manual_services,
		        COALESCE(authelia_label, 'Authelia'), COALESCE(authelia_ip, ''),
		        COALESCE(internet_label, 'Internet'), COALESCE(internet_ip, ''),
		        updated_at
		 FROM network_topology_config LIMIT 1`,
	).Scan(&cfg.ID, &cfg.RootLabel, &cfg.RootIP, &excludedPortsJSON, &cfg.ServiceMap,
		&cfg.HostOverrides, &cfg.ManualServices,
		&cfg.AutheliaLabel, &cfg.AutheliaIP, &cfg.InternetLabel, &cfg.InternetIP,
		&cfg.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return &models.NetworkTopologyConfig{
				RootLabel:      "Infrastructure",
				ExcludedPorts:  []int{},
				ServiceMap:     "{}",
				HostOverrides:  "{}",
				ManualServices: "[]",
				AutheliaLabel:  "Authelia",
				InternetLabel:  "Internet",
			}, nil
		}
		return nil, err
	}
	if len(excludedPortsJSON) > 0 {
		json.Unmarshal(excludedPortsJSON, &cfg.ExcludedPorts)
	}
	return &cfg, nil
}

func (db *DB) SaveNetworkTopologyConfig(cfg *models.NetworkTopologyConfig) error {
	excludedPortsJSON, _ := json.Marshal(cfg.ExcludedPorts)
	_, err := db.conn.Exec(
		`INSERT INTO network_topology_config (id, root_label, root_ip, excluded_ports, service_map, host_overrides, manual_services,
		        authelia_label, authelia_ip, internet_label, internet_ip, updated_at)
		 VALUES (1, $1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, $10, NOW())
		 ON CONFLICT(id) DO UPDATE SET
		   root_label = EXCLUDED.root_label,
		   root_ip = EXCLUDED.root_ip,
		   excluded_ports = EXCLUDED.excluded_ports,
		   service_map = EXCLUDED.service_map,
		   host_overrides = EXCLUDED.host_overrides,
		   manual_services = EXCLUDED.manual_services,
		   authelia_label = EXCLUDED.authelia_label,
		   authelia_ip = EXCLUDED.authelia_ip,
		   internet_label = EXCLUDED.internet_label,
		   internet_ip = EXCLUDED.internet_ip,
		   updated_at = NOW()`,
		cfg.RootLabel, cfg.RootIP, excludedPortsJSON,
		cfg.ServiceMap, cfg.HostOverrides, cfg.ManualServices,
		cfg.AutheliaLabel, cfg.AutheliaIP, cfg.InternetLabel, cfg.InternetIP,
	)
	return err
}

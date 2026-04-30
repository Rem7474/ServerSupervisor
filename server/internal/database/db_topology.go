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
		        COALESCE(node_positions::text, '{}'),
		        COALESCE(root_host_id, ''), COALESCE(authelia_host_id, ''),
		        COALESCE(root_port_id, ''), COALESCE(authelia_port_id, ''),
		        updated_at
		 FROM network_topology_config LIMIT 1`,
	).Scan(&cfg.ID, &cfg.RootLabel, &cfg.RootIP, &excludedPortsJSON, &cfg.ServiceMap,
		&cfg.HostOverrides, &cfg.ManualServices,
		&cfg.AutheliaLabel, &cfg.AutheliaIP, &cfg.InternetLabel, &cfg.InternetIP,
		&cfg.NodePositions,
		&cfg.RootHostID, &cfg.AutheliaHostID,
		&cfg.RootPortID, &cfg.AutheliaPortID,
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
				NodePositions:  "{}",
				RootPortID:     "",
				AutheliaPortID: "",
			}, nil
		}
		return nil, err
	}
	if len(excludedPortsJSON) > 0 {
		_ = json.Unmarshal(excludedPortsJSON, &cfg.ExcludedPorts)
	}
	return &cfg, nil
}

func (db *DB) SaveNetworkTopologyConfig(cfg *models.NetworkTopologyConfig) error {
	excludedPortsJSON, _ := json.Marshal(cfg.ExcludedPorts)
	nodePositions := cfg.NodePositions
	if nodePositions == "" {
		nodePositions = "{}"
	}
	_, err := db.conn.Exec(
		`INSERT INTO network_topology_config (id, root_label, root_ip, excluded_ports, service_map, host_overrides, manual_services,
		        authelia_label, authelia_ip, internet_label, internet_ip, node_positions,
		        root_host_id, authelia_host_id, root_port_id, authelia_port_id, updated_at)
		 VALUES (1, $1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $13, $14, $15, NOW())
		 ON CONFLICT(id) DO UPDATE SET
		   root_label       = EXCLUDED.root_label,
		   root_ip          = EXCLUDED.root_ip,
		   excluded_ports   = EXCLUDED.excluded_ports,
		   service_map      = EXCLUDED.service_map,
		   host_overrides   = EXCLUDED.host_overrides,
		   manual_services  = EXCLUDED.manual_services,
		   authelia_label   = EXCLUDED.authelia_label,
		   authelia_ip      = EXCLUDED.authelia_ip,
		   internet_label   = EXCLUDED.internet_label,
		   internet_ip      = EXCLUDED.internet_ip,
		   node_positions   = EXCLUDED.node_positions,
		   root_host_id     = EXCLUDED.root_host_id,
		   authelia_host_id = EXCLUDED.authelia_host_id,
		   root_port_id     = EXCLUDED.root_port_id,
		   authelia_port_id = EXCLUDED.authelia_port_id,
		   updated_at       = NOW()`,
		cfg.RootLabel, cfg.RootIP, excludedPortsJSON,
		cfg.ServiceMap, cfg.HostOverrides, cfg.ManualServices,
		cfg.AutheliaLabel, cfg.AutheliaIP, cfg.InternetLabel, cfg.InternetIP,
		nodePositions,
		cfg.RootHostID, cfg.AutheliaHostID,
		cfg.RootPortID, cfg.AutheliaPortID,
	)
	return err
}

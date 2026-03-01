package database

import (
	"encoding/json"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== Docker Containers ==========

func (db *DB) UpsertDockerContainers(hostID string, containers []models.DockerContainer) error {
	ids := make([]string, 0, len(containers))
	for _, c := range containers {
		labelsJSON, _ := json.Marshal(c.Labels)
		envVarsJSON, _ := json.Marshal(c.EnvVars)
		volumesJSON, _ := json.Marshal(c.Volumes)
		networksJSON, _ := json.Marshal(c.Networks)
		_, err := db.conn.Exec(`
			INSERT INTO docker_containers (id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels, env_vars, volumes, networks, net_rx_bytes, net_tx_bytes, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,NOW())
			ON CONFLICT (id) DO UPDATE SET
				name         = EXCLUDED.name,
				image        = EXCLUDED.image,
				image_tag    = EXCLUDED.image_tag,
				image_id     = EXCLUDED.image_id,
				state        = EXCLUDED.state,
				status       = EXCLUDED.status,
				created      = EXCLUDED.created,
				ports        = EXCLUDED.ports,
				labels       = EXCLUDED.labels,
				env_vars     = EXCLUDED.env_vars,
				volumes      = EXCLUDED.volumes,
				networks     = EXCLUDED.networks,
				net_rx_bytes = EXCLUDED.net_rx_bytes,
				net_tx_bytes = EXCLUDED.net_tx_bytes,
				updated_at   = NOW()`,
			c.ID, hostID, c.ContainerID, c.Name, c.Image, c.ImageTag, c.ImageID, c.State, c.Status, c.Created, c.Ports,
			string(labelsJSON), string(envVarsJSON), string(volumesJSON), string(networksJSON),
			c.NetRxBytes, c.NetTxBytes,
		)
		if err != nil {
			return err
		}
		ids = append(ids, c.ID)
	}

	if len(ids) > 0 {
		_, err := db.conn.Exec(
			`DELETE FROM docker_containers WHERE host_id = $1 AND NOT (id = ANY($2))`,
			hostID, pq.Array(ids),
		)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM docker_containers WHERE host_id = $1`, hostID)
	return err
}

func (db *DB) GetDockerContainers(hostID string) ([]models.DockerContainer, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels,
		 COALESCE(env_vars::text, '{}'), COALESCE(volumes::text, '[]'), COALESCE(networks::text, '[]'),
		 COALESCE(net_rx_bytes, 0), COALESCE(net_tx_bytes, 0), updated_at
		 FROM docker_containers WHERE host_id = $1 ORDER BY name`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []models.DockerContainer
	for rows.Next() {
		var c models.DockerContainer
		var labelsJSON, envVarsJSON, volumesJSON, networksJSON string
		if err := rows.Scan(&c.ID, &c.HostID, &c.ContainerID, &c.Name, &c.Image, &c.ImageTag, &c.ImageID,
			&c.State, &c.Status, &c.Created, &c.Ports, &labelsJSON, &envVarsJSON, &volumesJSON, &networksJSON,
			&c.NetRxBytes, &c.NetTxBytes, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(labelsJSON), &c.Labels)
		json.Unmarshal([]byte(envVarsJSON), &c.EnvVars)
		json.Unmarshal([]byte(volumesJSON), &c.Volumes)
		json.Unmarshal([]byte(networksJSON), &c.Networks)
		containers = append(containers, c)
	}
	return containers, nil
}

func (db *DB) GetAllDockerContainers() ([]models.DockerContainer, error) {
	rows, err := db.conn.Query(
		`SELECT dc.id, dc.host_id, h.hostname, dc.container_id, dc.name, dc.image, dc.image_tag, dc.image_id,
		 dc.state, dc.status, dc.created, dc.ports, dc.labels,
		 COALESCE(dc.env_vars::text, '{}'), COALESCE(dc.volumes::text, '[]'), COALESCE(dc.networks::text, '[]'),
		 COALESCE(dc.net_rx_bytes, 0), COALESCE(dc.net_tx_bytes, 0), dc.updated_at
		 FROM docker_containers dc
		 JOIN hosts h ON dc.host_id = h.id
		 ORDER BY h.hostname, dc.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []models.DockerContainer
	for rows.Next() {
		var c models.DockerContainer
		var labelsJSON, envVarsJSON, volumesJSON, networksJSON string
		if err := rows.Scan(&c.ID, &c.HostID, &c.Hostname, &c.ContainerID, &c.Name, &c.Image, &c.ImageTag, &c.ImageID,
			&c.State, &c.Status, &c.Created, &c.Ports, &labelsJSON, &envVarsJSON, &volumesJSON, &networksJSON,
			&c.NetRxBytes, &c.NetTxBytes, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(labelsJSON), &c.Labels)
		json.Unmarshal([]byte(envVarsJSON), &c.EnvVars)
		json.Unmarshal([]byte(volumesJSON), &c.Volumes)
		json.Unmarshal([]byte(networksJSON), &c.Networks)
		containers = append(containers, c)
	}
	return containers, nil
}

// ========== Docker Networks ==========

func (db *DB) UpsertDockerNetworks(hostID string, networks []models.DockerNetwork) error {
	if len(networks) == 0 {
		return nil
	}
	for _, n := range networks {
		containerIDsJSON, _ := json.Marshal(n.ContainerIDs)
		_, err := db.conn.Exec(
			`INSERT INTO docker_networks (id, host_id, network_id, name, driver, scope, container_ids, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			 ON CONFLICT(id) DO UPDATE SET
			 container_ids = $7, updated_at = NOW()`,
			n.ID, hostID, n.NetworkID, n.Name, n.Driver, n.Scope, containerIDsJSON,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetDockerNetworksByHost(hostID string) ([]models.DockerNetwork, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, network_id, name, driver, scope, container_ids, updated_at
		 FROM docker_networks WHERE host_id = $1`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var networks []models.DockerNetwork
	for rows.Next() {
		var n models.DockerNetwork
		var containerIDsJSON []byte
		if err := rows.Scan(&n.ID, &n.HostID, &n.NetworkID, &n.Name, &n.Driver, &n.Scope, &containerIDsJSON, &n.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal(containerIDsJSON, &n.ContainerIDs)
		networks = append(networks, n)
	}
	return networks, nil
}

func (db *DB) GetAllDockerNetworks() ([]models.DockerNetwork, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, network_id, name, driver, scope, container_ids, updated_at
		 FROM docker_networks ORDER BY host_id, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var networks []models.DockerNetwork
	for rows.Next() {
		var n models.DockerNetwork
		var containerIDsJSON []byte
		if err := rows.Scan(&n.ID, &n.HostID, &n.NetworkID, &n.Name, &n.Driver, &n.Scope, &containerIDsJSON, &n.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal(containerIDsJSON, &n.ContainerIDs)
		networks = append(networks, n)
	}
	return networks, nil
}

// ========== Container Envs ==========
// Env vars are stored in docker_containers.env_vars (no separate table).

func (db *DB) GetAllContainerEnvs() ([]models.ContainerEnv, error) {
	rows, err := db.conn.Query(
		`SELECT name AS container_name, COALESCE(env_vars::text, '{}')
		 FROM docker_containers
		 WHERE env_vars IS NOT NULL AND env_vars != '{}'::jsonb
		 ORDER BY host_id, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []models.ContainerEnv
	for rows.Next() {
		var env models.ContainerEnv
		var envVarsJSON string
		if err := rows.Scan(&env.ContainerName, &envVarsJSON); err != nil {
			continue
		}
		json.Unmarshal([]byte(envVarsJSON), &env.EnvVars)
		envs = append(envs, env)
	}
	return envs, nil
}

// Docker domain types — mirror server/internal/models/docker.go JSON shapes.

export interface DockerContainer {
  id: string
  host_id: string
  hostname: string
  container_id: string
  name: string
  image: string
  image_tag: string
  image_id: string
  image_digest: string
  state: string // running, stopped, paused, …
  status: string
  created: string
  ports: string
  labels?: Record<string, string>
  env_vars?: Record<string, string>
  volumes?: string[]
  networks?: string[]
  net_rx_bytes: number
  net_tx_bytes: number
  updated_at: string
}

export interface ComposeProject {
  id: string
  host_id: string
  hostname: string
  name: string
  working_dir: string
  config_file: string
  services?: string[]
  raw_config: string
  updated_at: string
}

export interface DockerNetwork {
  id: string
  host_id: string
  network_id: string
  name: string
  driver: string // bridge, overlay, host, none
  scope: string // local, swarm
  container_ids?: string[]
  updated_at: string
}

/** Paginated envelope returned by GET /api/v1/docker/containers. */
export interface DockerContainersPage {
  containers: DockerContainer[]
  total: number
  limit: number
  offset: number
}

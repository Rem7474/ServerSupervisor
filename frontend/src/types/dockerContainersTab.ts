import type { VersionComparisonStatus } from './docker'

export interface Container {
  id: string
  name: string
  hostname?: string
  host_id: string
  image: string
  image_tag?: string
  state: string
  labels?: Record<string, string>
  env_vars?: Record<string, string>
  volumes?: string[]
  networks?: string[]
  net_rx_bytes?: number
  net_tx_bytes?: number
  [key: string]: unknown
}

export interface VersionComparison {
  tracker_id?: string
  host_id: string
  docker_image: string
  image_tag?: string
  running_version?: string
  latest_version?: string
  status?: VersionComparisonStatus
  is_up_to_date?: boolean
  update_confirmed?: boolean
  last_error?: string
}

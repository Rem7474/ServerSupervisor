// Proxmox domain types — mirror server/internal/models/proxmox.go JSON shapes.
// Keep in sync with the Go structs; timestamps are ISO strings on the wire.

export interface ProxmoxConnection {
  id: string
  name: string
  api_url: string
  token_id: string
  token_secret?: string
  insecure_skip_verify?: boolean
  enabled?: boolean
  poll_interval_sec?: number
  last_error?: string
  last_error_at?: string | null
  last_success_at?: string | null
  created_at?: string
  updated_at?: string
  // Computed (joined, not stored)
  node_count?: number
  guest_count?: number
}

export interface ProxmoxGuest {
  id: string
  connection_id: string
  node_name: string
  guest_type: 'vm' | 'lxc'
  vmid: number
  name: string
  status: string
  cpu_alloc: number
  cpu_usage: number
  mem_alloc: number
  mem_usage: number
  disk_alloc: number
  tags: string
  uptime: number
  last_seen_at: string
}

export interface ProxmoxStorage {
  id: string
  connection_id: string
  node_name: string
  storage_name: string
  storage_type: string
  total: number
  used: number
  avail: number
  enabled: boolean
  active: boolean
  shared: boolean
  last_seen_at: string
}

export interface ProxmoxTask {
  id: string
  connection_id: string
  node_name: string
  upid: string
  task_type: string
  status: string
  user_name: string
  start_time?: string | null
  end_time?: string | null
  exit_status: string
  object_id: string
  last_seen_at: string
}

export interface ProxmoxDisk {
  id: string
  connection_id: string
  node_name: string
  dev_path: string
  model: string
  serial: string
  size_bytes: number
  disk_type: 'ssd' | 'hdd' | 'nvme' | 'unknown'
  health: 'PASSED' | 'FAILED' | 'UNKNOWN'
  wearout: number
  last_seen_at: string
}

export interface ProxmoxNode {
  id: string
  connection_id: string
  node_name: string
  cpu_temp_source_host_id?: string
  cpu_temp_source_host_name?: string
  fan_rpm_source_host_id?: string
  fan_rpm_source_host_name?: string
  status: string
  cpu_count: number
  cpu_usage: number
  mem_total: number
  mem_used: number
  uptime: number
  pve_version: string
  cluster_name: string
  ip_address: string
  last_seen_at: string
  pending_updates: number
  security_updates: number
  last_update_check_at?: string | null
  vm_count?: number
  lxc_count?: number
  // Detail view (populated on single-node fetch)
  guests?: ProxmoxGuest[]
  storages?: ProxmoxStorage[]
  disks?: ProxmoxDisk[]
  tasks?: ProxmoxTask[]
}

export interface ProxmoxBackupJob {
  id: string
  connection_id: string
  job_id: string
  enabled: boolean
  schedule: string
  storage: string
  mode: string
  compress: string
  vmids: string
  mail_to: string
  last_seen_at: string
}

export interface ProxmoxBackupRun {
  id: string
  connection_id: string
  node_name: string
  vmid: number
  task_upid: string
  status: string
  start_time?: string | null
  end_time?: string | null
  exit_status: string
  last_seen_at: string
  guest_name?: string
}

export interface ProxmoxSummary {
  connection_count: number
  node_count: number
  vm_count: number
  lxc_count: number
  storage_total: number
  storage_used: number
  nodes_down: number
  storage_near_full: number
  storage_offline: number
  recent_failed_tasks: number
}

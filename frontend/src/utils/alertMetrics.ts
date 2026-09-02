import { i18n } from '../i18n'

export interface AlertMetricMeta {
  label: string
  unit: string
  icon: string
  badgeClass: string
  category: 'host' | 'proxmox' | 'synthetic' | 'docker'
}

type AlertMetricStaticMeta = Omit<AlertMetricMeta, 'label'>

export const ALERT_METRICS: Record<string, AlertMetricStaticMeta> = {
  cpu: {
    unit: '%',
    icon: '⚡',
    badgeClass: 'bg-red-lt text-red',
    category: 'host',
  },
  cpu_temperature: {
    unit: '°C',
    icon: '🌡',
    badgeClass: 'bg-orange-lt text-orange',
    category: 'host',
  },
  memory: {
    unit: '%',
    icon: '🧠',
    badgeClass: 'bg-blue-lt text-blue',
    category: 'host',
  },
  disk: {
    unit: '%',
    icon: '💾',
    badgeClass: 'bg-yellow-lt text-yellow',
    category: 'host',
  },
  load: {
    unit: '',
    icon: '📈',
    badgeClass: 'bg-purple-lt text-purple',
    category: 'host',
  },
  bandwidth_vs_rolling_avg: {
    unit: '%',
    icon: '📡',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'host',
  },
  heartbeat_timeout: {
    unit: 's',
    icon: '🫀',
    badgeClass: 'bg-orange-lt text-orange',
    category: 'host',
  },
  status_offline: {
    unit: '',
    icon: '🔌',
    badgeClass: 'bg-red-lt text-red',
    category: 'host',
  },
  disk_smart_status: {
    unit: '',
    icon: '🛡',
    badgeClass: 'bg-yellow-lt text-yellow',
    category: 'host',
  },
  disk_temperature: {
    unit: '°C',
    icon: '🌡',
    badgeClass: 'bg-orange-lt text-orange',
    category: 'host',
  },
  proxmox_storage_percent: {
    unit: '%',
    icon: '🖥',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_cpu_percent: {
    unit: '%',
    icon: '🧠',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_memory_percent: {
    unit: '%',
    icon: '📊',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_cpu_temperature: {
    unit: '°C',
    icon: '🌡',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_fan_rpm: {
    unit: ' RPM',
    icon: '🌀',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_guest_cpu_percent: {
    unit: '%',
    icon: '🧠',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_guest_memory_percent: {
    unit: '%',
    icon: '📊',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_pending_updates: {
    unit: '',
    icon: '🔄',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_recent_failed_tasks_24h: {
    unit: '',
    icon: '🕒',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_auth_failures_recent: {
    unit: '',
    icon: '🔒',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_disk_failed_count: {
    unit: '',
    icon: '💥',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_disk_min_wearout_percent: {
    unit: '%',
    icon: '🛠',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  docker_container_state: {
    unit: '',
    icon: '🐳',
    badgeClass: 'bg-blue-lt text-blue',
    category: 'docker',
  },
  docker_compose_degraded_services: {
    unit: '',
    icon: '🐳',
    badgeClass: 'bg-blue-lt text-blue',
    category: 'docker',
  },
  uptime_down_count: {
    unit: '',
    icon: '🚨',
    badgeClass: 'bg-red-lt text-red',
    category: 'synthetic',
  },
  ssl_min_days_remaining: {
    unit: 'j',
    icon: '🔐',
    badgeClass: 'bg-yellow-lt text-yellow',
    category: 'synthetic',
  },
}

export const ALERT_METRIC_ORDER = [
  'cpu',
  'cpu_temperature',
  'memory',
  'disk',
  'load',
  'bandwidth_vs_rolling_avg',
  'disk_smart_status',
  'disk_temperature',
  'heartbeat_timeout',
  'status_offline',
  'proxmox_storage_percent',
  'proxmox_node_cpu_percent',
  'proxmox_node_memory_percent',
  'proxmox_node_cpu_temperature',
  'proxmox_node_fan_rpm',
  'proxmox_guest_cpu_percent',
  'proxmox_guest_memory_percent',
  'proxmox_node_pending_updates',
  'proxmox_recent_failed_tasks_24h',
  'proxmox_auth_failures_recent',
  'proxmox_disk_failed_count',
  'proxmox_disk_min_wearout_percent',
  'docker_container_state',
  'docker_compose_degraded_services',
  'uptime_down_count',
  'ssl_min_days_remaining',
]

function metricLabelFor(metric: string): string {
  const key = `alerts.metricLabels.${metric}`
  return i18n.global.te(key) ? i18n.global.t(key) : metric
}

export function getAlertMetricMeta(metric: string): AlertMetricMeta {
  const meta = ALERT_METRICS[metric]
  const label = metricLabelFor(metric)
  if (!meta) {
    return {
      label,
      unit: '',
      icon: '📊',
      badgeClass: 'bg-secondary-lt text-secondary',
      category: 'host',
    }
  }
  return { ...meta, label }
}

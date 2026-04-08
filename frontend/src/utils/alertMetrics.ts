export interface AlertMetricMeta {
  label: string
  unit: string
  icon: string
  badgeClass: string
  category: 'host' | 'proxmox'
}

export const ALERT_METRICS: Record<string, AlertMetricMeta> = {
  cpu: {
    label: 'CPU',
    unit: '%',
    icon: '\u26a1',
    badgeClass: 'bg-red-lt text-red',
    category: 'host',
  },
  cpu_temperature: {
    label: 'Temp. CPU',
    unit: '\u00b0C',
    icon: '\ud83c\udf21',
    badgeClass: 'bg-orange-lt text-orange',
    category: 'host',
  },
  memory: {
    label: 'RAM',
    unit: '%',
    icon: '\ud83e\udde0',
    badgeClass: 'bg-blue-lt text-blue',
    category: 'host',
  },
  disk: {
    label: 'Disque',
    unit: '%',
    icon: '\ud83d\udcbe',
    badgeClass: 'bg-yellow-lt text-yellow',
    category: 'host',
  },
  load: {
    label: 'Load avg',
    unit: '',
    icon: '\ud83d\udcc8',
    badgeClass: 'bg-purple-lt text-purple',
    category: 'host',
  },
  heartbeat_timeout: {
    label: 'Heartbeat',
    unit: 's',
    icon: '\ud83e\dec0',
    badgeClass: 'bg-orange-lt text-orange',
    category: 'host',
  },
  status_offline: {
    label: 'Hote hors ligne',
    unit: '',
    icon: '\ud83d\udd0c',
    badgeClass: 'bg-red-lt text-red',
    category: 'host',
  },
  disk_smart_status: {
    label: 'SMART disque',
    unit: '',
    icon: '\ud83d\udee1',
    badgeClass: 'bg-yellow-lt text-yellow',
    category: 'host',
  },
  disk_temperature: {
    label: 'Temp. disque',
    unit: '\u00b0C',
    icon: '\ud83c\udf21',
    badgeClass: 'bg-orange-lt text-orange',
    category: 'host',
  },
  proxmox_storage_percent: {
    label: 'Proxmox stockage',
    unit: '%',
    icon: '\ud83d\udda5',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_cpu_percent: {
    label: 'Proxmox CPU noeud',
    unit: '%',
    icon: '\ud83e\udde0',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_memory_percent: {
    label: 'Proxmox RAM noeud',
    unit: '%',
    icon: '\ud83d\udcca',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_cpu_temperature: {
    label: 'Proxmox temp. CPU noeud',
    unit: '\u00b0C',
    icon: '\ud83c\udf21',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_fan_rpm: {
    label: 'Proxmox RPM ventilateurs noeud',
    unit: ' RPM',
    icon: '\ud83c\udf00',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_guest_cpu_percent: {
    label: 'CPU VM/LXC Proxmox',
    unit: '%',
    icon: '\ud83e\udde0',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_guest_memory_percent: {
    label: 'RAM VM/LXC Proxmox',
    unit: '%',
    icon: '\ud83d\udcca',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_node_pending_updates: {
    label: 'Paquets APT en attente',
    unit: '',
    icon: '\ud83d\udd04',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_recent_failed_tasks_24h: {
    label: 'Tâches Proxmox échouées (24h)',
    unit: '',
    icon: '\ud83d\udd52',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_disk_failed_count: {
    label: 'Disques physiques en échec',
    unit: '',
    icon: '\ud83d\udca5',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
  proxmox_disk_min_wearout_percent: {
    label: 'Usure disque min',
    unit: '%',
    icon: '\ud83d\udee0',
    badgeClass: 'bg-cyan-lt text-cyan',
    category: 'proxmox',
  },
}

export const ALERT_METRIC_ORDER = [
  'cpu',
  'cpu_temperature',
  'memory',
  'disk',
  'load',
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
  'proxmox_disk_failed_count',
  'proxmox_disk_min_wearout_percent',
]

export function getAlertMetricMeta(metric: string): AlertMetricMeta {
  return ALERT_METRICS[metric] || {
    label: metric,
    unit: '',
    icon: '\ud83d\udcca',
    badgeClass: 'bg-secondary-lt text-secondary',
    category: 'host',
  }
}

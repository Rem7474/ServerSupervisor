export interface AlertMetricMeta {
  label: string
  unit: string
  icon: string
  badgeClass: string
  category: 'host' | 'proxmox' | 'web'
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
  npm_requests: {
    label: 'NPM requetes',
    unit: 'req',
    icon: '\ud83c\udf10',
    badgeClass: 'bg-azure-lt text-azure',
    category: 'web',
  },
  npm_traffic_bytes: {
    label: 'NPM trafic',
    unit: 'B',
    icon: '\ud83d\udce6',
    badgeClass: 'bg-azure-lt text-azure',
    category: 'web',
  },
  npm_5xx_errors: {
    label: 'NPM erreurs 5xx',
    unit: 'err',
    icon: '\ud83d\udea8',
    badgeClass: 'bg-red-lt text-red',
    category: 'web',
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
  'npm_requests',
  'npm_traffic_bytes',
  'npm_5xx_errors',
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
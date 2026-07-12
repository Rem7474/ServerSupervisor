/**
 * Quick-start templates for the alert rule wizard. Applying one pre-fills the
 * metric/thresholds/name of AlertRuleModal's form — every field stays
 * editable afterwards (host filter, thresholds, notification channels), this
 * only removes the "start from a blank field" tax for the handful of rules
 * almost everyone wants (host down, CPU high, disk full, SSL expiring).
 */
export interface AlertRulePreset {
  key: string
  label: string
  icon: string
  metric: string
  operator: string
  thresholdWarn: number
  thresholdCrit: number
  duration: number
}

export const ALERT_RULE_PRESETS: AlertRulePreset[] = [
  {
    key: 'host-offline',
    label: 'Hôte hors ligne',
    icon: '🔌',
    metric: 'status_offline',
    operator: '>',
    thresholdWarn: 0.5,
    thresholdCrit: 0.5,
    duration: 0,
  },
  {
    key: 'cpu-high',
    label: 'CPU élevé',
    icon: '⚡',
    metric: 'cpu',
    operator: '>',
    thresholdWarn: 70,
    thresholdCrit: 85,
    duration: 300,
  },
  {
    key: 'disk-almost-full',
    label: 'Disque presque plein',
    icon: '💾',
    metric: 'disk',
    operator: '>',
    thresholdWarn: 80,
    thresholdCrit: 90,
    duration: 300,
  },
  {
    key: 'ssl-expiring',
    label: 'Certificat SSL bientôt expiré',
    icon: '🔐',
    metric: 'ssl_min_days_remaining',
    operator: '<',
    thresholdWarn: 30,
    thresholdCrit: 7,
    duration: 0,
  },
]

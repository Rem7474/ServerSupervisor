import { api } from './client'

export interface AlertRule {
  id?: number
  name?: string
  enabled?: boolean
  source_type?: 'agent' | 'proxmox'
  host_id?: string
  proxmox_scope?: {
    scope_mode?: string
    connection_id?: string
    node_id?: string
    storage_id?: string
    guest_id?: string
    disk_id?: string
  }
  metric?: string
  operator?: string
  threshold_warn?: number
  threshold_crit?: number
  threshold_clear_warn?: number
  threshold_clear_crit?: number
  duration?: number
  actions?: {
    channels?: string[]
    smtp_to?: string
    ntfy_topic?: string
    cooldown?: number
    command_trigger?: unknown
  }
}

export const alertsApi = {
  getAgentAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/agent'),
  getProxmoxAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/proxmox'),
  getSyntheticAlertRuleCapabilities: () => api.get('/v1/alert-rules/capabilities/synthetic'),
  getHostCapabilities: (hostId: string) => api.get(`/v1/hosts/${hostId}/capabilities`),
  getAlertRules: () => api.get('/v1/alert-rules'),
  createAlertRule: (payload: AlertRule) => api.post('/v1/alert-rules', payload),
  updateAlertRule: (id: number, payload: AlertRule) => api.patch(`/v1/alert-rules/${id}`, payload),
  deleteAlertRule: (id: number) => api.delete(`/v1/alert-rules/${id}`),
  testAlertRule: (payload: AlertRule) => api.post('/v1/alert-rules/test', payload),
  downloadAlertRuleTestLogs: (payload: AlertRule) =>
    api.post('/v1/alert-rules/test/logs', payload, { responseType: 'blob' }),
}

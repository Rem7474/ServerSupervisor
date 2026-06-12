// Barrel for the API client. The single default export is assembled from the
// per-domain modules so existing call sites (`api.getHosts()`, …) keep working
// unchanged. Add new endpoints to the relevant domain module, not here.
import { authApi } from './auth'
import { hostsApi } from './hosts'
import { dockerApi } from './docker'
import { aptApi } from './apt'
import { networkApi } from './network'
import { auditApi } from './audit'
import { usersApi } from './users'
import { alertsApi } from './alerts'
import { notificationsApi } from './notifications'
import { tasksApi } from './tasks'
import { trackersApi } from './trackers'
import { uptimeApi } from './uptime'
import { sslApi } from './ssl'
import { webhooksApi } from './webhooks'
import { settingsApi } from './settings'
import { proxmoxApi } from './proxmox'
import { npmApi } from './npm'

// Re-export shared helpers/types so `import api, { getApiErrorMessage } from '../api'`
// and type imports keep resolving.
export { getApiErrorMessage, isApiAbort } from './client'
export type { JsonObject } from './client'
export type { AlertRule } from './alerts'
export type { ProxmoxConnection } from './proxmox'

export default {
  ...authApi,
  ...hostsApi,
  ...dockerApi,
  ...aptApi,
  ...networkApi,
  ...auditApi,
  ...usersApi,
  ...alertsApi,
  ...notificationsApi,
  ...tasksApi,
  ...trackersApi,
  ...uptimeApi,
  ...sslApi,
  ...webhooksApi,
  ...settingsApi,
  ...proxmoxApi,
  ...npmApi,
}

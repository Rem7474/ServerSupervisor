import { api } from './client'
import type { NotificationItem } from '../types/generated'

export interface NotificationFilter {
  limit?: number
  severity?: 'warn' | 'crit' | ''
  type?: 'alert_incident' | 'release_tracker' | ''
  status?: 'active' | 'resolved' | ''
}

export const notificationsApi = {
  // Notifications
  getNotifications: (filter?: NotificationFilter, signal?: AbortSignal) =>
    api.get<{ notifications: NotificationItem[], total: number, read_at: string | null }>('/v1/notifications', {
      params: filter ? Object.fromEntries(Object.entries(filter).filter(([, v]) => v !== '' && v !== undefined)) : undefined,
      signal,
    }),
  markNotificationsRead: () => api.post('/v1/notifications/mark-read'),

  // Push (Web Push / VAPID)
  getPushVapidPublicKey: () => api.get('/v1/push/vapid-public-key'),
  subscribePush: (subscription: PushSubscriptionJSON) => api.post('/v1/push/subscribe', subscription),
  unsubscribePush: (endpoint: string) => api.delete('/v1/push/subscribe', { data: { endpoint } }),
}

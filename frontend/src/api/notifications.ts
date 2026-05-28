import { api } from './client'

export const notificationsApi = {
  // Notifications
  getNotifications: () => api.get('/v1/notifications'),
  markNotificationsRead: () => api.post('/v1/notifications/mark-read'),

  // Push (Web Push / VAPID)
  getPushVapidPublicKey: () => api.get('/v1/push/vapid-public-key'),
  subscribePush: (subscription: PushSubscriptionJSON) => api.post('/v1/push/subscribe', subscription),
  unsubscribePush: (endpoint: string) => api.delete('/v1/push/subscribe', { data: { endpoint } }),
}

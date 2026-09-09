import { api } from './client'
import type { AttentionItem, WSDashboardSnapshot } from '../types/generated'

export const dashboardApi = {
  getAttention: () => api.get<{ items: AttentionItem[] }>('/v1/dashboard/attention'),
  /** Serves the same payload as the first WS frame, from the shared cache.
   *  Lets the dashboard render without waiting for the WebSocket upgrade. */
  getDashboardInit: () => api.get<WSDashboardSnapshot>('/v1/dashboard/init'),
}

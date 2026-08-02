import { api } from './client'
import type { AttentionItem } from '../types/generated'

export const dashboardApi = {
  getAttention: () => api.get<{ items: AttentionItem[] }>('/v1/dashboard/attention'),
}

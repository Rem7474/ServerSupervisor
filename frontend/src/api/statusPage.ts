import { api } from './client'
import type { StatusPageConfig, StatusPageConfigRequest, PublicStatusPage } from '../types/uptime'

export const statusPageApi = {
  getStatusPageConfig: () => api.get<StatusPageConfig>('/v1/status-page'),
  updateStatusPageConfig: (payload: StatusPageConfigRequest) => api.put<StatusPageConfig>('/v1/status-page', payload),
  /** Unauthenticated, read-only public view; 404 while the page is disabled. */
  getPublicStatus: () => api.get<PublicStatusPage>('/public/status'),
}

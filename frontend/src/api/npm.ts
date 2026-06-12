import { api } from './client'
import type { NPMConnection, NPMConnectionRequest, NPMProxyHost, NPMProxyHostEnriched, NPMProxyHostPreview, NPMProxyHostUpdateRequest } from '../types/npm'

export const npmApi = {
  listConnections: () =>
    api.get<{ connections: NPMConnection[] }>('/v1/npm/connections'),

  createConnection: (payload: NPMConnectionRequest) =>
    api.post<NPMConnection>('/v1/npm/connections', payload),

  updateConnection: (id: string, payload: Partial<NPMConnectionRequest>) =>
    api.put<NPMConnection>(`/v1/npm/connections/${id}`, payload),

  deleteConnection: (id: string) =>
    api.delete(`/v1/npm/connections/${id}`),

  testConnection: (payload: { api_url: string; identity: string; secret: string }) =>
    api.post<{ success: boolean; error?: string }>('/v1/npm/connections/test', payload),

  previewProxyHosts: (id: string) =>
    api.get<{ proxy_hosts: NPMProxyHostPreview[] }>(`/v1/npm/connections/${id}/preview`),

  importSelected: (id: string, npmIds: number[]) =>
    api.post<{ imported: number }>(`/v1/npm/connections/${id}/import`, { npm_ids: npmIds }),

  listProxyHosts: (id: string) =>
    api.get<{ proxy_hosts: NPMProxyHost[] }>(`/v1/npm/connections/${id}/proxy-hosts`),

  refreshNow: (id: string) =>
    api.post(`/v1/npm/connections/${id}/refresh-now`),

  listAllProxyHosts: () =>
    api.get<{ proxy_hosts: NPMProxyHostEnriched[] }>('/v1/npm/proxy-hosts'),

  updateProxyHost: (id: string, payload: NPMProxyHostUpdateRequest) =>
    api.patch<NPMProxyHostEnriched>(`/v1/npm/proxy-hosts/${id}`, payload),
}

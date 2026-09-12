import { api } from './client'
import type {
  ConfigSummary,
  UpdateConfigKeyResponse,
  UpdateConfigBulkResponse,
} from '../types/config'

export const configApi = {
  getConfig: (reveal = false, signal?: AbortSignal) =>
    api.get<ConfigSummary>('/v1/config', {
      params: reveal ? { reveal: 'true' } : undefined,
      signal,
    }),

  updateParam: (key: string, value: string) =>
    api.put<UpdateConfigKeyResponse>(`/v1/config/${encodeURIComponent(key)}`, {
      value,
    }),

  resetParam: (key: string) =>
    api.delete<UpdateConfigKeyResponse>(`/v1/config/${encodeURIComponent(key)}`),

  updateBulk: (settings: Record<string, string>) =>
    api.put<UpdateConfigBulkResponse>('/v1/config', {
      settings,
    }),
}

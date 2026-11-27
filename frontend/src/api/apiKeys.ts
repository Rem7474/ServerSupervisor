import { api } from './client'
import type { APIKey, APIKeyCreated, APIKeyCreateRequest } from '../types/generated'

// Personal API keys (Authorization: Bearer ssk_…) for third-party clients.
export const apiKeysApi = {
  listMyApiKeys: (signal?: AbortSignal) => api.get<APIKey[]>('/v1/auth/api-keys', { signal }),
  createApiKey: (body: APIKeyCreateRequest) => api.post<APIKeyCreated>('/v1/auth/api-keys', body),
  revokeApiKey: (id: number) => api.delete(`/v1/auth/api-keys/${id}`),
}

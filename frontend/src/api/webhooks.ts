import { api, type JsonObject } from './client'

export const webhooksApi = {
  getGitWebhooks: () => api.get('/v1/webhooks/git'),
  getGitWebhook: (id: string) => api.get(`/v1/webhooks/git/${id}`),
  createGitWebhook: (payload: JsonObject) => api.post('/v1/webhooks/git', payload),
  updateGitWebhook: (id: string, payload: JsonObject) =>
    api.put(`/v1/webhooks/git/${id}`, payload),
  deleteGitWebhook: (id: string) => api.delete(`/v1/webhooks/git/${id}`),
  regenerateWebhookSecret: (id: string) => api.post(`/v1/webhooks/git/${id}/regenerate-secret`),
  getWebhookExecutions: (id: string, limit?: number) =>
    api.get(`/v1/webhooks/git/${id}/executions`, { params: { limit: limit ?? 50 } }),
}

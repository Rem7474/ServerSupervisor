import { api } from './client'
import type { GitWebhook, GitWebhookExecution, GitWebhookRequest } from '../types/webhook'

export const webhooksApi = {
  getGitWebhooks: () => api.get<{ webhooks: GitWebhook[] }>('/v1/webhooks/git'),
  getGitWebhook: (id: string, signal?: AbortSignal) =>
    api.get<{ webhook: GitWebhook, executions: GitWebhookExecution[] }>(`/v1/webhooks/git/${id}`, { signal }),
  createGitWebhook: (payload: Partial<GitWebhookRequest>) => api.post('/v1/webhooks/git', payload),
  updateGitWebhook: (id: string, payload: Partial<GitWebhookRequest>) =>
    api.put(`/v1/webhooks/git/${id}`, payload),
  deleteGitWebhook: (id: string) => api.delete(`/v1/webhooks/git/${id}`),
  regenerateWebhookSecret: (id: string) => api.post<{ secret: string }>(`/v1/webhooks/git/${id}/regenerate-secret`),
  getWebhookExecutions: (id: string, limit?: number, signal?: AbortSignal) =>
    api.get<{ executions: GitWebhookExecution[] }>(`/v1/webhooks/git/${id}/executions`, { params: { limit: limit ?? 50 }, signal }),
}

import { api } from './client'
import type { Runbook, RunbookCreate, RunbookUpdate, RunbookExecution } from '../types/generated'

export const runbooksApi = {
  getRunbooks: () => api.get<Runbook[]>('/v1/runbooks'),
  getRunbook: (id: string) => api.get<Runbook>(`/v1/runbooks/${id}`),
  createRunbook: (payload: RunbookCreate) => api.post<Runbook>('/v1/runbooks', payload),
  updateRunbook: (id: string, payload: RunbookUpdate) => api.patch(`/v1/runbooks/${id}`, payload),
  deleteRunbook: (id: string) => api.delete(`/v1/runbooks/${id}`),
  runRunbook: (id: string) => api.post<RunbookExecution>(`/v1/runbooks/${id}/run`),
  getRunbookExecutions: (id: string, limit?: number) =>
    api.get<RunbookExecution[]>(`/v1/runbooks/${id}/executions`, { params: { limit: limit ?? 20 } }),
  getRunbookExecution: (id: string, executionId: string) =>
    api.get<RunbookExecution>(`/v1/runbooks/${id}/executions/${executionId}`),
}

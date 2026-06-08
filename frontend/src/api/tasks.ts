import { api } from './client'
import type { ScheduledTask, ScheduledTaskWithHost, CustomTaskSummary, ScheduledTaskRequest } from '../types/task'
import type { ComposeProject } from '../types/docker'

export const tasksApi = {
  getAllScheduledTasks: () => api.get<ScheduledTaskWithHost[]>('/v1/scheduled-tasks'),
  getScheduledTasks: (hostId: string) => api.get<ScheduledTask[]>(`/v1/hosts/${hostId}/scheduled-tasks`),
  createScheduledTask: (hostId: string, payload: Partial<ScheduledTaskRequest>) =>
    api.post(`/v1/hosts/${hostId}/scheduled-tasks`, payload),
  updateScheduledTask: (id: string, payload: Partial<ScheduledTaskRequest>) =>
    api.put(`/v1/scheduled-tasks/${id}`, payload),
  deleteScheduledTask: (id: string) => api.delete(`/v1/scheduled-tasks/${id}`),
  runScheduledTask: (id: string) => api.post(`/v1/scheduled-tasks/${id}/run`),
  getScheduledTaskExecutions: (id: string, limit?: number) =>
    api.get(`/v1/scheduled-tasks/${id}/executions`, { params: { limit: limit ?? 20 } }),
  getHostCustomTasks: (hostId: string) => api.get<CustomTaskSummary[]>(`/v1/hosts/${hostId}/custom-tasks`),
  getHostTasksYaml: (hostId: string) => api.get<{ yaml: string }>(`/v1/hosts/${hostId}/tasks-yaml`),
  getHostComposeProjects: (hostId: string) => api.get<ComposeProject[]>(`/v1/hosts/${hostId}/compose-projects`),
}

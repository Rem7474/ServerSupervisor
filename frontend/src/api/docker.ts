import { api } from './client'
import type { DockerContainer, ComposeProject, DockerContainersPage } from '../types/docker'

export const dockerApi = {
  getContainers: (hostId: string) => api.get<DockerContainer[]>(`/v1/hosts/${hostId}/containers`),
  getAllContainers: () => api.get<DockerContainersPage>('/v1/docker/containers'),
  getComposeProjects: () => api.get<ComposeProject[]>('/v1/docker/compose'),
  sendDockerCommand: (hostId: string, containerName: string, action: string, workingDir?: string) =>
    api.post('/v1/docker/command', { host_id: hostId, container_name: containerName, action, working_dir: workingDir ?? '' }),
  sendJournalCommand: (hostId: string, serviceName: string) =>
    api.post('/v1/system/journalctl', { host_id: hostId, service_name: serviceName }),
  sendSystemdCommand: (hostId: string, serviceName: string, action: string) =>
    api.post('/v1/system/service', { host_id: hostId, service_name: serviceName, action }),
  sendProcessesCommand: (hostId: string) =>
    api.post('/v1/system/processes', { host_id: hostId }),
  getHostCommandHistory: (hostId: string, limit?: number) =>
    api.get(`/v1/hosts/${hostId}/commands/history`, { params: { limit: limit ?? 50 } }),
}

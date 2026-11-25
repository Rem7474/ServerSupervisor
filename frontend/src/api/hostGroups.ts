import { api } from './client'
import type { HostGroup, HostGroupRequest } from '../types/generated'

export const hostGroupsApi = {
  getHostGroups: () => api.get<HostGroup[]>('/v1/host-groups'),
  createHostGroup: (payload: HostGroupRequest) => api.post<HostGroup>('/v1/host-groups', payload),
  updateHostGroup: (id: string, payload: HostGroupRequest) => api.put<HostGroup>(`/v1/host-groups/${id}`, payload),
  deleteHostGroup: (id: string) => api.delete(`/v1/host-groups/${id}`),
}

import { api } from './client'
import type { MaintenanceWindow, MaintenanceWindowRequest } from '../types/maintenance'

export const maintenanceApi = {
  getAllMaintenanceWindows: () => api.get<MaintenanceWindow[]>('/v1/maintenance-windows'),
  getMaintenanceWindowsForHost: (hostId: string) =>
    api.get<MaintenanceWindow[]>(`/v1/hosts/${hostId}/maintenance-windows`),
  createMaintenanceWindow: (hostId: string, payload: MaintenanceWindowRequest) =>
    api.post<MaintenanceWindow>(`/v1/hosts/${hostId}/maintenance-windows`, payload),
  createGlobalMaintenanceWindow: (payload: MaintenanceWindowRequest) =>
    api.post<MaintenanceWindow>('/v1/maintenance-windows/global', payload),
  deleteMaintenanceWindow: (id: string) => api.delete(`/v1/maintenance-windows/${id}`),
}

import { Ref, ref } from 'vue'
import { useConfirmDialog } from './useConfirmDialog'
import apiClient, { getApiErrorMessage } from '../api'
import type { MaintenanceWindow, MaintenanceWindowRequest } from '../types/maintenance'

interface UseMaintenanceWindowsApi {
  windows: Ref<MaintenanceWindow[]>
  loading: Ref<boolean>
  fetched: Ref<boolean>
  error: Ref<string>
  saving: Ref<boolean>
  saveError: Ref<string>
  load: () => Promise<void>
  create: (hostId: string | null, payload: MaintenanceWindowRequest) => Promise<boolean>
  remove: (window: MaintenanceWindow) => Promise<void>
}

// Fleet-wide maintenance windows (the global /alerts view, "Maintenance" tab).
// A host-scoped view (e.g. a future HostDetailView panel) should call
// apiClient.getMaintenanceWindowsForHost directly rather than filtering this
// list client-side, since a host-scoped read is already its own endpoint.
export function useMaintenanceWindows(): UseMaintenanceWindowsApi {
  const { confirm } = useConfirmDialog()

  const windows: Ref<MaintenanceWindow[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const fetched: Ref<boolean> = ref(false)
  const error: Ref<string> = ref('')
  const saving: Ref<boolean> = ref(false)
  const saveError: Ref<string> = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.getAllMaintenanceWindows()
      windows.value = res.data || []
      fetched.value = true
    } catch (e) {
      error.value = getApiErrorMessage(e, 'Impossible de charger les fenêtres de maintenance')
    } finally {
      loading.value = false
    }
  }

  // hostId null creates a global (all-hosts) window — admin-only server-side.
  async function create(hostId: string | null, payload: MaintenanceWindowRequest): Promise<boolean> {
    saving.value = true
    saveError.value = ''
    try {
      if (hostId) {
        await apiClient.createMaintenanceWindow(hostId, payload)
      } else {
        await apiClient.createGlobalMaintenanceWindow(payload)
      }
      await load()
      return true
    } catch (e) {
      saveError.value = getApiErrorMessage(e, 'Impossible de créer la fenêtre de maintenance')
      return false
    } finally {
      saving.value = false
    }
  }

  async function remove(window: MaintenanceWindow): Promise<void> {
    const label = window.host_name || (window.host_id ? window.host_id : 'tous les hôtes')
    const ok = await confirm({
      title: 'Supprimer la fenêtre de maintenance',
      message: `Supprimer la fenêtre de maintenance "${window.reason}" (${label}) ?`,
      variant: 'danger',
      destructive: true,
      okLabel: 'Supprimer',
    })
    if (!ok) return
    try {
      await apiClient.deleteMaintenanceWindow(window.id)
      windows.value = windows.value.filter((w) => w.id !== window.id)
    } catch (e) {
      error.value = getApiErrorMessage(e, 'Impossible de supprimer la fenêtre de maintenance')
    }
  }

  return { windows, loading, fetched, error, saving, saveError, load, create, remove }
}

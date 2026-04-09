import { onMounted, ref, Ref } from 'vue'
import apiClient from '../api'

type AuditLog = Record<string, unknown> & {
  id?: string | number
  type?: string
  title?: string
  description?: string
  command?: string
  action?: string
  status?: string
  timestamp?: string | number | Date
  created_at?: string | number | Date
}

export function useAuditLogs() {
  const auditLogs: Ref<AuditLog[]> = ref([])
  const isLoading = ref(false)

  async function fetchAuditLogs(): Promise<void> {
    isLoading.value = true
    try {
      const response = await apiClient.getAuditLogs(1, 100)
      const payload = response.data
      const logs = Array.isArray(payload)
        ? payload
        : payload?.logs || payload?.audit_logs || payload?.items || []
      auditLogs.value = Array.isArray(logs) ? logs : []
    } catch {
      auditLogs.value = []
    } finally {
      isLoading.value = false
    }
  }

  onMounted(() => {
    void fetchAuditLogs()
  })

  return {
    auditLogs,
    isLoading,
    fetchAuditLogs,
  }
}

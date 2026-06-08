import { onMounted, ref, Ref } from 'vue'
import apiClient from '../api'
import type { AuditLog } from '../types/audit'

export function useAuditLogs() {
  const auditLogs: Ref<AuditLog[]> = ref([])
  const isLoading = ref(false)

  async function fetchAuditLogs(): Promise<void> {
    isLoading.value = true
    try {
      const response = await apiClient.getAuditLogs(1, 100)
      auditLogs.value = response.data?.logs ?? []
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

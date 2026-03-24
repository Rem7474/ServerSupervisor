import { ref, Ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'

const TTL_MS = 30_000 // 30 seconds

interface AlertRule {
  id?: number
  [key: string]: any
}

export const useAlertRulesStore = defineStore('alertRules', () => {
  const rules: Ref<AlertRule[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const fetchedAt: Ref<number | null> = ref(null)

  async function fetchRules(force: boolean = false): Promise<void> {
    if (!force && fetchedAt.value && Date.now() - fetchedAt.value < TTL_MS) return
    loading.value = true
    try {
      const res = await apiClient.getAlertRules()
      rules.value = res.data || []
      fetchedAt.value = Date.now()
    } catch {
      // Keep stale data on error
    } finally {
      loading.value = false
    }
  }

  function invalidate(): void {
    fetchedAt.value = null
  }

  return { rules, loading, fetchRules, invalidate }
})

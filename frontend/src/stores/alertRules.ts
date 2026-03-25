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
  const error: Ref<string> = ref('')
  const fetched: Ref<boolean> = ref(false)
  const fetchedAt: Ref<number | null> = ref(null)

  async function fetchRules(force: boolean = false): Promise<void> {
    if (!force && fetchedAt.value && Date.now() - fetchedAt.value < TTL_MS) return
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.getAlertRules()
      rules.value = res.data || []
      fetchedAt.value = Date.now()
    } catch (e: any) {
      // Keep stale data on error
      error.value = e?.response?.data?.error || e?.message || 'Erreur de chargement'
    } finally {
      loading.value = false
      fetched.value = true
    }
  }

  function invalidate(): void {
    fetchedAt.value = null
    fetched.value = false
  }

  return { rules, loading, error, fetched, fetchRules, invalidate }
})

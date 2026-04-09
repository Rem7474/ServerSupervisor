import { ref, Ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'

const TTL_MS = 30_000 // 30 seconds

interface AlertRule {
  id?: number
  [key: string]: unknown
}

function getErrorMessage(error: unknown): string {
  if (typeof error === 'object' && error !== null) {
    const response = 'response' in error ? (error as { response?: { data?: { error?: unknown } } }).response : undefined
    const responseError = response?.data?.error
    if (typeof responseError === 'string') return responseError
    const message = 'message' in error ? (error as { message?: unknown }).message : undefined
    if (typeof message === 'string') return message
  }
  return 'Erreur de chargement'
}

export const useAlertRulesStore = defineStore('alertRules', () => {
  const rules: Ref<AlertRule[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const error: Ref<string> = ref('')
  const fetched: Ref<boolean> = ref(false)
  const fetchedAt: Ref<number | null> = ref(null)

  async function fetchRules(force: boolean = false): Promise<void> {
    if (!force && fetchedAt.value && Date.now() - fetchedAt.value < TTL_MS) {
      fetched.value = true
      return
    }
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.getAlertRules()
      rules.value = res.data || []
      fetchedAt.value = Date.now()
    } catch (e: unknown) {
      // Keep stale data on error
      error.value = getErrorMessage(e)
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

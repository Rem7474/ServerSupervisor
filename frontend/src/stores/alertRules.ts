import { ref, Ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import type { AlertRule } from '../types/alert'
import { i18n } from '../i18n'

const TTL_MS = 30_000 // 30 seconds

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
      error.value = getApiErrorMessage(e, i18n.global.t('common.loadError'))
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

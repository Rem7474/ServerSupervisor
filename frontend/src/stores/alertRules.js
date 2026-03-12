import { ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'

const TTL_MS = 30_000 // 30 secondes

export const useAlertRulesStore = defineStore('alertRules', () => {
  const rules = ref([])
  const loading = ref(false)
  const fetchedAt = ref(null)

  async function fetchRules(force = false) {
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

  function invalidate() {
    fetchedAt.value = null
  }

  return { rules, loading, fetchRules, invalidate }
})

import { ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'

const TTL_MS = 30_000 // 30 secondes

export const useAlertRulesStore = defineStore('alertRules', () => {
  const rules = ref([])
  const loading = ref(false)
  const error = ref('')
  const fetched = ref(false)  // true après le premier fetch abouti (succès ou erreur)
  const fetchedAt = ref(null)

  async function fetchRules(force = false) {
    if (!force && fetchedAt.value && Date.now() - fetchedAt.value < TTL_MS) return
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.getAlertRules()
      rules.value = res.data || []
      fetchedAt.value = Date.now()
    } catch (err) {
      error.value = err?.response?.data?.error || err?.message || 'Erreur de chargement'
    } finally {
      loading.value = false
      fetched.value = true
    }
  }

  function invalidate() {
    fetchedAt.value = null
    fetched.value = false
  }

  return { rules, loading, error, fetched, fetchRules, invalidate }
})

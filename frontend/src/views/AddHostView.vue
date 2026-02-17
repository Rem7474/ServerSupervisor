<template>
  <div>
    <div class="flex items-center gap-4 mb-8">
      <router-link to="/" class="text-gray-400 hover:text-gray-200">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"/>
        </svg>
      </router-link>
      <h1 class="text-2xl font-bold">Ajouter un hôte</h1>
    </div>

    <div class="card max-w-lg">
      <form @submit.prevent="handleSubmit" class="space-y-6">
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">Hostname</label>
          <input v-model="form.hostname" type="text" class="input-field" required placeholder="my-server-01" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">Adresse IP</label>
          <input v-model="form.ip_address" type="text" class="input-field" required placeholder="192.168.1.100" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">Système d'exploitation</label>
          <input v-model="form.os" type="text" class="input-field" required placeholder="Ubuntu 22.04 LTS" />
        </div>

        <div v-if="error" class="bg-red-500/10 border border-red-500/30 rounded-lg p-3 text-red-400 text-sm">
          {{ error }}
        </div>

        <button type="submit" class="btn-primary w-full" :disabled="loading">
          {{ loading ? 'Enregistrement...' : 'Enregistrer l\'hôte' }}
        </button>
      </form>

      <!-- Success card with API key -->
      <div v-if="result" class="mt-6 bg-emerald-500/10 border border-emerald-500/30 rounded-lg p-6">
        <h3 class="text-emerald-400 font-semibold mb-3">Hôte enregistré avec succès !</h3>
        <p class="text-sm text-gray-300 mb-4">Utilisez cette clé API dans la configuration de l'agent :</p>
        <div class="bg-dark-900 rounded-lg p-4">
          <code class="text-primary-400 text-sm break-all">{{ result.api_key }}</code>
        </div>
        <p class="text-xs text-gray-400 mt-3">⚠ Cette clé ne sera plus affichée. Copiez-la maintenant.</p>

        <div class="mt-4 bg-dark-900 rounded-lg p-4">
          <p class="text-sm text-gray-400 mb-2">Configuration agent :</p>
          <pre class="text-xs text-gray-300">server_url: "http://{{ window.location.hostname }}:8080"
api_key: "{{ result.api_key }}"
report_interval: 30
collect_docker: true
collect_apt: true</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import apiClient from '../api'

const form = ref({ hostname: '', ip_address: '', os: '' })
const error = ref('')
const loading = ref(false)
const result = ref(null)

async function handleSubmit() {
  loading.value = true
  error.value = ''
  try {
    const res = await apiClient.registerHost(form.value)
    result.value = res.data
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de l\'enregistrement'
  } finally {
    loading.value = false
  }
}
</script>

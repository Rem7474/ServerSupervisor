<template>
  <div>
    <div class="page-header d-flex align-items-center gap-3 mb-4">
      <router-link to="/" class="btn btn-outline-secondary btn-icon" aria-label="Retour">
        <svg class="icon" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"/>
        </svg>
      </router-link>
      <div>
        <h2 class="page-title">Ajouter un hote</h2>
        <div class="text-secondary">Enregistrer un nouvel hote</div>
      </div>
    </div>

    <div class="card" style="max-width: 520px;">
      <div class="card-body">
        <form @submit.prevent="handleSubmit">
          <div class="mb-3">
            <label class="form-label">Nom (alias personnel)</label>
            <input v-model="form.name" type="text" class="form-control" required placeholder="Prod Web Server" />
          </div>
          <div class="mb-3">
            <label class="form-label">Adresse IP</label>
            <input v-model="form.ip_address" type="text" class="form-control" required placeholder="192.168.1.100" />
          </div>

          <div v-if="error" class="alert alert-danger" role="alert">
            {{ error }}
          </div>

          <div class="text-secondary small mb-3">
            OS et Hostname seront recuperes automatiquement lors de la premiere connexion de l'agent.
          </div>

          <button type="submit" class="btn btn-primary w-100" :disabled="loading">
            {{ loading ? 'Enregistrement...' : 'Enregistrer l\'hote' }}
          </button>
        </form>

        <div v-if="result" class="alert alert-success mt-4" role="alert">
          <div class="fw-semibold mb-2">Hote enregistre avec succes</div>
          <div class="text-secondary mb-2">Utilisez cette cle API dans la configuration de l'agent :</div>
          <div class="bg-dark rounded p-2 mb-2">
            <code class="text-light">{{ result.api_key }}</code>
          </div>
          <div class="text-secondary small">Cette cle ne sera plus affichee. Copiez-la maintenant.</div>
          <div class="mt-3">
            <div class="text-secondary small mb-1">Configuration agent :</div>
            <pre class="bg-dark text-light p-2 rounded small">server_url: "http://{{ serverHostname }}:8080"
api_key: "{{ result.api_key }}"
report_interval: 30
collect_docker: true
collect_apt: true</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import apiClient from '../api'

const serverHostname =
  typeof window !== 'undefined' && window.location?.hostname
    ? window.location.hostname
    : 'localhost'

const form = ref({ name: '', ip_address: '' })
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

<template>
  <div>
    <div class="page-header mb-4">
      <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <div class="text-secondary small">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="mx-1">/</span>
            <span>Ajouter un hote</span>
          </div>
          <h2 class="page-title">Ajouter un hote</h2>
          <div class="text-secondary">Enregistrer un nouvel hote</div>
        </div>
        <router-link to="/" class="btn btn-outline-secondary">Retour au dashboard</router-link>
      </div>
    </div>

    <div class="row justify-content-center">
      <div class="col-12 col-md-8 col-lg-6">
        <div class="card">
          <div class="card-body">
            <form v-if="!result" @submit.prevent="handleSubmit">
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

            <div v-else class="alert alert-success" role="alert">
              <div class="fw-semibold mb-2">Hote enregistre avec succes</div>
              <div class="text-secondary mb-2">Utilisez cette cle API dans la configuration de l'agent :</div>
              <div class="d-flex align-items-center gap-2 mb-2">
                <div class="bg-dark rounded p-2 flex-fill">
                  <code class="text-light">{{ result.api_key }}</code>
                </div>
                <button type="button" class="btn btn-outline-light" @click="copyApiKey">
                  {{ copiedApiKey ? 'Copie' : 'Copier' }}
                </button>
              </div>
              <div class="text-secondary small">Cette cle ne sera plus affichee. Copiez-la maintenant.</div>
              <div class="mt-3">
                <div class="d-flex align-items-center justify-content-between mb-1">
                  <div class="text-secondary small">Configuration agent :</div>
                  <button type="button" class="btn btn-outline-light btn-sm" @click="copyAgentConfig">
                    {{ copiedConfig ? 'Copie' : 'Copier la config' }}
                  </button>
                </div>
                <pre class="bg-dark text-light p-2 rounded small">{{ agentConfig }}</pre>
              </div>
              <div class="mt-3 d-flex justify-content-end">
                <button type="button" class="btn btn-success" @click="finishAdd">Termine</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import apiClient from '../api'

const serverHostname =
  typeof window !== 'undefined' && window.location?.hostname
    ? window.location.hostname
    : 'localhost'

const form = ref({ name: '', ip_address: '' })
const error = ref('')
const loading = ref(false)
const result = ref(null)
const copiedApiKey = ref(false)
const copiedConfig = ref(false)
const router = useRouter()

const agentConfig = computed(() => {
  if (!result.value) return ''
  return `server_url: "http://${serverHostname}:8080"\napi_key: "${result.value.api_key}"\nreport_interval: 30\ncollect_docker: true\ncollect_apt: true`
})

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

async function copyApiKey() {
  if (!result.value?.api_key) return
  await navigator.clipboard.writeText(result.value.api_key)
  copiedApiKey.value = true
  setTimeout(() => {
    copiedApiKey.value = false
  }, 1500)
}

async function copyAgentConfig() {
  if (!agentConfig.value) return
  await navigator.clipboard.writeText(agentConfig.value)
  copiedConfig.value = true
  setTimeout(() => {
    copiedConfig.value = false
  }, 1500)
}

function finishAdd() {
  if (result.value?.id) {
    router.push(`/hosts/${result.value.id}`)
  } else {
    router.push('/')
  }
}
</script>

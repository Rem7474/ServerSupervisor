<template>
  <div>
    <div class="page-header mb-4">
      <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <div class="text-secondary small">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="mx-1">/</span>
            <span>Ajouter un hôte</span>
          </div>
          <h2 class="page-title">Ajouter un hôte</h2>
          <div class="text-secondary">Enregistrer un nouvel hôte</div>
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
                {{ loading ? 'Enregistrement...' : 'Enregistrer l\'hôte' }}
              </button>
            </form>

            <div v-else class="host-success" role="alert">
              <div class="host-success-header">
                <div>
                  <div class="fw-semibold">Hôte enregistré avec succès</div>
                  <div class="text-secondary small">La cle ne sera plus affichee. Copiez-la maintenant.</div>
                </div>
                <button type="button" class="btn btn-success" @click="finishAdd">Termine</button>
              </div>

              <div class="host-success-grid">
                <div class="host-success-card">
                  <div class="text-secondary small mb-2">Cle API agent</div>
                  <div class="host-success-key">
                    <code>{{ result.api_key }}</code>
                    <button type="button" class="btn btn-outline-light btn-sm" @click="copyApiKey">
                      {{ copiedApiKey ? 'Copie' : 'Copier' }}
                    </button>
                  </div>
                </div>
                <div class="host-success-card">
                  <div class="d-flex align-items-center justify-content-between mb-2">
                    <div class="text-secondary small">Configuration agent</div>
                    <button type="button" class="btn btn-outline-light btn-sm" @click="copyAgentConfig">
                      {{ copiedConfig ? 'Copie' : 'Copier la config' }}
                    </button>
                  </div>
                  <pre class="host-success-config">{{ agentConfig }}</pre>
                </div>
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

<style scoped>
.host-success {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 14px;
  padding: 20px;
  color: #e2e8f0;
}

.host-success-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.host-success-grid {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(280px, 1.4fr);
  gap: 16px;
}

.host-success-card {
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 12px;
  padding: 14px;
}

.host-success-key {
  display: flex;
  align-items: center;
  gap: 10px;
}

.host-success-key code {
  display: block;
  background: rgba(2, 6, 23, 0.6);
  color: #f8fafc;
  padding: 8px 10px;
  border-radius: 8px;
  flex: 1;
  word-break: break-all;
}

.host-success-config {
  background: rgba(2, 6, 23, 0.6);
  color: #e2e8f0;
  border-radius: 10px;
  padding: 10px;
  margin: 0;
  font-size: 12px;
}

@media (max-width: 991px) {
  .host-success-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .host-success-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<template>
  <div>
    <div class="page-header mb-4">
      <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              Dashboard
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>Ajouter un hôte</span>
          </div>
          <h2 class="page-title">
            Ajouter un hôte
          </h2>
          <div class="text-secondary">
            Enregistrer un nouvel hôte
          </div>
        </div>
        <router-link
          to="/"
          class="btn btn-outline-secondary"
        >
          Retour au dashboard
        </router-link>
      </div>
    </div>

    <div class="row justify-content-center">
      <div class="col-12 col-md-8 col-lg-6">
        <div class="card">
          <div class="card-body">
            <form
              v-if="!result"
              @submit.prevent="handleSubmit"
            >
              <div class="mb-3">
                <label
                  class="form-label"
                  for="host-name"
                >Nom (alias personnel)</label>
                <input
                  id="host-name"
                  v-model="form.name"
                  type="text"
                  :class="['form-control', touched.name && !form.name.trim() ? 'is-invalid' : '']"
                  required
                  placeholder="Ex: Prod Web Server"
                  @blur="touched.name = true"
                >
                <div
                  v-if="touched.name && !form.name.trim()"
                  class="invalid-feedback"
                >
                  Ce champ est requis
                </div>
              </div>
              <div class="mb-3">
                <label
                  class="form-label"
                  for="host-ip"
                >Adresse IP</label>
                <input
                  id="host-ip"
                  v-model="form.ip_address"
                  type="text"
                  :class="['form-control', touched.ip_address && isValidIp === false ? 'is-invalid' : touched.ip_address && isValidIp ? 'is-valid' : '']"
                  required
                  placeholder="192.168.1.100"
                  @blur="touched.ip_address = true"
                >
                <div
                  v-if="ipFeedback"
                  class="invalid-feedback"
                >
                  {{ ipFeedback }}
                </div>
              </div>

              <div
                v-if="error"
                class="alert alert-danger"
                role="alert"
              >
                {{ error }}
              </div>

              <div class="text-secondary small mb-3">
                OS et Hostname seront récupérés automatiquement lors de la première connexion de l'agent.
              </div>

              <button
                type="submit"
                class="btn btn-primary w-100"
                :disabled="loading"
              >
                {{ loading ? 'Enregistrement...' : 'Enregistrer l\'hôte' }}
              </button>
            </form>

            <div
              v-else
              class="host-success"
              role="alert"
            >
              <div class="host-success-header">
                <div>
                  <div class="fw-semibold">
                    Hôte enregistré avec succès
                  </div>
                  <div class="text-secondary small">
                    La clé ne sera plus affichée. Copiez-la maintenant.
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-success"
                  @click="finishAdd"
                >
                  Terminé
                </button>
              </div>

              <!-- Agent connection status -->
              <div
                class="agent-connection-status mb-3"
                :class="agentConnected ? 'agent-connected' : 'agent-waiting'"
              >
                <template v-if="agentConnected">
                  <IconCircleCheck
                    :size="16"
                    :stroke-width="2.5"
                  />
                  Agent connecté — premier rapport reçu !
                </template>
                <template v-else>
                  <span
                    class="spinner-border spinner-border-sm"
                    style="width:.8rem;height:.8rem;border-width:2px"
                  />
                  En attente du premier rapport agent…
                </template>
              </div>

              <div class="host-success-card mb-3">
                <div class="d-flex align-items-center justify-content-between mb-2">
                  <div class="text-secondary small">
                    Installation en une commande
                  </div>
                  <button
                    type="button"
                    class="btn btn-outline-light btn-sm"
                    @click="copyInstallCmd"
                  >
                    {{ copiedInstall ? 'Copié' : 'Copier' }}
                  </button>
                </div>
                <pre class="host-success-config host-success-install">{{ installCmd }}</pre>
              </div>

              <div class="host-success-grid">
                <div class="host-success-card">
                  <div class="text-secondary small mb-2">
                    Clé API agent
                  </div>
                  <div class="host-success-key">
                    <code>{{ result.api_key }}</code>
                    <button
                      type="button"
                      class="btn btn-outline-light btn-sm"
                      @click="copyApiKey"
                    >
                      {{ copiedApiKey ? 'Copié' : 'Copier' }}
                    </button>
                  </div>
                </div>
                <div class="host-success-card">
                  <div class="d-flex align-items-center justify-content-between mb-2">
                    <div class="text-secondary small">
                      Configuration agent (YAML)
                    </div>
                    <button
                      type="button"
                      class="btn btn-outline-light btn-sm"
                      @click="copyAgentConfig"
                    >
                      {{ copiedConfig ? 'Copié' : 'Copier' }}
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

<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { IconCircleCheck } from '@tabler/icons-vue'
import { useRouter } from 'vue-router'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'

interface HostResult {
  id?: string
  api_key?: string
}

const serverUrl =
  typeof window !== 'undefined' && window.location?.origin
    ? window.location.origin
    : 'http://localhost:8080'

const INSTALL_SCRIPT_URL =
  'https://raw.githubusercontent.com/Rem7474/ServerSupervisor/main/agent/install.sh'

const form = ref({ name: '', ip_address: '' })
const error = ref('')
const loading = ref(false)
const touched = ref({ name: false, ip_address: false })

const IP_RE = /^(\d{1,3}\.){3}\d{1,3}$/
const isValidIp = computed<boolean | null>(() => {
  const v = form.value.ip_address.trim()
  if (!v) return null
  if (!IP_RE.test(v)) return false
  return v.split('.').every((n) => Number(n) <= 255)
})
const ipFeedback = computed(() => {
  if (!touched.value.ip_address || isValidIp.value === null) return ''
  return isValidIp.value ? '' : 'Adresse IPv4 invalide (ex: 192.168.1.100)'
})
const result = ref<HostResult | null>(null)
const copiedApiKey = ref(false)
const copiedConfig = ref(false)
const copiedInstall = ref(false)
const agentConnected = ref(false)
const router = useRouter()

const AGENT_POLL_INTERVAL_MS = 3000
const AGENT_POLL_TIMEOUT_MS = 120_000
let agentPollTimer: ReturnType<typeof setInterval> | null = null
let agentPollStarted: number | null = null

function startAgentPolling(hostId: string): void {
  agentPollStarted = Date.now()
  agentPollTimer = setInterval(async () => {
    if (agentPollStarted !== null && Date.now() - agentPollStarted > AGENT_POLL_TIMEOUT_MS) {
      stopAgentPolling()
      return
    }
    try {
      const res = await apiClient.getHost(hostId)
      if (res.data?.status === 'online' || res.data?.status === 'warning') {
        agentConnected.value = true
        stopAgentPolling()
      }
    } catch {
      // ignore transient errors, keep polling
    }
  }, AGENT_POLL_INTERVAL_MS)
}

function stopAgentPolling(): void {
  if (agentPollTimer) {
    clearInterval(agentPollTimer)
    agentPollTimer = null
  }
}

onUnmounted(stopAgentPolling)

const installCmd = computed(() => {
  if (!result.value) return ''
  return `curl -sSL ${INSTALL_SCRIPT_URL} | sudo bash -s -- --server-url ${serverUrl} --api-key "${result.value.api_key}"`
})

const agentConfig = computed(() => {
  if (!result.value) return ''
  return `server_url: "${serverUrl}"\napi_key: "${result.value.api_key}"\nreport_interval: 30\ncollect_docker: true\ncollect_apt: true`
})

async function handleSubmit(): Promise<void> {
  touched.value.name = true
  touched.value.ip_address = true
  if (!form.value.name.trim() || !isValidIp.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await apiClient.registerHost(form.value)
    result.value = res.data
    if (res.data?.id) startAgentPolling(res.data.id)
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, 'Erreur lors de l\'enregistrement')
  } finally {
    loading.value = false
  }
}

async function copyApiKey(): Promise<void> {
  if (!result.value?.api_key) return
  await navigator.clipboard.writeText(result.value.api_key)
  copiedApiKey.value = true
  setTimeout(() => { copiedApiKey.value = false }, 1500)
}

async function copyAgentConfig(): Promise<void> {
  if (!agentConfig.value) return
  await navigator.clipboard.writeText(agentConfig.value)
  copiedConfig.value = true
  setTimeout(() => { copiedConfig.value = false }, 1500)
}

async function copyInstallCmd(): Promise<void> {
  if (!installCmd.value) return
  await navigator.clipboard.writeText(installCmd.value)
  copiedInstall.value = true
  setTimeout(() => { copiedInstall.value = false }, 1500)
}

function finishAdd(): void {
  if (result.value?.id) {
    router.push(`/hosts/${result.value.id}`)
  } else {
    router.push('/')
  }
}
</script>

<style scoped>
.host-success {
  background: var(--ss-panel-medium);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 14px;
  padding: 20px;
  color: var(--ss-text-on-dark);
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
  background: var(--ss-panel-strong);
  border: 1px solid var(--ss-border-default);
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
  color: var(--ss-text-strong);
  padding: 8px 10px;
  border-radius: 8px;
  flex: 1;
  word-break: break-all;
}

.host-success-config {
  background: rgba(2, 6, 23, 0.6);
  color: var(--ss-text-on-dark);
  border-radius: 10px;
  padding: 10px;
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.host-success-install {
  border-left: 3px solid rgba(56, 189, 248, 0.6);
  color: #7dd3fc;
}

.agent-connection-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 500;
}

.agent-waiting {
  background: rgba(99, 102, 241, 0.1);
  border: 1px solid rgba(99, 102, 241, 0.3);
  color: #a5b4fc;
}

.agent-connected {
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.35);
  color: #86efac;
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

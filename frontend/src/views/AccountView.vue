<template>
  <div>
    <!-- Forced password change banner -->
    <div v-if="auth.mustChangePassword" class="alert alert-warning alert-dismissible mb-4" role="alert">
      <div class="d-flex align-items-center">
        <svg class="icon alert-icon me-2" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
        </svg>
        <strong>Changement de mot de passe requis.</strong>&nbsp;Pour des raisons de sécurité, veuillez définir un nouveau mot de passe avant de continuer.
      </div>
    </div>

    <div class="page-header mb-4">
      <div class="row align-items-center">
        <div class="col-auto">
          <div class="page-pretitle">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="text-muted mx-1">/</span>
            <span>Mon compte</span>
          </div>
          <h2 class="page-title">Mon compte</h2>
          <div class="text-secondary">Gérez vos informations personnelles et la sécurité de votre compte</div>
        </div>
      </div>
    </div>

    <div class="row g-4">
      <!-- Profile info card -->
      <div class="col-12 col-lg-4">
        <div class="card">
          <div class="card-body text-center py-4">
            <div class="avatar avatar-xl mb-3" style="width:64px;height:64px;font-size:1.6rem;background:var(--tblr-azure-lt);color:var(--tblr-azure);border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto;">
              {{ auth.username?.slice(0, 2).toUpperCase() }}
            </div>
            <div class="h3 mb-1">{{ profile?.username || auth.username }}</div>
            <div class="mb-3">
              <span class="badge" :class="roleBadgeClass">{{ roleLabel }}</span>
            </div>
            <div class="text-secondary small" v-if="profile?.created_at">
              Membre depuis {{ formatDate(profile.created_at) }}
            </div>
          </div>
          <div class="card-footer text-center py-3">
            <div class="row g-3">
              <div class="col-6 border-end">
                <div class="text-secondary small">MFA</div>
                <div class="fw-bold" :class="profile?.mfa_enabled ? 'text-success' : 'text-secondary'">
                  {{ profile?.mfa_enabled ? 'Activé' : 'Désactivé' }}
                </div>
              </div>
              <div class="col-6">
                <div class="text-secondary small">Statut</div>
                <div class="fw-bold text-success">Actif</div>
              </div>
            </div>
          </div>
        </div>

        <!-- MFA card -->
        <div class="card mt-4">
          <div class="card-header">
            <h3 class="card-title">
              <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
              </svg>
              Authentification à deux facteurs
            </h3>
          </div>
          <div class="card-body">
            <div class="d-flex align-items-center justify-content-between mb-3">
              <div>
                <div class="fw-bold">TOTP (Authenticator)</div>
                <div class="text-secondary small">Google Authenticator, Authy, etc.</div>
              </div>
              <span class="badge" :class="profile?.mfa_enabled ? 'bg-green-lt text-green' : 'bg-orange-lt text-orange'">
                {{ profile?.mfa_enabled ? 'Actif' : 'Inactif' }}
              </span>
            </div>
            <router-link to="/security" class="btn btn-outline-secondary w-100">
              Gérer le MFA
            </router-link>
          </div>
        </div>
      </div>

      <!-- Change password + Activity -->
      <div class="col-12 col-lg-8">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">
              <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/>
              </svg>
              Changer le mot de passe
            </h3>
          </div>
          <div class="card-body">
            <form @submit.prevent="submitChangePassword">
              <div class="mb-3">
                <label class="form-label required">Mot de passe actuel</label>
                <input v-model="pwForm.current" type="password" class="form-control" :class="{ 'is-invalid': pwErrors.current }" placeholder="••••••••" required />
                <div v-if="pwErrors.current" class="invalid-feedback">{{ pwErrors.current }}</div>
              </div>
              <div class="mb-3">
                <label class="form-label required">Nouveau mot de passe</label>
                <input v-model="pwForm.next" type="password" class="form-control" :class="{ 'is-invalid': pwErrors.next }" placeholder="••••••••" required />
                <div v-if="pwErrors.next" class="invalid-feedback">{{ pwErrors.next }}</div>
                <div class="form-hint">Au moins 8 caractères.</div>
              </div>
              <div class="mb-4">
                <label class="form-label required">Confirmer le nouveau mot de passe</label>
                <input v-model="pwForm.confirm" type="password" class="form-control" :class="{ 'is-invalid': pwErrors.confirm }" placeholder="••••••••" required />
                <div v-if="pwErrors.confirm" class="invalid-feedback">{{ pwErrors.confirm }}</div>
              </div>

              <div v-if="pwError" class="alert alert-danger mb-3" role="alert">{{ pwError }}</div>
              <div v-if="pwSuccess" class="alert alert-success mb-3" role="alert">{{ pwSuccess }}</div>

              <div class="d-flex gap-2">
                <button type="submit" class="btn btn-primary" :disabled="pwLoading">
                  <span v-if="pwLoading" class="spinner-border spinner-border-sm me-2"></span>
                  {{ pwLoading ? 'Enregistrement...' : 'Mettre à jour le mot de passe' }}
                </button>
                <button v-if="!auth.mustChangePassword" type="button" class="btn btn-outline-secondary" @click="resetPwForm">
                  Annuler
                </button>
              </div>
            </form>
          </div>
        </div>

        <!-- Recent activity — command history for this user -->
        <div class="card mt-4">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title mb-0">
              <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
              </svg>
              Activité récente
            </h3>
            <span class="badge bg-azure-lt text-azure">{{ myCommands.length }}</span>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Hôte</th>
                  <th>Type</th>
                  <th>Commande</th>
                  <th>Statut</th>
                  <th>Durée</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="cmdsLoading">
                  <td colspan="7" class="text-center text-secondary py-3">Chargement...</td>
                </tr>
                <tr v-else-if="!myCommands.length">
                  <td colspan="7" class="text-center text-secondary py-3">Aucune activité récente</td>
                </tr>
                <tr v-for="cmd in myCommands" :key="cmd.id" :class="{ 'table-active': selectedCmd?.id === cmd.id }">
                  <td class="text-secondary small">{{ formatDateTime(cmd.created_at) }}</td>
                  <td>
                    <router-link :to="`/hosts/${cmd.host_id}`" class="text-decoration-none fw-semibold">
                      {{ cmd.host_name || cmd.host_id }}
                    </router-link>
                  </td>
                  <td><span :class="moduleClass(cmd.module)">{{ moduleLabel(cmd.module) }}</span></td>
                  <td><code class="small">{{ cmdLabel(cmd) }}</code></td>
                  <td><span :class="statusClass(cmd.status)">{{ cmd.status }}</span></td>
                  <td class="text-secondary small">{{ formatDuration(cmd.started_at, cmd.ended_at) }}</td>
                  <td>
                    <button
                      class="btn btn-sm btn-outline-secondary"
                      @click="openLogViewer(cmd)"
                      :disabled="!cmd.output && cmd.status === 'pending'"
                    >Logs</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Inline log viewer -->
          <div v-if="selectedCmd" class="border-top">
            <div class="px-3 pt-3 pb-2" style="background: #1e293b;">
              <div class="d-flex align-items-center justify-content-between">
                <div class="d-flex align-items-center gap-2 flex-wrap">
                  <span :class="moduleClass(selectedCmd.module)">{{ moduleLabel(selectedCmd.module) }}</span>
                  <code style="color: #94a3b8;">{{ cmdLabel(selectedCmd) }}</code>
                  <span class="text-secondary small">— {{ selectedCmd.host_name || selectedCmd.host_id }}</span>
                  <span :class="statusClass(selectedCmd.status)">{{ selectedCmd.status }}</span>
                </div>
                <button class="btn btn-sm btn-ghost-secondary" @click="closeLogViewer" title="Fermer">
                  <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="20" height="20" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none">
                    <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                    <path d="M18 6l-12 12" /><path d="M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>
            <pre
              ref="logViewerEl"
              style="background:#0f172a;color:#e2e8f0;padding:1rem;margin:0;max-height:350px;overflow-y:auto;font-family:'Consolas','Monaco','Courier New',monospace;font-size:0.813rem;line-height:1.5;"
            >{{ liveOutput || 'Aucune sortie disponible.' }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import { formatDateLong as formatDate, formatDateTime } from '../utils/formatters'

const auth = useAuthStore()

const profile = ref(null)

const pwForm = ref({ current: '', next: '', confirm: '' })
const pwErrors = ref({ current: '', next: '', confirm: '' })
const pwError = ref('')
const pwSuccess = ref('')
const pwLoading = ref(false)

// Commands history
const allCommands = ref([])
const cmdsLoading = ref(false)
const myCommands = computed(() =>
  allCommands.value.filter(c => c.triggered_by === auth.username).slice(0, 20)
)

// Log viewer
const selectedCmd = ref(null)
const liveOutput = ref('')
const logViewerEl = ref(null)
let streamWs = null

const roleBadgeClass = computed(() => {
  const map = { admin: 'bg-danger-lt text-danger', operator: 'bg-warning-lt text-warning', viewer: 'bg-secondary-lt text-secondary' }
  return map[profile.value?.role] || 'bg-secondary-lt text-secondary'
})

const roleLabel = computed(() => {
  const map = { admin: 'Administrateur', operator: 'Opérateur', viewer: 'Lecteur' }
  return map[profile.value?.role] || profile.value?.role || auth.role
})

const MODULE_META = {
  apt:       { label: 'APT',        cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',     cls: 'badge bg-blue-lt text-blue' },
  systemd:   { label: 'Systemd',    cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',    cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus',  cls: 'badge bg-orange-lt text-orange' },
}
function moduleLabel(module) { return MODULE_META[module]?.label ?? module }
function moduleClass(module) { return MODULE_META[module]?.cls ?? 'badge bg-secondary-lt text-secondary' }
function cmdLabel(cmd) { return [cmd.action, cmd.target].filter(Boolean).join(' ') }

function formatDuration(startedAt, endedAt) {
  if (!startedAt || !endedAt) return '—'
  const diff = Math.max(0, Math.round((new Date(endedAt) - new Date(startedAt)) / 1000))
  if (diff < 60) return `${diff}s`
  const m = Math.floor(diff / 60), s = diff % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

function renderOutput(raw) {
  if (!raw) return ''
  const lines = ['']
  let cur = ''
  for (const ch of raw) {
    if (ch === '\r') { cur = ''; lines[lines.length - 1] = ''; continue }
    if (ch === '\n') { cur = ''; lines.push(''); continue }
    cur += ch; lines[lines.length - 1] = cur
  }
  return lines.join('\n')
}

function openLogViewer(cmd) {
  if (selectedCmd.value?.id === cmd.id) { closeLogViewer(); return }
  closeLogViewer()
  selectedCmd.value = cmd
  liveOutput.value = renderOutput(cmd.output || '')
  if (cmd.status === 'running' || cmd.status === 'pending') connectStream(cmd.id)
}

function closeLogViewer() {
  if (streamWs) { streamWs.close(); streamWs = null }
  selectedCmd.value = null
  liveOutput.value = ''
}

function connectStream(commandId) {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  streamWs = new WebSocket(`${protocol}://${window.location.host}/api/v1/ws/commands/stream/${commandId}`)
  streamWs.onopen = () => streamWs.send(JSON.stringify({ type: 'auth', token: auth.token }))
  streamWs.onmessage = (event) => {
    try {
      const p = JSON.parse(event.data)
      if (p.type === 'cmd_stream_init') {
        if (selectedCmd.value) selectedCmd.value.status = p.status
        liveOutput.value = renderOutput(p.output || '')
      } else if (p.type === 'cmd_stream') {
        liveOutput.value += p.chunk
      } else if (p.type === 'cmd_status_update') {
        if (selectedCmd.value) selectedCmd.value.status = p.status
        if (p.output) liveOutput.value = renderOutput(p.output)
      }
    } catch { /* ignore */ }
  }
}

function resetPwForm() {
  pwForm.value = { current: '', next: '', confirm: '' }
  pwErrors.value = { current: '', next: '', confirm: '' }
  pwError.value = ''
  pwSuccess.value = ''
}

async function submitChangePassword() {
  pwErrors.value = { current: '', next: '', confirm: '' }
  pwError.value = ''
  pwSuccess.value = ''

  let valid = true
  if (!pwForm.value.current) { pwErrors.value.current = 'Le mot de passe actuel est requis.'; valid = false }
  if (pwForm.value.next.length < 8) { pwErrors.value.next = 'Le nouveau mot de passe doit faire au moins 8 caractères.'; valid = false }
  if (pwForm.value.next !== pwForm.value.confirm) { pwErrors.value.confirm = 'La confirmation ne correspond pas.'; valid = false }
  if (!valid) return

  pwLoading.value = true
  try {
    await apiClient.changePassword(pwForm.value.current, pwForm.value.next)
    pwSuccess.value = 'Mot de passe mis à jour avec succès.'
    pwForm.value = { current: '', next: '', confirm: '' }
    auth.clearMustChangePassword()
  } catch (e) {
    pwError.value = e.response?.data?.error || 'Erreur lors de la mise à jour du mot de passe.'
  } finally {
    pwLoading.value = false
  }
}

async function loadProfile() {
  try {
    const { data } = await apiClient.getProfile()
    profile.value = data
  } catch { /* fallback to store data */ }
}

async function loadMyCommands() {
  cmdsLoading.value = true
  try {
    const res = await apiClient.getCommandsHistory(1, 100)
    allCommands.value = res.data?.commands || []
  } catch {
    allCommands.value = []
  } finally {
    cmdsLoading.value = false
  }
}

onMounted(() => {
  loadProfile()
  loadMyCommands()
})

onUnmounted(() => { if (streamWs) streamWs.close() })
</script>

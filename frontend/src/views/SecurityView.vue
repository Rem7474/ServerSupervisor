<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Sécurité</h2>
        <div class="text-secondary">MFA, activité de connexion et menaces</div>
      </div>
    </div>

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'mfa' }" href="#" @click.prevent="tab = 'mfa'">
          Authentification MFA
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'activity' }" href="#" @click.prevent="switchToActivity">
          Activité de connexion
        </a>
      </li>
      <li class="nav-item" v-if="isAdmin">
        <a class="nav-link" :class="{ active: tab === 'threats' }" href="#" @click.prevent="switchToThreats">
          Menaces
        </a>
      </li>
    </ul>

    <!-- MFA Tab -->
    <div v-if="tab === 'mfa'" class="card" style="max-width: 640px;">
      <div class="card-body">
        <div class="d-flex align-items-center justify-content-between mb-3">
          <div class="fw-semibold">Authentification multi-facteur</div>
          <span :class="mfaEnabled ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
            {{ mfaEnabled ? 'Activé' : 'Désactivée' }}
          </span>
        </div>

        <div v-if="!mfaEnabled">
          <p class="text-secondary">Activez le MFA pour renforcer la sécurité du compte.</p>
          <button class="btn btn-primary" @click="startSetup" :disabled="loading">
            {{ loading ? 'Chargement...' : 'Activer MFA' }}
          </button>
        </div>

        <div v-else>
          <p class="text-secondary">Le MFA est actif. Vous pouvez le désactiver si besoin.</p>
          <button class="btn btn-outline-danger" @click="showDisable = true">Désactiver le MFA</button>
        </div>

        <div v-if="setupVisible" class="mt-4">
          <div class="border rounded p-3">
            <div class="fw-semibold mb-2">Configuration MFA</div>
            <div class="text-secondary small mb-3">Scannez le QR code avec votre application d'authentification.</div>
            <div class="d-flex flex-column flex-md-row gap-3 align-items-center">
              <img :src="setup.qr_code" alt="QR Code" class="border rounded" style="width: 160px; height: 160px;" />
              <div class="flex-fill">
                <div class="text-secondary small mb-1">Cle secrete</div>
                <div class="bg-dark text-light rounded p-2 mb-3"><code>{{ setup.secret }}</code></div>
                <div class="mb-3">
                  <label class="form-label">Code TOTP</label>
                  <input v-model="verifyCode" type="text" class="form-control" placeholder="123456" inputmode="numeric" maxlength="6" />
                </div>
                <button class="btn btn-success" @click="verifySetup" :disabled="loading || !verifyCode">
                  {{ loading ? 'Vérification...' : 'Vérifier et activer' }}
                </button>
              </div>
            </div>

            <div v-if="setup.backup_codes?.length" class="mt-4">
              <div class="text-secondary small mb-1">Codes de secours</div>
              <pre class="bg-dark text-light rounded p-2 small">{{ setup.backup_codes.join('\n') }}</pre>
              <button class="btn btn-outline-light btn-sm" @click="copyBackupCodes">
                {{ copiedBackup ? 'Copie' : 'Copier les codes' }}
              </button>
            </div>
          </div>
        </div>

        <div v-if="showDisable" class="mt-4">
          <div class="border rounded p-3">
            <div class="fw-semibold mb-2">Désactiver le MFA</div>
            <div class="mb-3">
              <label class="form-label">Mot de passe</label>
              <input v-model="disablePassword" type="password" class="form-control" placeholder="••••••••" />
            </div>
            <button class="btn btn-danger" @click="disableMFA" :disabled="loading || !disablePassword">
              {{ loading ? 'Désactivation...' : 'Confirmer la désactivation' }}
            </button>
            <button class="btn btn-outline-secondary ms-2" @click="showDisable = false" :disabled="loading">Annuler</button>
          </div>
        </div>

        <div v-if="error" class="alert alert-danger mt-3" role="alert">{{ error }}</div>
        <div v-if="success" class="alert alert-success mt-3" role="alert">{{ success }}</div>
      </div>
    </div>

    <!-- Activity Tab -->
    <div v-if="tab === 'activity'">
      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between">
          <div class="card-title">Dernières connexions</div>
          <button class="btn btn-sm btn-outline-secondary" @click="loadLoginEvents" :disabled="activityLoading">
            <svg v-if="!activityLoading" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="me-1"><polyline points="23 4 23 10 17 10"></polyline><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path></svg>
            <span v-if="activityLoading" class="spinner-border spinner-border-sm me-1"></span>
            Actualiser
          </button>
        </div>
        <div class="card-body p-0">
          <div v-if="activityLoading && !loginEvents.length" class="text-center py-4 text-secondary">Chargement…</div>
          <div v-else-if="!loginEvents.length" class="text-center py-4 text-secondary">Aucun événement de connexion trouvé.</div>
          <div v-else class="table-responsive">
            <table class="table table-vcenter table-hover card-table mb-0">
              <thead>
                <tr>
                  <th>Date / heure</th>
                  <th>Adresse IP</th>
                  <th>Résultat</th>
                  <th>User agent</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="evt in loginEvents" :key="evt.id">
                  <td class="text-secondary small text-nowrap">{{ formatDate(evt.created_at) }}</td>
                  <td class="text-monospace small">{{ evt.ip_address || '—' }}</td>
                  <td>
                    <span v-if="evt.success" class="badge bg-green-lt text-green">Succès</span>
                    <span v-else class="badge bg-red-lt text-red">Échec</span>
                  </td>
                  <td class="text-secondary small" style="max-width:300px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" :title="evt.user_agent">
                    {{ evt.user_agent || '—' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- Threats Tab -->
    <div v-if="tab === 'threats'">
      <!-- Stats cards -->
      <div class="row row-cards mb-4">
        <div class="col-sm-4">
          <div class="card">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Connexions (24h)</div>
              <div class="h2 mb-0">{{ security.stats_24h?.total ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Échecs (24h)</div>
              <div class="h2 mb-0 text-danger">{{ security.stats_24h?.failures ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">IPs uniques (24h)</div>
              <div class="h2 mb-0">{{ security.stats_24h?.unique_ips ?? '—' }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards">
        <!-- Blocked IPs -->
        <div class="col-lg-5">
          <div class="card h-100">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title">IPs bloquées</h3>
              <button class="btn btn-sm btn-outline-secondary" @click="loadSecurity" :disabled="threatsLoading">
                <span v-if="threatsLoading" class="spinner-border spinner-border-sm"></span>
                <span v-else>↻</span>
              </button>
            </div>
            <div class="card-body p-0">
              <div v-if="threatsLoading && !security.blocked_ips?.length" class="text-center py-4 text-secondary">Chargement…</div>
              <div v-else-if="!security.blocked_ips?.length" class="text-center py-4 text-secondary small">
                Aucune IP bloquée actuellement
              </div>
              <div v-else>
                <div v-for="ip in security.blocked_ips" :key="ip" class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom">
                  <div class="d-flex align-items-center gap-2">
                    <span class="badge bg-red-lt text-red">Bloquée</span>
                    <span class="font-monospace small">{{ ip }}</span>
                  </div>
                  <button class="btn btn-sm btn-outline-success" @click="unblockIP(ip)" :disabled="unblockingIP === ip">
                    {{ unblockingIP === ip ? '...' : 'Débloquer' }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Top failed IPs -->
        <div class="col-lg-7">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title">Top 10 IPs — échecs de connexion (24h)</h3>
            </div>
            <div class="card-body p-0">
              <div v-if="!security.top_failed_ips?.length" class="text-center py-4 text-secondary small">
                Aucun échec enregistré sur cette période
              </div>
              <div v-else>
                <div v-for="item in security.top_failed_ips" :key="item.ip_address" class="px-3 py-2 border-bottom">
                  <div class="d-flex align-items-center justify-content-between mb-1">
                    <span class="font-monospace small">{{ item.ip_address }}</span>
                    <span class="badge bg-red-lt text-red">{{ item.fail_count }} échecs</span>
                  </div>
                  <div class="progress" style="height: 4px;">
                    <div
                      class="progress-bar bg-danger"
                      :style="{ width: progressWidth(item.fail_count) + '%' }"
                    ></div>
                  </div>
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
import { ref, computed, onMounted } from 'vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { formatDateTime as formatDate } from '../utils/formatters'

const auth = useAuthStore()
const isAdmin = computed(() => auth.user?.role === 'admin')

const tab = ref('mfa')

// MFA state
const mfaEnabled = ref(false)
const setupVisible = ref(false)
const setup = ref({ secret: '', qr_code: '', backup_codes: [] })
const verifyCode = ref('')
const disablePassword = ref('')
const showDisable = ref(false)
const loading = ref(false)
const error = ref('')
const success = ref('')
const copiedBackup = ref(false)

// Activity state
const loginEvents = ref([])
const activityLoading = ref(false)

// Threats state
const security = ref({ stats_24h: null, blocked_ips: [], top_failed_ips: [] })
const threatsLoading = ref(false)
const unblockingIP = ref('')

async function loadStatus() {
  try {
    const res = await apiClient.getMFAStatus()
    mfaEnabled.value = !!res.data?.mfa_enabled
  } catch (e) {
    mfaEnabled.value = false
  }
}

async function loadLoginEvents() {
  activityLoading.value = true
  try {
    const res = await apiClient.getLoginEvents()
    loginEvents.value = res.data?.events || []
  } catch (e) {
    loginEvents.value = []
  } finally {
    activityLoading.value = false
  }
}

async function loadSecurity() {
  threatsLoading.value = true
  try {
    const res = await apiClient.getSecuritySummary()
    security.value = res.data || { stats_24h: null, blocked_ips: [], top_failed_ips: [] }
  } catch (e) {
    console.error('Failed to load security summary:', e)
  } finally {
    threatsLoading.value = false
  }
}

function switchToActivity() {
  tab.value = 'activity'
  if (!loginEvents.value.length) loadLoginEvents()
}

function switchToThreats() {
  tab.value = 'threats'
  loadSecurity()
}

async function unblockIP(ip) {
  unblockingIP.value = ip
  try {
    await apiClient.unblockIP(ip)
    await loadSecurity()
  } catch (e) {
    console.error('Failed to unblock IP:', e)
  } finally {
    unblockingIP.value = ''
  }
}

function progressWidth(failCount) {
  const max = Math.max(...(security.value.top_failed_ips?.map(i => i.fail_count) || [1]))
  return max > 0 ? Math.round((failCount / max) * 100) : 0
}

async function startSetup() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await apiClient.setupMFA()
    setup.value = res.data
    setupVisible.value = true
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la configuration MFA'
  } finally {
    loading.value = false
  }
}

async function verifySetup() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    await apiClient.verifyMFA(setup.value.secret, verifyCode.value, setup.value.backup_codes)
    success.value = 'MFA activé avec succès.'
    setupVisible.value = false
    verifyCode.value = ''
    await loadStatus()
  } catch (e) {
    error.value = e.response?.data?.error || 'Code invalide'
  } finally {
    loading.value = false
  }
}

async function disableMFA() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    await apiClient.disableMFA(disablePassword.value)
    success.value = 'MFA désactivé.'
    showDisable.value = false
    disablePassword.value = ''
    await loadStatus()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la désactivation'
  } finally {
    loading.value = false
  }
}

async function copyBackupCodes() {
  if (!setup.value.backup_codes?.length) return
  await navigator.clipboard.writeText(setup.value.backup_codes.join('\n'))
  copiedBackup.value = true
  setTimeout(() => { copiedBackup.value = false }, 1500)
}

onMounted(loadStatus)
</script>

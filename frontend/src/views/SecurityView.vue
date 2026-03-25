<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link to="/" class="text-decoration-none">Dashboard</router-link>
          <span class="text-muted mx-1">/</span>
          <span>Sécurité hôtes</span>
        </div>
        <h2 class="page-title">Sécurité plateforme</h2>
        <div class="text-secondary">Menaces hôtes et sécurité du compte</div>
      </div>
    </div>

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'mfa' }" href="#" @click.prevent="tab = 'mfa'">
          Mon compte (MFA)
        </a>
      </li>
      <li class="nav-item" v-if="isAdmin">
        <a class="nav-link" :class="{ active: tab === 'threats' }" href="#" @click.prevent="switchToThreats">
          Sécurité hôtes
        </a>
      </li>
    </ul>

    <!-- MFA Tab -->
    <div v-if="tab === 'mfa'" class="card" style="max-width: 640px;">
      <div class="card-body">
        <div class="d-flex align-items-center justify-content-between mb-3">
          <div class="fw-semibold">Authentification multi-facteur</div>
          <span :class="mfaEnabled ? 'badge bg-green-lt text-green' : 'badge bg-orange-lt text-orange'">
            {{ mfaEnabled ? 'Activé' : 'Désactivé' }}
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

    <!-- Threats Tab -->
    <div v-if="tab === 'threats'">
      <!-- Period selector + refresh -->
      <div class="d-flex align-items-center justify-content-between mb-3">
        <div class="btn-group btn-group-sm">
          <button
            v-for="p in periodOptions"
            :key="p.hours"
            class="btn"
            :class="threatsPeriod === p.hours ? 'btn-primary' : 'btn-outline-secondary'"
            @click="setThreatsPeriod(p.hours)"
          >{{ p.label }}</button>
        </div>
        <button class="btn btn-sm btn-outline-secondary" @click="loadSecurity" :disabled="threatsLoading">
          <span v-if="threatsLoading" class="spinner-border spinner-border-sm"></span>
          <span v-else>↻</span>
        </button>
      </div>

      <!-- Stats cards -->
      <div class="row row-cards mb-4">
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Connexions ({{ periodLabel }})</div>
              <div class="h2 mb-0">{{ security.stats?.total ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Échecs ({{ periodLabel }})</div>
              <div class="h2 mb-0 text-danger">{{ security.stats?.failures ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">IPs uniques ({{ periodLabel }})</div>
              <div class="h2 mb-0">{{ security.stats?.unique_ips ?? '—' }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Requêtes suspectes (logs web)</div>
              <div class="h2 mb-0 text-orange">{{ security.bot_detection?.total_suspicious_requests ?? 0 }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">IPs suspectes (logs web)</div>
              <div class="h2 mb-0">{{ botTopIPs.length }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Hôtes impactés</div>
              <div class="h2 mb-0">{{ botHosts.length }}</div>
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
              <h3 class="card-title">Top 10 IPs — échecs de connexion ({{ periodLabel }})</h3>
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

      <div class="row row-cards mt-4">
        <div class="col-lg-7">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title">Top IPs suspectes (logs Nginx/Apache/NPM)</h3>
            </div>
            <div class="card-body p-0">
              <div v-if="!botTopIPs.length" class="text-center py-4 text-secondary small">
                Aucune activité suspecte détectée dans les logs web.
              </div>
              <div v-else>
                <div v-for="item in botTopIPs" :key="item.ip" class="px-3 py-2 border-bottom">
                  <div class="d-flex align-items-center justify-content-between mb-1">
                    <span class="font-monospace small">{{ item.ip }}</span>
                    <span class="badge bg-orange-lt text-orange">{{ item.hits }} hits</span>
                  </div>
                  <div class="d-flex align-items-center justify-content-between text-secondary small mb-1">
                    <span>Hosts: {{ item.host_count || 1 }}</span>
                    <span>Paths: {{ item.unique_paths || 0 }}</span>
                  </div>
                  <div class="progress" style="height: 4px;">
                    <div class="progress-bar bg-orange" :style="{ width: progressWidthBot(item.hits) + '%' }"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="col-lg-5">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title">Top chemins scannés</h3>
            </div>
            <div class="card-body p-0">
              <div v-if="!botTopPaths.length" class="text-center py-4 text-secondary small">
                Aucun chemin suspect détecté.
              </div>
              <div v-else>
                <div v-for="item in botTopPaths" :key="item.path" class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom">
                  <span class="font-monospace small text-truncate me-2" style="max-width: 70%;">{{ item.path }}</span>
                  <span class="badge bg-yellow-lt text-yellow">{{ item.hits }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card mt-4">
        <div class="card-header">
          <h3 class="card-title">Hôtes les plus ciblés (logs web)</h3>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Hôte</th>
                <th class="text-end">Requêtes suspectes</th>
                <th class="text-end">IPs suspectes</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!botHosts.length">
                <td colspan="3" class="text-center text-secondary py-4">Aucune donnée bot-detection remontée par les agents.</td>
              </tr>
              <tr v-for="item in botHosts" :key="item.host_id">
                <td>{{ item.host_name || item.host_id }}</td>
                <td class="text-end">{{ item.suspicious_requests || 0 }}</td>
                <td class="text-end">{{ item.unique_suspicious_ips || 0 }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'

const periodOptions = [
  { hours: 24,  label: '24h' },
  { hours: 168, label: '7j' },
  { hours: 720, label: '30j' },
]
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const isAdmin = computed(() => auth.user?.role === 'admin')
const route = useRoute()

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

// Threats state
const security = ref({ stats: null, blocked_ips: [], top_failed_ips: [] })
const threatsLoading = ref(false)
const unblockingIP = ref('')
const threatsPeriod = ref(24)
const periodLabel = computed(() => periodOptions.find(p => p.hours === threatsPeriod.value)?.label ?? '24h')
const botTopIPs = computed(() => security.value.bot_detection?.top_suspicious_ips || [])
const botTopPaths = computed(() => security.value.bot_detection?.top_suspicious_paths || [])
const botHosts = computed(() => security.value.bot_detection?.hosts || [])

async function loadStatus() {
  try {
    const res = await apiClient.getMFAStatus()
    mfaEnabled.value = !!res.data?.mfa_enabled
  } catch (e) {
    mfaEnabled.value = false
  }
}

async function loadSecurity() {
  threatsLoading.value = true
  try {
    const res = await apiClient.getSecuritySummary(threatsPeriod.value)
    security.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
  } catch (e) {
    console.error('Failed to load security summary:', e)
  } finally {
    threatsLoading.value = false
  }
}

function switchToThreats() {
  tab.value = 'threats'
  loadSecurity()
}

function setThreatsPeriod(hours) {
  threatsPeriod.value = hours
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

function progressWidthBot(hits) {
  const max = Math.max(...(botTopIPs.value.map(i => i.hits) || [1]))
  return max > 0 ? Math.round((hits / max) * 100) : 0
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

onMounted(async () => {
  await loadStatus()

  const requestedTab = String(route.query.tab || '').toLowerCase()
  if (requestedTab === 'mfa') {
    tab.value = 'mfa'
    return
  }
  if (requestedTab === 'threats' && isAdmin.value) {
    switchToThreats()
    return
  }
  if (isAdmin.value) {
    switchToThreats()
  }
})
</script>

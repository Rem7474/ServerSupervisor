<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Paramètres</h2>
      <div class="text-secondary">Configuration et diagnostics du système</div>
    </div>

    <!-- Configuration Actuelle -->
    <div class="row row-cards mb-4">
      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Configuration</h3>
          </div>
          <div class="card-body">
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">URL de base (Frontend)</div>
              <div class="font-monospace">{{ settings.baseUrl || 'Non configuré' }}</div>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Base de données</div>
              <div class="font-monospace">{{ settings.dbHost }}:{{ settings.dbPort }}</div>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Mode HTTPS/SSL</div>
              <span :class="settings.tlsEnabled ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
                {{ settings.tlsEnabled ? 'Activé' : 'Désactivé' }}
              </span>
            </div>
            <div class="mb-3">
              <div class="text-secondary small">Rétention des métriques</div>
              <div>{{ settings.metricsRetentionDays }} jours</div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">État de la base de données</h3>
          </div>
          <div class="card-body">
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Statut de la connexion</div>
              <span :class="dbStatus.connected ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
                {{ dbStatus.connected ? 'Connecté' : 'Déconnecté' }}
              </span>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Enregistrements audit</div>
              <div>{{ formatNumber(dbStatus.auditLogCount) }} entrées</div>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Métriques stockées</div>
              <div>{{ formatNumber(dbStatus.metricsCount) }} points</div>
            </div>
            <div class="mb-3">
              <div class="text-secondary small">Hôtes enregistrés</div>
              <div>{{ dbStatus.hostsCount }} hôtes</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Notifications & SMTP -->
    <div class="row row-cards mb-4">
      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Email (SMTP)</h3>
          </div>
          <div class="card-body">
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">SMTP Configuré</div>
              <span :class="settings.smtpConfigured ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
                {{ settings.smtpConfigured ? 'Activé' : 'Non configuré' }}
              </span>
            </div>
            <div v-if="settings.smtpConfigured" class="mb-3">
              <div class="text-secondary small">Serveur SMTP</div>
              <div class="font-monospace small">{{ settings.smtpHost }}:{{ settings.smtpPort }}</div>
            </div>
            <button 
              v-if="settings.smtpConfigured"
              class="btn btn-primary btn-sm mt-2"
              @click="testSmtp"
              :disabled="testingSmtp"
            >
              {{ testingSmtp ? 'Test en cours...' : 'Tester la connexion' }}
            </button>
            <div v-if="smtpTestMessage" :class="['alert mt-2', smtpTestSuccess ? 'alert-success' : 'alert-danger']">
              {{ smtpTestMessage }}
            </div>
          </div>
        </div>
      </div>

      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Webhooks</h3>
          </div>
          <div class="card-body">
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">ntfy.sh Webhook</div>
              <span :class="settings.ntfyUrl ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                {{ settings.ntfyUrl ? 'Configuré' : 'Non configuré' }}
              </span>
            </div>
            <div v-if="settings.ntfyUrl" class="mb-3">
              <div class="text-secondary small">URL</div>
              <div class="font-monospace small text-truncate">{{ settings.ntfyUrl }}</div>
            </div>
            <button 
              v-if="settings.ntfyUrl"
              class="btn btn-primary btn-sm mt-2"
              @click="testNtfy"
              :disabled="testingNtfy"
            >
              {{ testingNtfy ? 'Test en cours...' : 'Envoyer test' }}
            </button>
            <div v-if="ntfyTestMessage" :class="['alert mt-2', ntfyTestSuccess ? 'alert-success' : 'alert-danger']">
              {{ ntfyTestMessage }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Maintenance -->
    <div class="card">
      <div class="card-header">
        <h3 class="card-title">Maintenance</h3>
      </div>
      <div class="card-body">
        <div class="row g-3">
          <div class="col-md-6">
            <h4 class="text-sm mb-2">Nettoyage des métriques</h4>
            <p class="text-secondary small mb-3">
              Supprime les métriques brutes + agrégats plus anciens que {{ settings.metricsRetentionDays }} jours
            </p>
            <button 
              class="btn btn-warning btn-sm"
              @click="requestCleanMetrics"
              :disabled="cleaningMetrics"
            >
              {{ cleaningMetrics ? 'Nettoyage en cours...' : 'Lancer le nettoyage' }}
            </button>
            <div v-if="cleanMessage" :class="['alert alert-sm mt-2', cleanSuccess ? 'alert-success' : 'alert-danger']">
              {{ cleanMessage }}
            </div>
          </div>

          <div class="col-md-6">
            <h4 class="text-sm mb-2">Nettoyage des logs audit</h4>
            <p class="text-secondary small mb-3">
              Supprime les entrées audit plus anciennes que 90 jours (conformité)
            </p>
            <button 
              class="btn btn-warning btn-sm"
              @click="requestCleanAuditLogs"
              :disabled="cleaningAuditLogs"
            >
              {{ cleaningAuditLogs ? 'Nettoyage en cours...' : 'Lancer le nettoyage' }}
            </button>
            <div v-if="auditCleanMessage" :class="['alert alert-sm mt-2', auditCleanSuccess ? 'alert-success' : 'alert-danger']">
              {{ auditCleanMessage }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Confirmation Modale Cleanup Metrics -->
    <div v-if="showCleanMetricsModal" class="modal modal-blur fade show" style="display: block; background: rgba(0,0,0,0.5);">
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <button @click="showCleanMetricsModal = false" type="button" class="btn-close" aria-label="Close"></button>
          <div class="modal-status bg-warning"></div>
          <div class="modal-body text-center py-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 9m-6 0a6 6 0 1 0 12 0a6 6 0 1 0 -12 0" /><path d="M12 7v5" /><path d="M12 13v.01" /></svg>
            <h3>Confirmer le nettoyage des métriques</h3>
            <div class="text-secondary mb-3">Les métriques plus anciennes que {{ settings.metricsRetentionDays }} jours seront supprimées définitivement.</div>
          </div>
          <div class="modal-footer">
            <div class="w-100 d-flex gap-2">
              <button @click="showCleanMetricsModal = false" type="button" class="btn btn-link link-secondary w-100">Annuler</button>
              <button @click="cleanMetrics(); showCleanMetricsModal = false;" type="button" class="btn btn-warning w-100">Continuer</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Confirmation Modale Cleanup Audit Logs -->
    <div v-if="showCleanAuditModal" class="modal modal-blur fade show" style="display: block; background: rgba(0,0,0,0.5);">
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <button @click="showCleanAuditModal = false" type="button" class="btn-close" aria-label="Close"></button>
          <div class="modal-status bg-warning"></div>
          <div class="modal-body text-center py-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 9m-6 0a6 6 0 1 0 12 0a6 6 0 1 0 -12 0" /><path d="M12 7v5" /><path d="M12 13v.01" /></svg>
            <h3>Confirmer le nettoyage des logs audit</h3>
            <div class="text-secondary mb-3">Les entrées audit plus anciennes que 90 jours seront supprimées. Cette action est irréversible.</div>
          </div>
          <div class="modal-footer">
            <div class="w-100 d-flex gap-2">
              <button @click="showCleanAuditModal = false" type="button" class="btn btn-link link-secondary w-100">Annuler</button>
              <button @click="cleanAuditLogs(); showCleanAuditModal = false;" type="button" class="btn btn-warning w-100">Continuer</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'

const auth = useAuthStore()

const settings = ref({
  baseUrl: '',
  dbHost: '',
  dbPort: '',
  tlsEnabled: false,
  metricsRetentionDays: 7,
  smtpConfigured: false,
  smtpHost: '',
  smtpPort: '',
  ntfyUrl: '',
})

const dbStatus = ref({
  connected: false,
  auditLogCount: 0,
  metricsCount: 0,
  hostsCount: 0,
})

const testingSmtp = ref(false)
const smtpTestMessage = ref('')
const smtpTestSuccess = ref(false)

const testingNtfy = ref(false)
const ntfyTestMessage = ref('')
const ntfyTestSuccess = ref(false)

const cleaningMetrics = ref(false)
const cleanMessage = ref('')
const cleanSuccess = ref(false)

const cleaningAuditLogs = ref(false)
const auditCleanMessage = ref('')
const auditCleanSuccess = ref(false)

const showCleanMetricsModal = ref(false)
const showCleanAuditModal = ref(false)

function requestCleanMetrics() {
  showCleanMetricsModal.value = true
}

function requestCleanAuditLogs() {
  showCleanAuditModal.value = true
}

function formatNumber(n) {
  if (!n) return '0'
  return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ')
}

async function fetchSettings() {
  try {
    const res = await apiClient.getSettings()
    if (res.data) {
      settings.value = res.data.settings || {}
      dbStatus.value = res.data.dbStatus || {}
    }
  } catch (e) {
    console.error('Erreur lors du chargement des paramètres:', e)
  }
}

async function testSmtp() {
  testingSmtp.value = true
  smtpTestMessage.value = ''
  try {
    const res = await apiClient.testSmtp()
    smtpTestSuccess.value = true
    smtpTestMessage.value = 'Connexion SMTP réussie!'
    setTimeout(() => { smtpTestMessage.value = '' }, 5000)
  } catch (e) {
    smtpTestSuccess.value = false
    smtpTestMessage.value = `Erreur: ${e.response?.data?.error || e.message}`
    setTimeout(() => { smtpTestMessage.value = '' }, 5000)
  } finally {
    testingSmtp.value = false
  }
}

async function testNtfy() {
  testingNtfy.value = true
  ntfyTestMessage.value = ''
  try {
    const res = await apiClient.testNtfy()
    ntfyTestSuccess.value = true
    ntfyTestMessage.value = 'Message test envoyé à ntfy.sh!'
    setTimeout(() => { ntfyTestMessage.value = '' }, 5000)
  } catch (e) {
    ntfyTestSuccess.value = false
    ntfyTestMessage.value = `Erreur: ${e.response?.data?.error || e.message}`
    setTimeout(() => { ntfyTestMessage.value = '' }, 5000)
  } finally {
    testingNtfy.value = false
  }
}

async function cleanMetrics() {
  cleaningMetrics.value = true
  cleanMessage.value = ''
  try {
    const res = await apiClient.cleanupMetrics()
    cleanSuccess.value = true
    cleanMessage.value = res.data?.message || 'Nettoyage des métriques réussi'
    setTimeout(() => { cleanMessage.value = '' }, 5000)
    // Refresh DB status
    await fetchSettings()
  } catch (e) {
    cleanSuccess.value = false
    cleanMessage.value = `Erreur: ${e.response?.data?.error || e.message}`
    setTimeout(() => { cleanMessage.value = '' }, 5000)
  } finally {
    cleaningMetrics.value = false
  }
}

async function cleanAuditLogs() {
  cleaningAuditLogs.value = true
  auditCleanMessage.value = ''
  try {
    const res = await apiClient.cleanupAudit()
    auditCleanSuccess.value = true
    auditCleanMessage.value = res.data?.message || 'Nettoyage des logs audit réussi'
    setTimeout(() => { auditCleanMessage.value = '' }, 5000)
    // Refresh DB status
    await fetchSettings()
  } catch (e) {
    auditCleanSuccess.value = false
    auditCleanMessage.value = `Erreur: ${e.response?.data?.error || e.message}`
    setTimeout(() => { auditCleanMessage.value = '' }, 5000)
  } finally {
    cleaningAuditLogs.value = false
  }
}

onMounted(() => {
  fetchSettings()
})
</script>

<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Paramètres</h2>
      <div class="text-secondary">Configuration et diagnostics du système</div>
    </div>

    <!-- System Info + DB Status -->
    <div class="row row-cards mb-4">
      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Système</h3>
          </div>
          <div class="card-body">
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">URL de base</div>
              <div class="font-monospace">{{ settings.baseUrl || 'Non configuré' }}</div>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Base de données</div>
              <div class="font-monospace">{{ settings.dbHost }}:{{ settings.dbPort }}</div>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Mode HTTPS/TLS</div>
              <span :class="settings.tlsEnabled ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
                {{ settings.tlsEnabled ? 'Activé' : 'Désactivé' }}
              </span>
            </div>
            <div class="mb-3">
              <div class="text-secondary small">Version agent recommandée</div>
              <div class="font-monospace">{{ settings.latestAgentVersion || '—' }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Base de données</h3>
          </div>
          <div class="card-body">
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Connexion</div>
              <span :class="dbStatus.connected ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
                {{ dbStatus.connected ? 'Connectée' : 'Déconnectée' }}
              </span>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Logs audit</div>
              <div>{{ formatNumber(dbStatus.auditLogCount) }} entrées</div>
            </div>
            <div class="mb-3 pb-3 border-bottom">
              <div class="text-secondary small">Métriques stockées</div>
              <div>{{ formatNumber(dbStatus.metricsCount) }} points</div>
            </div>
            <div>
              <div class="text-secondary small">Hôtes enregistrés</div>
              <div>{{ dbStatus.hostsCount }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- SMTP -->
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Email (SMTP)</h3>
      </div>
      <div class="card-body">
        <div class="row g-3">
          <div class="col-md-8">
            <label class="form-label">Hôte SMTP</label>
            <input type="text" class="form-control" v-model="form.smtpHost" placeholder="mail.example.com">
          </div>
          <div class="col-md-4">
            <label class="form-label">Port</label>
            <input type="number" class="form-control" v-model.number="form.smtpPort" placeholder="587">
          </div>
          <div class="col-md-6">
            <label class="form-label">Utilisateur</label>
            <input type="text" class="form-control" v-model="form.smtpUser" placeholder="user@example.com" autocomplete="off">
          </div>
          <div class="col-md-6">
            <label class="form-label">Mot de passe</label>
            <div class="input-group">
              <input :type="showSmtpPass ? 'text' : 'password'" class="form-control" v-model="form.smtpPass" autocomplete="new-password">
              <button class="btn btn-outline-secondary" type="button" @click="showSmtpPass = !showSmtpPass">
                {{ showSmtpPass ? 'Masquer' : 'Afficher' }}
              </button>
            </div>
          </div>
          <div class="col-md-6">
            <label class="form-label">Expéditeur (From)</label>
            <input type="email" class="form-control" v-model="form.smtpFrom" placeholder="no-reply@example.com">
          </div>
          <div class="col-md-6">
            <label class="form-label">Destinataire (To)</label>
            <input type="email" class="form-control" v-model="form.smtpTo" placeholder="admin@example.com">
          </div>
          <div class="col-12">
            <label class="form-check">
              <input type="checkbox" class="form-check-input" v-model="form.smtpTls">
              <span class="form-check-label">TLS / STARTTLS activé</span>
            </label>
          </div>
        </div>
      </div>
      <div class="card-footer d-flex align-items-center gap-2">
        <button
          v-if="auth.user?.role === 'admin'"
          class="btn btn-primary"
          @click="saveSmtp"
          :disabled="savingSmtp"
        >
          {{ savingSmtp ? 'Enregistrement...' : 'Enregistrer SMTP' }}
        </button>
        <button
          class="btn btn-outline-secondary"
          @click="testSmtp"
          :disabled="testingSmtp || !form.smtpHost"
        >
          {{ testingSmtp ? 'Test en cours...' : 'Tester la connexion' }}
        </button>
        <span v-if="smtpSaveMsg" :class="['ms-auto small', smtpSaveOk ? 'text-success' : 'text-danger']">
          {{ smtpSaveMsg }}
        </span>
        <span v-if="smtpTestMessage" :class="['ms-auto small', smtpTestSuccess ? 'text-success' : 'text-danger']">
          {{ smtpTestMessage }}
        </span>
      </div>
    </div>

    <!-- Notifications + Retention -->
    <div class="row row-cards mb-4">
      <!-- Notifications -->
      <div class="col-lg-6">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">Notifications</h3>
          </div>
          <div class="card-body">
            <div class="mb-3">
              <label class="form-label">URL ntfy.sh</label>
              <input type="text" class="form-control" v-model="form.ntfyUrl" placeholder="https://ntfy.sh/mon-topic">
            </div>
            <div class="mb-0">
              <label class="form-label">GitHub Token</label>
              <div class="input-group">
                <input :type="showGitHubToken ? 'text' : 'password'" class="form-control" v-model="form.githubToken" placeholder="ghp_..." autocomplete="new-password">
                <button class="btn btn-outline-secondary" type="button" @click="showGitHubToken = !showGitHubToken">
                  {{ showGitHubToken ? 'Masquer' : 'Afficher' }}
                </button>
              </div>
              <div class="form-hint">Pour le suivi des releases GitHub</div>
            </div>
          </div>
          <div class="card-footer d-flex align-items-center gap-2">
            <button
              v-if="auth.user?.role === 'admin'"
              class="btn btn-primary"
              @click="saveNotifications"
              :disabled="savingNotif"
            >
              {{ savingNotif ? 'Enregistrement...' : 'Enregistrer' }}
            </button>
            <button
              class="btn btn-outline-secondary"
              @click="testNtfy"
              :disabled="testingNtfy || !form.ntfyUrl"
            >
              {{ testingNtfy ? 'Test...' : 'Tester ntfy' }}
            </button>
            <span v-if="notifSaveMsg" :class="['ms-auto small', notifSaveOk ? 'text-success' : 'text-danger']">
              {{ notifSaveMsg }}
            </span>
            <span v-if="ntfyTestMessage" :class="['ms-auto small', ntfyTestSuccess ? 'text-success' : 'text-danger']">
              {{ ntfyTestMessage }}
            </span>
          </div>
        </div>
      </div>

      <!-- Retention -->
      <div class="col-lg-6">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">Rétention des données</h3>
          </div>
          <div class="card-body">
            <div class="mb-3">
              <label class="form-label">Métriques (jours)</label>
              <input type="number" class="form-control" v-model.number="form.metricsRetentionDays" min="1" max="365">
              <div class="form-hint">Métriques brutes et agrégats plus anciens que ce seuil sont supprimés</div>
            </div>
            <div class="mb-0">
              <label class="form-label">Logs audit (jours)</label>
              <input type="number" class="form-control" v-model.number="form.auditRetentionDays" min="1" max="3650">
              <div class="form-hint">Entrées d'audit plus anciennes que ce seuil sont supprimées</div>
            </div>
          </div>
          <div class="card-footer d-flex align-items-center gap-2">
            <button
              v-if="auth.user?.role === 'admin'"
              class="btn btn-primary"
              @click="saveRetention"
              :disabled="savingRetention"
            >
              {{ savingRetention ? 'Enregistrement...' : 'Enregistrer' }}
            </button>
            <span v-if="retentionSaveMsg" :class="['ms-auto small', retentionSaveOk ? 'text-success' : 'text-danger']">
              {{ retentionSaveMsg }}
            </span>
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
              Supprime les entrées audit plus anciennes que {{ settings.auditRetentionDays }} jours
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

    <!-- Modal: Confirm metrics cleanup -->
    <div v-if="showCleanMetricsModal" class="modal modal-blur fade show" style="display: block; background: rgba(0,0,0,0.5);">
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <button @click="showCleanMetricsModal = false" type="button" class="btn-close" aria-label="Close"></button>
          <div class="modal-status bg-warning"></div>
          <div class="modal-body text-center py-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 9m-6 0a6 6 0 1 0 12 0a6 6 0 1 0 -12 0" /><path d="M12 7v5" /><path d="M12 13v.01" /></svg>
            <h3>Confirmer le nettoyage</h3>
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

    <!-- Modal: Confirm audit cleanup -->
    <div v-if="showCleanAuditModal" class="modal modal-blur fade show" style="display: block; background: rgba(0,0,0,0.5);">
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <button @click="showCleanAuditModal = false" type="button" class="btn-close" aria-label="Close"></button>
          <div class="modal-status bg-warning"></div>
          <div class="modal-body text-center py-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 9m-6 0a6 6 0 1 0 12 0a6 6 0 1 0 -12 0" /><path d="M12 7v5" /><path d="M12 13v.01" /></svg>
            <h3>Confirmer le nettoyage</h3>
            <div class="text-secondary mb-3">Les entrées audit plus anciennes que {{ settings.auditRetentionDays }} jours seront supprimées. Cette action est irréversible.</div>
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
  metricsRetentionDays: 30,
  auditRetentionDays: 90,
  smtpConfigured: false,
  smtpHost: '',
  smtpPort: 587,
  ntfyUrl: '',
  latestAgentVersion: '',
})

const dbStatus = ref({
  connected: false,
  auditLogCount: 0,
  metricsCount: 0,
  hostsCount: 0,
})

// Editable form state
const form = ref({
  smtpHost: '',
  smtpPort: 587,
  smtpUser: '',
  smtpPass: '',
  smtpFrom: '',
  smtpTo: '',
  smtpTls: true,
  ntfyUrl: '',
  githubToken: '',
  metricsRetentionDays: 30,
  auditRetentionDays: 90,
})

const showSmtpPass = ref(false)
const showGitHubToken = ref(false)

// SMTP save/test state
const savingSmtp = ref(false)
const smtpSaveMsg = ref('')
const smtpSaveOk = ref(false)
const testingSmtp = ref(false)
const smtpTestMessage = ref('')
const smtpTestSuccess = ref(false)

// Notifications save/test state
const savingNotif = ref(false)
const notifSaveMsg = ref('')
const notifSaveOk = ref(false)
const testingNtfy = ref(false)
const ntfyTestMessage = ref('')
const ntfyTestSuccess = ref(false)

// Retention save state
const savingRetention = ref(false)
const retentionSaveMsg = ref('')
const retentionSaveOk = ref(false)

// Maintenance state
const cleaningMetrics = ref(false)
const cleanMessage = ref('')
const cleanSuccess = ref(false)
const cleaningAuditLogs = ref(false)
const auditCleanMessage = ref('')
const auditCleanSuccess = ref(false)
const showCleanMetricsModal = ref(false)
const showCleanAuditModal = ref(false)

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
      const s = res.data.settings || {}
      form.value.smtpHost = s.smtpHost || ''
      form.value.smtpPort = s.smtpPort || 587
      form.value.smtpUser = s.smtpUser || ''
      form.value.smtpPass = s.smtpPass || ''
      form.value.smtpFrom = s.smtpFrom || ''
      form.value.smtpTo = s.smtpTo || ''
      form.value.smtpTls = s.smtpTls !== undefined ? s.smtpTls : true
      form.value.ntfyUrl = s.ntfyUrl || ''
      form.value.githubToken = s.githubToken || ''
      form.value.metricsRetentionDays = s.metricsRetentionDays || 30
      form.value.auditRetentionDays = s.auditRetentionDays || 90
    }
  } catch (e) {
    console.error('Erreur chargement paramètres:', e)
  }
}

async function saveSmtp() {
  savingSmtp.value = true
  smtpSaveMsg.value = ''
  try {
    await apiClient.updateSettings({
      smtp_host: form.value.smtpHost,
      smtp_port: form.value.smtpPort,
      smtp_user: form.value.smtpUser,
      smtp_pass: form.value.smtpPass,
      smtp_from: form.value.smtpFrom,
      smtp_to: form.value.smtpTo,
      smtp_tls: form.value.smtpTls,
    })
    smtpSaveOk.value = true
    smtpSaveMsg.value = 'Configuration SMTP enregistrée'
    await fetchSettings()
    setTimeout(() => { smtpSaveMsg.value = '' }, 4000)
  } catch (e) {
    smtpSaveOk.value = false
    smtpSaveMsg.value = `Erreur : ${e.response?.data?.error || e.message}`
    setTimeout(() => { smtpSaveMsg.value = '' }, 5000)
  } finally {
    savingSmtp.value = false
  }
}

async function saveNotifications() {
  savingNotif.value = true
  notifSaveMsg.value = ''
  try {
    await apiClient.updateSettings({
      ntfy_url: form.value.ntfyUrl,
      github_token: form.value.githubToken,
    })
    notifSaveOk.value = true
    notifSaveMsg.value = 'Notifications enregistrées'
    await fetchSettings()
    setTimeout(() => { notifSaveMsg.value = '' }, 4000)
  } catch (e) {
    notifSaveOk.value = false
    notifSaveMsg.value = `Erreur : ${e.response?.data?.error || e.message}`
    setTimeout(() => { notifSaveMsg.value = '' }, 5000)
  } finally {
    savingNotif.value = false
  }
}

async function saveRetention() {
  savingRetention.value = true
  retentionSaveMsg.value = ''
  try {
    await apiClient.updateSettings({
      metrics_retention_days: form.value.metricsRetentionDays,
      audit_retention_days: form.value.auditRetentionDays,
    })
    retentionSaveOk.value = true
    retentionSaveMsg.value = 'Rétention enregistrée'
    await fetchSettings()
    setTimeout(() => { retentionSaveMsg.value = '' }, 4000)
  } catch (e) {
    retentionSaveOk.value = false
    retentionSaveMsg.value = `Erreur : ${e.response?.data?.error || e.message}`
    setTimeout(() => { retentionSaveMsg.value = '' }, 5000)
  } finally {
    savingRetention.value = false
  }
}

async function testSmtp() {
  testingSmtp.value = true
  smtpTestMessage.value = ''
  try {
    await apiClient.testSmtp()
    smtpTestSuccess.value = true
    smtpTestMessage.value = 'Connexion SMTP réussie'
    setTimeout(() => { smtpTestMessage.value = '' }, 5000)
  } catch (e) {
    smtpTestSuccess.value = false
    smtpTestMessage.value = `Erreur : ${e.response?.data?.error || e.message}`
    setTimeout(() => { smtpTestMessage.value = '' }, 5000)
  } finally {
    testingSmtp.value = false
  }
}

async function testNtfy() {
  testingNtfy.value = true
  ntfyTestMessage.value = ''
  try {
    await apiClient.testNtfy()
    ntfyTestSuccess.value = true
    ntfyTestMessage.value = 'Message test envoyé'
    setTimeout(() => { ntfyTestMessage.value = '' }, 5000)
  } catch (e) {
    ntfyTestSuccess.value = false
    ntfyTestMessage.value = `Erreur : ${e.response?.data?.error || e.message}`
    setTimeout(() => { ntfyTestMessage.value = '' }, 5000)
  } finally {
    testingNtfy.value = false
  }
}

function requestCleanMetrics() {
  showCleanMetricsModal.value = true
}

function requestCleanAuditLogs() {
  showCleanAuditModal.value = true
}

async function cleanMetrics() {
  cleaningMetrics.value = true
  cleanMessage.value = ''
  try {
    const res = await apiClient.cleanupMetrics()
    cleanSuccess.value = true
    cleanMessage.value = res.data?.message || 'Nettoyage des métriques terminé'
    await fetchSettings()
    setTimeout(() => { cleanMessage.value = '' }, 5000)
  } catch (e) {
    cleanSuccess.value = false
    cleanMessage.value = `Erreur : ${e.response?.data?.error || e.message}`
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
    auditCleanMessage.value = res.data?.message || 'Nettoyage des logs audit terminé'
    await fetchSettings()
    setTimeout(() => { auditCleanMessage.value = '' }, 5000)
  } catch (e) {
    auditCleanSuccess.value = false
    auditCleanMessage.value = `Erreur : ${e.response?.data?.error || e.message}`
    setTimeout(() => { auditCleanMessage.value = '' }, 5000)
  } finally {
    cleaningAuditLogs.value = false
  }
}

onMounted(() => {
  fetchSettings()
})
</script>

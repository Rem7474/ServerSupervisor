<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>Paramètres</span>
      </div>
      <h2 class="page-title">Paramètres</h2>
    </div>

    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'general' }" href="#" @click.prevent="tab = 'general'">Général</a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'notifications' }" href="#" @click.prevent="tab = 'notifications'">Notifications</a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'integrations' }" href="#" @click.prevent="tab = 'integrations'">Intégrations</a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'retention' }" href="#" @click.prevent="tab = 'retention'">Rétention</a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: tab === 'maintenance' }" href="#" @click.prevent="tab = 'maintenance'">Maintenance</a>
      </li>
    </ul>

    <!-- Général -->
    <div v-show="tab === 'general'" class="row row-cards">
      <div class="col-lg-6">
        <SettingsSystemInfoCard :settings="settings" />
      </div>
      <div class="col-lg-6">
        <SettingsDatabaseCard :db-status="dbStatus" :format-number="formatNumber" />
      </div>
    </div>

    <!-- Notifications -->
    <div v-show="tab === 'notifications'">
      <SettingsSmtpCard
        :form="form"
        :auth-is-admin="auth.isAdmin"
        :show-smtp-pass="showSmtpPass"
        :saving-smtp="savingSmtp"
        :smtp-save-msg="smtpSaveMsg"
        :smtp-save-ok="smtpSaveOk"
        :testing-smtp="testingSmtp"
        :smtp-test-message="smtpTestMessage"
        :smtp-test-success="smtpTestSuccess"
        @update:show-smtp-pass="showSmtpPass = $event"
        @save="saveSmtp"
        @test="testSmtp"
      />
      <div class="mt-4">
        <SettingsNotificationsCard
          :form="form"
          :auth-is-admin="auth.isAdmin"
          :show-git-hub-token="showGitHubToken"
          :saving-notif="savingNotif"
          :notif-save-msg="notifSaveMsg"
          :notif-save-ok="notifSaveOk"
          :testing-ntfy="testingNtfy"
          :ntfy-test-message="ntfyTestMessage"
          :ntfy-test-success="ntfyTestSuccess"
          @update:show-github-token="showGitHubToken = $event"
          @save="saveNotifications"
          @test="testNtfy"
        />
      </div>
    </div>

    <!-- Intégrations -->
    <div v-show="tab === 'integrations'">
      <SettingsProxmoxCard :auth-is-admin="auth.isAdmin" />
    </div>

    <!-- Rétention -->
    <div v-show="tab === 'retention'">
      <SettingsRetentionCard
        :form="form"
        :auth-is-admin="auth.isAdmin"
        :saving-retention="savingRetention"
        :retention-save-msg="retentionSaveMsg"
        :retention-save-ok="retentionSaveOk"
        @save="saveRetention"
      />
    </div>

    <!-- Maintenance -->
    <div v-show="tab === 'maintenance'">
      <SettingsMaintenanceCard
        :settings="settings"
        :cleaning-metrics="cleaningMetrics"
        :clean-message="cleanMessage"
        :clean-success="cleanSuccess"
        :cleaning-audit-logs="cleaningAuditLogs"
        :audit-clean-message="auditCleanMessage"
        :audit-clean-success="auditCleanSuccess"
        @clean-metrics="cleanMetrics"
        @clean-audit="cleanAuditLogs"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient, { getApiErrorMessage } from '../api'
import SettingsDatabaseCard from '../components/settings/SettingsDatabaseCard.vue'
import SettingsMaintenanceCard from '../components/settings/SettingsMaintenanceCard.vue'
import SettingsNotificationsCard from '../components/settings/SettingsNotificationsCard.vue'
import SettingsRetentionCard from '../components/settings/SettingsRetentionCard.vue'
import SettingsSmtpCard from '../components/settings/SettingsSmtpCard.vue'
import SettingsSystemInfoCard from '../components/settings/SettingsSystemInfoCard.vue'
import SettingsProxmoxCard from '../components/settings/SettingsProxmoxCard.vue'

const auth = useAuthStore()

const tab = ref('general')

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
    smtpSaveMsg.value = `Erreur : ${getApiErrorMessage(e)}`
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
    notifSaveMsg.value = `Erreur : ${getApiErrorMessage(e)}`
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
    retentionSaveMsg.value = `Erreur : ${getApiErrorMessage(e)}`
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
    smtpTestMessage.value = `Erreur : ${getApiErrorMessage(e)}`
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
    ntfyTestMessage.value = `Erreur : ${getApiErrorMessage(e)}`
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
    cleanMessage.value = res.data?.message || 'Nettoyage des métriques terminé'
    await fetchSettings()
    setTimeout(() => { cleanMessage.value = '' }, 5000)
  } catch (e) {
    cleanSuccess.value = false
    cleanMessage.value = `Erreur : ${getApiErrorMessage(e)}`
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
    auditCleanMessage.value = `Erreur : ${getApiErrorMessage(e)}`
    setTimeout(() => { auditCleanMessage.value = '' }, 5000)
  } finally {
    cleaningAuditLogs.value = false
  }
}

onMounted(() => {
  fetchSettings()
})
</script>

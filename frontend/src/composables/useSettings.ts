import { ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import apiClient, { getApiErrorMessage } from '../api'
import { isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'

export function useSettings() {
  const route = useRoute()
  const router = useRouter()
  const auth = useAuthStore()
  const signal = useAbortSignal()

  const tab = ref((route.query.tab as string) || 'general')

  watch(tab, (t) => {
    router.replace({ query: { ...route.query, tab: t } })
  })

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
    // Threat-detection weights — defaults mirror threatdetect.DefaultWeights()
    // server-side; overwritten by fetchSettings() once the real config loads.
    threatWeightWordpress: 2,
    threatWeightAdminpanel: 3,
    threatWeightPathtraversal: 5,
    threatWeightKnownscanner: 4,
    threatWeightSuspiciousmethod: 2,
    threatWeightStatus2xx: 0.1,
    threatWeightStatus3xx: 1,
    threatWeightStatus404: 2,
    threatWeightStatus4xx: 1.5,
    threatWeightStatus5xx: 3,
    threatWeightBreadth: 3,
    threatWeightHits: 2,
    threatThresholdMedium: 15,
    threatThresholdHigh: 50,
    threatThresholdCritical: 150,
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

  // Threat-detection weights save state
  const savingThreatDetection = ref(false)
  const threatDetectionSaveMsg = ref('')
  const threatDetectionSaveOk = ref(false)

  // Maintenance state
  const cleaningMetrics = ref(false)
  const cleanMessage = ref('')
  const cleanSuccess = ref(false)
  const cleaningAuditLogs = ref(false)
  const auditCleanMessage = ref('')
  const auditCleanSuccess = ref(false)

  function formatNumber(n: number | undefined): string {
    if (!n) return '0'
    return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ')
  }

  async function fetchSettings(): Promise<void> {
    try {
      const res = await apiClient.getSettings(signal)
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
        form.value.threatWeightWordpress = s.threatWeightWordPress ?? 2
        form.value.threatWeightAdminpanel = s.threatWeightAdminPanel ?? 3
        form.value.threatWeightPathtraversal = s.threatWeightPathTraversal ?? 5
        form.value.threatWeightKnownscanner = s.threatWeightKnownScanner ?? 4
        form.value.threatWeightSuspiciousmethod = s.threatWeightSuspiciousMethod ?? 2
        form.value.threatWeightStatus2xx = s.threatWeightStatus2xx ?? 0.1
        form.value.threatWeightStatus3xx = s.threatWeightStatus3xx ?? 1
        form.value.threatWeightStatus404 = s.threatWeightStatus404 ?? 2
        form.value.threatWeightStatus4xx = s.threatWeightStatus4xx ?? 1.5
        form.value.threatWeightStatus5xx = s.threatWeightStatus5xx ?? 3
        form.value.threatWeightBreadth = s.threatWeightBreadth ?? 3
        form.value.threatWeightHits = s.threatWeightHits ?? 2
        form.value.threatThresholdMedium = s.threatThresholdMedium ?? 15
        form.value.threatThresholdHigh = s.threatThresholdHigh ?? 50
        form.value.threatThresholdCritical = s.threatThresholdCritical ?? 150
      }
    } catch (e) {
      if (isApiAbort(e)) return
      console.error('Erreur chargement paramètres:', getApiErrorMessage(e))
    }
  }

  async function saveSmtp(): Promise<void> {
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

  async function saveNotifications(): Promise<void> {
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

  async function saveRetention(): Promise<void> {
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

  async function saveThreatDetection(): Promise<void> {
    savingThreatDetection.value = true
    threatDetectionSaveMsg.value = ''
    try {
      await apiClient.updateSettings({
        threat_weight_wordpress: form.value.threatWeightWordpress,
        threat_weight_adminpanel: form.value.threatWeightAdminpanel,
        threat_weight_pathtraversal: form.value.threatWeightPathtraversal,
        threat_weight_knownscanner: form.value.threatWeightKnownscanner,
        threat_weight_suspiciousmethod: form.value.threatWeightSuspiciousmethod,
        threat_weight_status_2xx: form.value.threatWeightStatus2xx,
        threat_weight_status_3xx: form.value.threatWeightStatus3xx,
        threat_weight_status_404: form.value.threatWeightStatus404,
        threat_weight_status_4xx: form.value.threatWeightStatus4xx,
        threat_weight_status_5xx: form.value.threatWeightStatus5xx,
        threat_weight_breadth: form.value.threatWeightBreadth,
        threat_weight_hits: form.value.threatWeightHits,
        threat_threshold_medium: form.value.threatThresholdMedium,
        threat_threshold_high: form.value.threatThresholdHigh,
        threat_threshold_critical: form.value.threatThresholdCritical,
      })
      threatDetectionSaveOk.value = true
      threatDetectionSaveMsg.value = 'Score de menace enregistré'
      await fetchSettings()
      setTimeout(() => { threatDetectionSaveMsg.value = '' }, 4000)
    } catch (e) {
      threatDetectionSaveOk.value = false
      threatDetectionSaveMsg.value = `Erreur : ${getApiErrorMessage(e)}`
      setTimeout(() => { threatDetectionSaveMsg.value = '' }, 5000)
    } finally {
      savingThreatDetection.value = false
    }
  }

  async function testSmtp(): Promise<void> {
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

  async function testNtfy(): Promise<void> {
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

  async function cleanMetrics(): Promise<void> {
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

  async function cleanAuditLogs(): Promise<void> {
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

  return {
    auth,
    tab,
    settings,
    dbStatus,
    form,
    showSmtpPass,
    showGitHubToken,
    savingSmtp,
    smtpSaveMsg,
    smtpSaveOk,
    testingSmtp,
    smtpTestMessage,
    smtpTestSuccess,
    savingNotif,
    notifSaveMsg,
    notifSaveOk,
    testingNtfy,
    ntfyTestMessage,
    ntfyTestSuccess,
    savingRetention,
    retentionSaveMsg,
    retentionSaveOk,
    savingThreatDetection,
    threatDetectionSaveMsg,
    threatDetectionSaveOk,
    cleaningMetrics,
    cleanMessage,
    cleanSuccess,
    cleaningAuditLogs,
    auditCleanMessage,
    auditCleanSuccess,
    formatNumber,
    fetchSettings,
    saveSmtp,
    saveNotifications,
    saveRetention,
    saveThreatDetection,
    testSmtp,
    testNtfy,
    cleanMetrics,
    cleanAuditLogs,
  }
}

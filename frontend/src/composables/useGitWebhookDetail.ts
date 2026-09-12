import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import { useCommandStream } from './useCommandStream'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { GitWebhook, GitWebhookExecution, GitWebhookRequest } from '../types/webhook'
import type { Host } from '../types/host'
import type { WebhookFormData } from './useWebhookForm'

interface CmdRow { id: string; status?: string; output?: string; [key: string]: unknown }

export function useGitWebhookDetail() {
  const { t } = useI18n()
  const route = useRoute()
  const id = route.params.id as string
  const signal = useAbortSignal()

  const webhook = ref<GitWebhook | null>(null)
  const executions = ref<GitWebhookExecution[]>([])
  const hosts = ref<Host[]>([])
  const loading = ref(false)
  const error = ref('')
  const revealedSecret = ref('')
  const selectedCmd = ref<CmdRow | null>(null)
  const showConsole = ref(false)

  const showModal = ref(false)
  const saving = ref(false)
  const modalError = ref('')
  const { openCommandStream, closeStream } = useCommandStream()

  const envVarKeys = [
    { name: 'SS_REPO_NAME', descKey: 'webhooks.repoNameFullDesc' },
    { name: 'SS_BRANCH', descKey: 'webhooks.branchTriggerDesc' },
    { name: 'SS_COMMIT_SHA', descKey: 'webhooks.commitShaDesc' },
    { name: 'SS_COMMIT_MESSAGE', descKey: 'webhooks.commitMessageFirstLineDesc' },
    { name: 'SS_PUSHER', descKey: 'webhooks.pusherUsernameDesc' },
    { name: 'SS_WEBHOOK_NAME', descKey: 'webhooks.webhookNameDesc' },
    { name: 'SS_EVENT_TYPE', descKey: 'webhooks.eventTypeDesc' },
  ]
  const envVars = computed(() => envVarKeys.map((v) => ({ name: v.name, desc: t(v.descKey) })))

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const [whRes, hostsRes] = await Promise.all([api.getGitWebhook(id, signal), api.getHosts(signal)])
      webhook.value = whRes.data.webhook
      executions.value = whRes.data.executions || []
      hosts.value = hostsRes.data || []
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, t('webhooks.loadErrorGeneric'))
    } finally {
      loading.value = false
    }
  }

  async function loadExecutions(): Promise<void> {
    try {
      const res = await api.getWebhookExecutions(id)
      executions.value = res.data.executions || []
    } catch { /* ignore */ }
  }

  function clearExecutionLogs(): void {
    closeStream()
    selectedCmd.value = null
    showConsole.value = false
  }

  function connectExecutionStream(commandId: string): void {
    openCommandStream(commandId, {
      onInit(payload) {
        if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
        selectedCmd.value = {
          ...selectedCmd.value,
          status: payload.status || selectedCmd.value.status,
          output: payload.output ?? selectedCmd.value.output,
        }
      },
      onChunk(payload) {
        if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
        selectedCmd.value = {
          ...selectedCmd.value,
          output: (selectedCmd.value.output || '') + (payload.chunk || ''),
        }
      },
      onStatus(payload) {
        if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
        selectedCmd.value = {
          ...selectedCmd.value,
          status: payload.status || selectedCmd.value.status,
          output: payload.output ?? selectedCmd.value.output,
        }

        const idx = executions.value.findIndex((e: GitWebhookExecution) => e.command_id === commandId)
        if (idx !== -1) {
          const next = [...executions.value]
          next[idx] = { ...next[idx], status: payload.status || next[idx].status }
          executions.value = next
        }
      },
    })
  }

  async function openExecutionLogs(commandId: string): Promise<void> {
    closeStream()
    try {
      const res = await api.getCommandStatus(commandId)
      const cmd = res.data
      selectedCmd.value = cmd as unknown as CmdRow
      showConsole.value = true
      if (cmd?.status === 'pending' || cmd?.status === 'running') {
        connectExecutionStream(commandId)
      }
    } catch {
      error.value = t('webhooks.couldNotLoadCommandLogsError')
    }
  }

  function openEdit(): void {
    modalError.value = ''
    showModal.value = true
  }

  async function saveEdit(payload: WebhookFormData): Promise<void> {
    saving.value = true
    modalError.value = ''
    try {
      await api.updateGitWebhook(id, payload as unknown as GitWebhookRequest)
      closeEdit()
      await load()
    } catch (e: unknown) {
      modalError.value = getApiErrorMessage(e, t('common.error'))
    } finally {
      saving.value = false
    }
  }

  function closeEdit(): void {
    showModal.value = false
    modalError.value = ''
  }

  function onSecretRegenerated(secret: string): void {
    revealedSecret.value = secret
  }

  function providerBadge(provider: string): string {
    const map: Record<string, string> = {
      github:  'bg-blue-lt text-blue',
      gitlab:  'bg-orange-lt text-orange',
      gitea:   'bg-teal-lt text-teal',
      forgejo: 'bg-purple-lt text-purple',
      custom:  'bg-secondary-lt text-secondary',
    }
    return map[provider] || 'bg-secondary-lt text-secondary'
  }

  function channelBadge(ch: string): string {
    const map: Record<string, string> = {
      smtp:    'bg-blue-lt text-blue',
      ntfy:    'bg-orange-lt text-orange',
      browser: 'bg-purple-lt text-purple',
    }
    return map[ch] || 'bg-secondary-lt text-secondary'
  }

  onMounted(load)
  onUnmounted(() => {
    closeStream()
  })

  return {
    id,
    webhook,
    executions,
    hosts,
    loading,
    error,
    revealedSecret,
    selectedCmd,
    showConsole,
    showModal,
    saving,
    modalError,
    envVars,
    formatDateTime,
    loadExecutions,
    clearExecutionLogs,
    openExecutionLogs,
    openEdit,
    saveEdit,
    closeEdit,
    onSecretRegenerated,
    providerBadge,
    channelBadge,
  }
}

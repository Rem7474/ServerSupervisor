import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import { formatDateLong as formatDate, formatDateTime } from '../utils/formatters'
import { useCommandStream } from './useCommandStream'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { RemoteCommand } from '../types/generated'
import { useStatusBadge } from './useStatusBadge'
import { moduleLabel, moduleClass } from '../utils/moduleMeta'

// The audit/commands endpoint enriches RemoteCommand with the resolved host name.
type CommandRow = RemoteCommand & { host_name?: string }

interface Profile {
  username?: string
  role?: string
  created_at?: string
  mfa_enabled?: boolean
  [key: string]: unknown
}

export function useAccount() {
  const auth = useAuthStore()

  const signal = useAbortSignal()
  const activeTab = ref('profil')
  const showConsole = ref(false)

  const profile = ref<Profile | null>(null)

  const pwForm = ref({ current: '', next: '', confirm: '' })
  const pwErrors = ref({ current: '', next: '', confirm: '' })
  const pwError = ref('')
  const pwSuccess = ref('')
  const pwLoading = ref(false)

  const pwStrength = computed(() => {
    const pw = pwForm.value.next
    if (!pw) return 0
    let score = 0
    if (pw.length >= 8) score++
    if (pw.length >= 12) score++
    if (/[A-Z]/.test(pw)) score++
    if (/[0-9]/.test(pw)) score++
    if (/[^A-Za-z0-9]/.test(pw)) score++
    return score
  })

  const pwStrengthMeta = computed(() => {
    if (!pwForm.value.next) return null
    const s = pwStrength.value
    if (s <= 1) return { label: 'Faible', cls: 'bg-danger', width: '25%' }
    if (s <= 2) return { label: 'Moyen', cls: 'bg-warning', width: '50%' }
    if (s <= 3) return { label: 'Bon', cls: 'bg-info', width: '75%' }
    return { label: 'Fort', cls: 'bg-success', width: '100%' }
  })

  watch(() => pwForm.value.next, (val) => {
    if (val.length > 0 && val.length < 8) {
      pwErrors.value.next = 'Au moins 8 caractères requis.'
    } else {
      pwErrors.value.next = ''
    }
    if (pwForm.value.confirm && val !== pwForm.value.confirm) {
      pwErrors.value.confirm = 'La confirmation ne correspond pas.'
    } else if (pwForm.value.confirm) {
      pwErrors.value.confirm = ''
    }
  })

  watch(() => pwForm.value.confirm, (val) => {
    if (val && val !== pwForm.value.next) {
      pwErrors.value.confirm = 'La confirmation ne correspond pas.'
    } else {
      pwErrors.value.confirm = ''
    }
  })

  // Commands history
  const allCommands = ref<CommandRow[]>([])
  const cmdsLoading = ref(false)
  const myCommands = computed(() =>
    allCommands.value.filter((c: CommandRow) => c.triggered_by === auth.username).slice(0, 50)
  )

  const selectedCmd = ref<CommandRow | null>(null)

  const { openCommandStream, closeStream } = useCommandStream()
  const { getStatusBadgeClass } = useStatusBadge()

  const roleBadgeClass = computed(() => {
    const map: Record<string, string> = { admin: 'bg-red-lt text-red', operator: 'bg-yellow-lt text-yellow', viewer: 'bg-secondary-lt text-secondary' }
    return map[profile.value?.role ?? ""] || 'bg-secondary-lt text-secondary'
  })

  const roleLabel = computed(() => {
    const map: Record<string, string> = { admin: 'Administrateur', operator: 'Opérateur', viewer: 'Lecteur' }
    return map[profile.value?.role ?? ""] || profile.value?.role || auth.role
  })

  function formatDuration(startedAt: string | undefined, endedAt: string | undefined): string {
    if (!startedAt || !endedAt) return '—'
    const diff = Math.max(0, Math.round((new Date(endedAt).getTime() - new Date(startedAt).getTime()) / 1000))
    if (diff < 60) return `${diff}s`
    const m = Math.floor(diff / 60), s = diff % 60
    return s > 0 ? `${m}m ${s}s` : `${m}m`
  }

  function cmdLabel(cmd: CommandRow): string {
    return [cmd?.action, cmd?.target].filter(Boolean).join(' ')
  }

  function statusClass(status: string | undefined): string {
    return getStatusBadgeClass(status, 'badge bg-warning-lt text-warning')
  }

  function openLogViewer(cmd: CommandRow): void {
    if (selectedCmd.value?.id === cmd.id) { closeLogViewer(); return }
    closeLogViewer()
    selectedCmd.value = { ...cmd }
    showConsole.value = true
    if (cmd.status === 'running' || cmd.status === 'pending') connectStream(cmd.id)
  }

  function closeLogViewer(): void {
    closeStream()
    selectedCmd.value = null
    showConsole.value = false
  }

  function connectStream(commandId: string): void {
    openCommandStream(commandId, {
      onInit(p) {
        if (selectedCmd.value) { selectedCmd.value.status = p.status; selectedCmd.value.output = p.output || '' }
      },
      onChunk(p) {
        if (selectedCmd.value) selectedCmd.value.output = (selectedCmd.value.output || '') + p.chunk
      },
      onStatus(p) {
        if (selectedCmd.value) { selectedCmd.value.status = p.status; if (p.output) selectedCmd.value.output = p.output }
      },
    })
  }

  function resetPwForm(): void {
    pwForm.value = { current: '', next: '', confirm: '' }
    pwErrors.value = { current: '', next: '', confirm: '' }
    pwError.value = ''
    pwSuccess.value = ''
  }

  async function submitChangePassword(): Promise<void> {
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
    } catch (e: unknown) {
      pwError.value = getApiErrorMessage(e, 'Erreur lors de la mise à jour du mot de passe.')
    } finally {
      pwLoading.value = false
    }
  }

  async function loadProfile(): Promise<void> {
    try {
      const { data } = await apiClient.getProfile(signal)
      profile.value = data
    } catch { /* fallback to store data */ }
  }

  async function loadMyCommands(): Promise<void> {
    cmdsLoading.value = true
    try {
      const res = await apiClient.getCommandsHistory(1, 100, undefined, signal)
      allCommands.value = res.data?.commands || []
    } catch (e) {
      if (isApiAbort(e)) return
      allCommands.value = []
    } finally {
      cmdsLoading.value = false
    }
  }

  function switchToHistorique() {
    activeTab.value = 'historique'
    if (!allCommands.value.length && !cmdsLoading.value) loadMyCommands()
  }

  onMounted(() => {
    loadProfile()
    loadMyCommands()
  })

  onUnmounted(() => { closeStream() })

  return {
    auth,
    activeTab,
    showConsole,
    profile,
    pwForm,
    pwErrors,
    pwError,
    pwSuccess,
    pwLoading,
    pwStrength,
    pwStrengthMeta,
    allCommands,
    cmdsLoading,
    myCommands,
    selectedCmd,
    roleBadgeClass,
    roleLabel,
    formatDate,
    formatDateTime,
    formatDuration,
    cmdLabel,
    statusClass,
    moduleLabel,
    moduleClass,
    openLogViewer,
    closeLogViewer,
    resetPwForm,
    submitChangePassword,
    switchToHistorique,
  }
}

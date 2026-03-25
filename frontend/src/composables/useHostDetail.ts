import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import apiClient from '../api'
import { useHostCommandConsole } from './useHostCommandConsole'
import { useCommandStream } from './useCommandStream'
import { useConfirmDialog } from './useConfirmDialog'
import { useWebSocket } from './useWebSocket'
import { useAuthStore } from '../stores/auth'

type AnyRecord = Record<string, any>

export function useHostDetail() {
  const route = useRoute()
  const router = useRouter()
  const hostId = String(route.params.id)

  const auth = useAuthStore()
  const dialog = useConfirmDialog()
  const canRunApt = computed(() => auth.canManage)

  const activeTab = ref('metrics')
  const isEditing = ref(false)
  const tasksCount = ref(0)
  const aptCmdLoading = ref('')

  const host = ref<AnyRecord | null>(null)
  const metrics = ref<AnyRecord | null>(null)
  const containers = ref<AnyRecord[]>([])
  const versionComparisons = ref<AnyRecord[]>([])
  const aptStatus = ref<AnyRecord | null>(null)
  const cmdHistory = ref<AnyRecord[]>([])
  const diskMetrics = ref<AnyRecord | null>(null)
  const diskHealth = ref<AnyRecord | null>(null)
  const latestAgentVersion = ref('')

  const proxmoxLink = ref<AnyRecord | null>(null)
  const linkSaving = ref(false)

  const effectiveMetrics = computed(() => {
    const m = metrics.value
    const link = proxmoxLink.value
    if (!m || !link || link.status !== 'confirmed') return m

    const src = link.metrics_source ?? 'auto'
    const useProxmox = src === 'proxmox' || (src === 'auto' && (link.mem_alloc ?? 0) > 0)

    if (!useProxmox) return m

    const cpuPct = (link.cpu_usage ?? 0) * 100
    const memUsed = link.mem_usage ?? 0
    const memTotal = link.mem_alloc ?? 0
    return {
      ...m,
      cpu_usage_percent: cpuPct,
      memory_used: memUsed,
      memory_total: memTotal,
      memory_percent: memTotal > 0 ? (memUsed / memTotal) * 100 : 0,
    }
  })

  const effectiveMetricsSource = computed(() => {
    const link = proxmoxLink.value
    if (!link || link.status !== 'confirmed') return 'agent'
    const src = link.metrics_source ?? 'auto'
    if (src === 'proxmox') return 'proxmox'
    if (src === 'auto' && (link.mem_alloc ?? 0) > 0) return 'proxmox'
    return 'agent'
  })
  const showLinkForm = ref(false)
  const showLinkButton = ref(false)
  const linkCandidates = ref<AnyRecord[]>([])
  const linkCandidatesLoading = ref(false)
  const selectedCandidate = ref('')

  const { liveCommand, showConsole, openCommand: _openCommand, closeConsole, updateCommand } = useHostCommandConsole()
  const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })

  function openCommand(rawCmd: AnyRecord) {
    _openCommand({ ...rawCmd, host_name: host.value?.hostname })
  }

  function connectStream(commandId: string) {
    openCommandStream(commandId, {
      onInit: (p: AnyRecord) => {
        const current = liveCommand.value
        if (!current) return
        updateCommand({ ...current, status: p.status, output: p.output || '' })
        nextTick(() => {})
      },
      onChunk: (p: AnyRecord) => {
        const current = liveCommand.value
        if (!current) return
        updateCommand({ ...current, output: (current.output || '') + (p.chunk || '') })
      },
      onStatus: (p: AnyRecord) => {
        const current = liveCommand.value
        if (!current) return
        updateCommand({ ...current, status: p.status })
        if (p.status === 'completed' || p.status === 'failed') {
          loadCmdHistoryRefresh()
        }
      },
    })
  }

  watch(
    () => liveCommand.value?.id,
    (id: string | number | undefined) => {
      if (!id || !showConsole.value) return
      connectStream(String(id))
    }
  )

  watch(showConsole, (show) => {
    if (!show) {
      closeStream()
    } else if (liveCommand.value?.id) {
      connectStream(String(liveCommand.value.id))
    }
  })

  function closeConsoleAndStream() {
    closeStream()
    closeConsole()
  }

  function clearConsoleOutput() {
    if (liveCommand.value) updateCommand({ ...liveCommand.value, output: '' })
  }

  const { wsStatus, wsError, retryCount, reconnect } = useWebSocket(
    `/api/v1/ws/hosts/${hostId}`,
    (payload: AnyRecord) => {
      if (payload.type !== 'host_detail') return
      host.value = payload.host
      metrics.value = payload.metrics
      containers.value = payload.containers || []
      versionComparisons.value = payload.version_comparisons || []
      aptStatus.value = payload.apt_status
      if ('proxmox_link' in payload) {
        proxmoxLink.value = payload.proxmox_link
      }
    },
    { debounceMs: 200 }
  )

  async function sendAptCmd(command: string) {
    const confirmed = await dialog.confirm({
      title: `apt ${command}`,
      message: `Exécuter sur : ${host.value?.hostname}`,
      variant: command === 'dist-upgrade' ? 'danger' : 'warning',
    })

    if (!confirmed) return

    aptCmdLoading.value = command
    try {
      const response = await apiClient.sendAptCommand([hostId], command)
      if (response.data?.commands?.length > 0) {
        const cmd = response.data.commands[0]
        if (cmd.command_id) {
          openCommand({ id: cmd.command_id, module: 'apt', action: command, status: 'pending', output: '' })
        }
      }
    } catch (e: any) {
      await dialog.confirm({
        title: 'Erreur',
        message: e.response?.data?.error || e.message,
        variant: 'danger',
      })
    } finally {
      aptCmdLoading.value = ''
    }
  }

  function isAgentUpToDate(version: string) {
    if (!version || !latestAgentVersion.value) return false
    return version === latestAgentVersion.value
  }

  async function loadComplete() {
    try {
      const res = await apiClient.getHostComplete(hostId)
      const d = res.data
      if (d.host) host.value = d.host
      if (d.metrics) metrics.value = d.metrics
      if (d.containers) containers.value = d.containers
      if (d.apt_status) aptStatus.value = d.apt_status
      if (d.disk_metrics) diskMetrics.value = d.disk_metrics
      if (d.disk_health) diskHealth.value = d.disk_health
      if (d.command_history) cmdHistory.value = d.command_history
      if (d.latest_agent_version) latestAgentVersion.value = d.latest_agent_version
    } catch {
      // Non-critical — WS will populate live data
    }
  }

  async function loadCmdHistoryRefresh() {
    try {
      const res = await apiClient.getHostCommandHistory(hostId)
      cmdHistory.value = res.data?.commands || []
    } catch {
      cmdHistory.value = []
    }
  }

  async function deleteHost() {
    const confirmed = await dialog.confirm({
      title: "Supprimer l'hôte",
      message: 'Cette action est irréversible. Toutes les données associées seront supprimées.',
      variant: 'danger',
      requiredText: host.value?.hostname || host.value?.name,
    })

    if (!confirmed) return

    try {
      await apiClient.deleteHost(hostId)
      router.push('/')
    } catch (e: any) {
      await dialog.confirm({
        title: 'Erreur',
        message: e.response?.data?.error || e.message,
        variant: 'danger',
      })
    }
  }

  async function loadProxmoxLink() {
    try {
      const res = await apiClient.getHostProxmoxLink(hostId)
      proxmoxLink.value = res.data
      if (!res.data) {
        const cands = await apiClient.getHostProxmoxCandidates(hostId).catch(() => ({ data: [] }))
        showLinkButton.value = (cands.data?.length ?? 0) > 0
      }
    } catch {
      proxmoxLink.value = null
      showLinkButton.value = false
    }
  }

  async function confirmLink() {
    if (!proxmoxLink.value) return
    linkSaving.value = true
    try {
      const res = await apiClient.updateProxmoxLink(proxmoxLink.value.id, { status: 'confirmed' })
      proxmoxLink.value = res.data
    } finally {
      linkSaving.value = false
    }
  }

  async function ignoreLink() {
    if (!proxmoxLink.value) return
    linkSaving.value = true
    try {
      await apiClient.updateProxmoxLink(proxmoxLink.value.id, { status: 'ignored' })
      proxmoxLink.value = null
      showLinkButton.value = true
    } finally {
      linkSaving.value = false
    }
  }

  async function changeMetricsSource(source: 'agent' | 'proxmox' | 'auto') {
    if (!proxmoxLink.value) return
    linkSaving.value = true
    try {
      const res = await apiClient.updateProxmoxLink(proxmoxLink.value.id, { metrics_source: source })
      proxmoxLink.value = res.data
    } finally {
      linkSaving.value = false
    }
  }

  async function deleteLink() {
    if (!proxmoxLink.value) return
    linkSaving.value = true
    try {
      await apiClient.deleteProxmoxLink(proxmoxLink.value.id)
      proxmoxLink.value = null
      showLinkButton.value = true
    } finally {
      linkSaving.value = false
    }
  }

  async function openLinkForm() {
    showLinkForm.value = true
    if (linkCandidates.value.length > 0) return
    linkCandidatesLoading.value = true
    try {
      const res = await apiClient.getHostProxmoxCandidates(hostId)
      linkCandidates.value = res.data || []
    } finally {
      linkCandidatesLoading.value = false
    }
  }

  async function createManualLink() {
    if (!selectedCandidate.value) return
    linkSaving.value = true
    try {
      const res = await apiClient.createProxmoxLink({
        guest_id: selectedCandidate.value,
        host_id: hostId,
        status: 'confirmed',
        metrics_source: 'auto',
      })
      proxmoxLink.value = res.data
      showLinkForm.value = false
      showLinkButton.value = false
      selectedCandidate.value = ''
    } finally {
      linkSaving.value = false
    }
  }

  function formatBytesLink(bytes: number) {
    if (!bytes) return '0 B'
    const units = ['B', 'Ko', 'Mo', 'Go', 'To']
    let i = 0
    let v = bytes
    while (v >= 1024 && i < units.length - 1) {
      v /= 1024
      i++
    }
    return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
  }

  const hostPerms = ref<AnyRecord[]>([])
  const permLoading = ref(false)
  const addPermModal = ref(false)
  const newPermUsername = ref('')
  const newPermLevel = ref('viewer')
  const permSaving = ref(false)
  const permError = ref('')
  const allUsers = ref<AnyRecord[]>([])

  const availableUsers = computed(() =>
    allUsers.value.filter((u: AnyRecord) => u.role !== 'admin' && !hostPerms.value.some((p: AnyRecord) => p.username === u.username))
  )

  async function loadHostPerms() {
    if (!auth.isAdmin) return
    permLoading.value = true
    try {
      const [permsRes, usersRes] = await Promise.all([apiClient.getHostPermissions(hostId), apiClient.getUsers()])
      hostPerms.value = permsRes.data || []
      allUsers.value = usersRes.data || []
    } finally {
      permLoading.value = false
    }
  }

  function openAddPermission() {
    newPermUsername.value = ''
    newPermLevel.value = 'viewer'
    permError.value = ''
    addPermModal.value = true
  }

  async function savePermission() {
    permSaving.value = true
    permError.value = ''
    try {
      await apiClient.setHostPermission(hostId, newPermUsername.value, newPermLevel.value)
      addPermModal.value = false
      await loadHostPerms()
    } catch (e: any) {
      permError.value = e?.response?.data?.error || "Erreur lors de l'enregistrement"
    } finally {
      permSaving.value = false
    }
  }

  async function revokePermission(username: string) {
    try {
      await apiClient.deleteHostPermission(hostId, username)
      await loadHostPerms()
    } catch {
      // ignore
    }
  }

  onMounted(() => {
    loadComplete()
    loadProxmoxLink()
    loadHostPerms()
  })

  return {
    auth,
    hostId,
    canRunApt,
    activeTab,
    isEditing,
    tasksCount,
    aptCmdLoading,
    host,
    containers,
    versionComparisons,
    aptStatus,
    cmdHistory,
    diskMetrics,
    diskHealth,
    proxmoxLink,
    linkSaving,
    effectiveMetrics,
    effectiveMetricsSource,
    showLinkForm,
    showLinkButton,
    linkCandidates,
    linkCandidatesLoading,
    selectedCandidate,
    liveCommand,
    showConsole,
    wsStatus,
    wsError,
    retryCount,
    reconnect,
    openCommand,
    sendAptCmd,
    isAgentUpToDate,
    deleteHost,
    loadCmdHistoryRefresh,
    confirmLink,
    ignoreLink,
    changeMetricsSource,
    deleteLink,
    openLinkForm,
    createManualLink,
    closeConsoleAndStream,
    clearConsoleOutput,
    formatBytesLink,
    hostPerms,
    permLoading,
    addPermModal,
    newPermUsername,
    newPermLevel,
    permSaving,
    permError,
    availableUsers,
    openAddPermission,
    savePermission,
    revokePermission,
  }
}

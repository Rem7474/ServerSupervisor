import { ref, onUnmounted, computed } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from './useWebSocket'
import type { WSAptSnapshot, UnattendedUpgradesDB } from '../types/ws'
import { useConfirmDialog } from './useConfirmDialog'
import { confirmAptCommand } from '../utils/aptConfirm'
import { confirmBulkAction } from '../utils/bulkActionHelpers'
import { addToast } from './useGlobalToast'
import { useCommandStream } from './useCommandStream'
import type { Host } from '../types/host'

// API-facing shapes (the server transforms the Go AptStatus.cve_list JSON
// string into a parsed array, so the generated model doesn't apply here).
interface CveInfo { severity?: string; [key: string]: unknown }
interface AptStatusView {
  pending_packages?: number
  security_updates?: number
  cve_list?: CveInfo[]
  package_list?: unknown
  updated_at?: string
  cve_updated_at?: string
  [key: string]: unknown
}

function parseJsonArray<T = unknown>(value: unknown): T[] {
  if (!value) return []
  try {
    const parsed = typeof value === 'string' ? JSON.parse(value) : value
    return Array.isArray(parsed) ? (parsed as T[]) : []
  } catch {
    return []
  }
}
interface AptCommand {
  id: string
  host_id?: string
  hostId?: string
  action?: string
  command?: string
  status?: string
  output?: string
  created_at?: string
  started_at?: string | null
  ended_at?: string | null
  triggered_by?: string
  [key: string]: unknown
}
interface AptCommandResult {
  command_id?: string
  host_id?: string
  status?: string
  error?: string
  [key: string]: unknown
}
interface LiveCommand {
  id: string
  hostId: string | null
  host_name: string
  module: string
  action: string
  target: string
  status?: string
  output?: string
}
interface CommandPatch { status?: string; output?: string; action?: string }

export function useApt() {
  // ── État hôtes / APT ─────────────────────────────────────────────────────────
  const hosts = ref<Host[]>([])
  const selectedHosts = ref<string[]>([])
  const hostExpanded = ref<Record<string, boolean>>({})
  const aptStatuses = ref<Record<string, AptStatusView>>({})
  const aptHistories = ref<Record<string, AptCommand[]>>({})
  const uuStatuses = ref<Record<string, UnattendedUpgradesDB | undefined>>({})
  const latestAgentVersion = ref('')
  const hostCmdLoading = ref<Record<string, string | null>>({})
  const bulkAgentUpdateLoading = ref(false)

  // ── Enrichissement CVE en arrière-plan (post-commande) ──────────────────────
  // After an apt update/upgrade/dist-upgrade command completes, the agent still
  // runs its CVE-enrichment refresh in the background before pushing a fresh
  // apt_status (agent/internal/dispatcher/handler_apt.go's aptStatusRefreshTimeout,
  // bounded to 5min) — without this, the UI goes silent between "command
  // completed" and the CVE/security numbers actually updating, which reads as
  // broken/stuck. Tracked per host, cleared as soon as a WS snapshot arrives
  // with a newer apt_status.cve_updated_at than the one captured when the
  // command completed, or after a safety timeout. cve_updated_at (not the
  // general updated_at) is what's compared: updated_at also bumps from the
  // separate, near-instant pending-packages-only refresh bundled into the
  // command's own completion report (CollectAPTFast/UpsertAptPendingPackages,
  // which never touches security_updates/cve_list) — comparing updated_at
  // would clear this badge the moment *that* lands, seconds in, well before
  // the CVE data this badge exists to cover has actually refreshed.
  const ENRICHING_ACTIONS = new Set(['update', 'upgrade', 'dist-upgrade'])
  const ENRICHING_SAFETY_TIMEOUT_MS = 5.5 * 60_000
  const enrichingHosts = ref<Record<string, boolean>>({})
  const enrichingSinceCveUpdatedAt: Record<string, string | undefined> = {}
  const enrichingTimers: Record<string, ReturnType<typeof setTimeout>> = {}

  function startEnriching(hostId: string): void {
    enrichingSinceCveUpdatedAt[hostId] = aptStatuses.value[hostId]?.cve_updated_at
    enrichingHosts.value = { ...enrichingHosts.value, [hostId]: true }
    clearTimeout(enrichingTimers[hostId])
    enrichingTimers[hostId] = setTimeout(() => stopEnriching(hostId), ENRICHING_SAFETY_TIMEOUT_MS)
  }

  function stopEnriching(hostId: string): void {
    clearTimeout(enrichingTimers[hostId])
    delete enrichingTimers[hostId]
    if (!enrichingHosts.value[hostId]) return
    const next = { ...enrichingHosts.value }
    delete next[hostId]
    enrichingHosts.value = next
  }

  const auth = useAuthStore()
  const dialog = useConfirmDialog()
  const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

  const selectAll = computed({
    get() {
      const ids = filteredHosts.value.map((h: Host) => h.id)
      return ids.length > 0 && ids.every((id: string) => selectedHosts.value.includes(id))
    },
    set(val: boolean) {
      selectedHosts.value = val ? filteredHosts.value.map((h: Host) => h.id) : []
    },
  })

  function toggleSelected(hostId: string, selected: boolean): void {
    if (selected) {
      if (!selectedHosts.value.includes(hostId)) selectedHosts.value = [...selectedHosts.value, hostId]
    } else {
      selectedHosts.value = selectedHosts.value.filter((id) => id !== hostId)
    }
  }

  // ── Modal planification ───────────────────────────────────────────────────────
  const scheduleHost = ref<Host | null>(null)

  function openScheduleModal(host: Host): void {
    scheduleHost.value = host
  }

  // ── Console ───────────────────────────────────────────────────────────────────
  const showConsole = ref(false)
  const liveCommand = ref<LiveCommand | null>(null)
  const { openCommandStream, closeStream } = useCommandStream()
  const aptBulkLoading = ref<string | null>(null)

  // ── Filtres / tri des hôtes ───────────────────────────────────────────────────
  const hostSearch = ref('')
  const hostQuickFilter = ref('all')
  const hostSortKey = ref<'name' | 'pending' | 'security' | 'cve'>('name')
  const hostSortDir = ref<'asc' | 'desc'>('asc')

  const hostFilterOptions = [
    { value: 'all', label: 'Tous' },
    { value: 'critical', label: 'CVE critiques' },
    { value: 'security', label: 'Sécu > 0' },
    { value: 'reboot', label: 'Redémarrage requis' },
    { value: 'outdated_agent', label: 'Agent obsolète' },
  ]

  function isAgentOutdated(host: Host): boolean {
    return !!host.agent_version && !!latestAgentVersion.value && host.agent_version !== latestAgentVersion.value
  }

  const filteredHosts = computed(() => {
    let list = [...hosts.value]

    const q = hostSearch.value.trim().toLowerCase()
    if (q) {
      list = list.filter((h: Host) => {
        const primary = (h.name || h.hostname || '').toLowerCase()
        const secondary = (h.hostname || '').toLowerCase()
        if (primary.includes(q) || secondary.includes(q) || (h.ip_address || '').includes(q)) return true
        // Also match a package name or CVE id, so "I heard about CVE-2024-XXXX,
        // which hosts does it affect?" doesn't require expanding every card.
        const status = aptStatuses.value[h.id]
        const packages = parseJsonArray<string>(status?.package_list)
        if (packages.some((pkg) => pkg.toLowerCase().includes(q))) return true
        const cves = status?.cve_list || []
        return cves.some((c) => (c.id ? String(c.id).toLowerCase().includes(q) : false))
      })
    }

    if (hostQuickFilter.value === 'critical') {
      list = list.filter((h: Host) => {
        const cves = aptStatuses.value[h.id]?.cve_list
        return Array.isArray(cves) && cves.some((c: CveInfo) => c.severity === 'CRITICAL')
      })
    } else if (hostQuickFilter.value === 'security') {
      list = list.filter((h: Host) => (aptStatuses.value[h.id]?.security_updates || 0) > 0)
    } else if (hostQuickFilter.value === 'reboot') {
      list = list.filter((h: Host) => !!uuStatuses.value[h.id]?.reboot_required)
    } else if (hostQuickFilter.value === 'outdated_agent') {
      list = list.filter((h: Host) => isAgentOutdated(h))
    }

    list.sort((a: Host, b: Host) => {
      let va: number | string, vb: number | string
      if (hostSortKey.value === 'pending') {
        va = aptStatuses.value[a.id]?.pending_packages || 0
        vb = aptStatuses.value[b.id]?.pending_packages || 0
      } else if (hostSortKey.value === 'security') {
        va = aptStatuses.value[a.id]?.security_updates || 0
        vb = aptStatuses.value[b.id]?.security_updates || 0
      } else if (hostSortKey.value === 'cve') {
        va = (aptStatuses.value[a.id]?.cve_list || []).length
        vb = (aptStatuses.value[b.id]?.cve_list || []).length
      } else {
        va = (a.name || a.hostname || '').toLowerCase()
        vb = (b.name || b.hostname || '').toLowerCase()
      }
      if (va < vb) return hostSortDir.value === 'asc' ? -1 : 1
      if (va > vb) return hostSortDir.value === 'asc' ? 1 : -1
      return 0
    })

    return list
  })

  // ── Console / streaming ─────────────────────────────────────────────────────
  function watchCommand(cmd: AptCommand, host: Host | null): void {
    showConsole.value = true
    liveCommand.value = {
      id: cmd.id,
      hostId: host?.id || cmd.hostId || cmd.host_id || null,
      host_name: host?.name || host?.hostname || '—',
      module: 'apt',
      action: cmd.action || cmd.command || '—',
      target: '',
      status: cmd.status,
      output: cmd.output || '',
    }
    connectStreamWebSocket(cmd.id)
  }

  function closeLiveConsole(): void {
    closeStream()
    liveCommand.value = null
    showConsole.value = false
  }

  function upsertAptHistory(hostId: string, nextCommand: AptCommand): void {
    if (!hostId || !nextCommand?.id) return
    const currentHistory = Array.isArray(aptHistories.value[hostId]) ? [...aptHistories.value[hostId]] : []
    const currentIndex = currentHistory.findIndex((cmd: AptCommand) => cmd.id === nextCommand.id)
    if (currentIndex >= 0) {
      currentHistory[currentIndex] = { ...currentHistory[currentIndex], ...nextCommand }
    } else {
      currentHistory.unshift(nextCommand)
    }
    currentHistory.sort((left: AptCommand, right: AptCommand) => new Date(right.created_at || 0).getTime() - new Date(left.created_at || 0).getTime())
    aptHistories.value = { ...aptHistories.value, [hostId]: currentHistory }
  }

  function syncLiveCommand(commandId: string, patch: CommandPatch): void {
    if (!liveCommand.value || liveCommand.value.id !== commandId) return
    liveCommand.value = { ...liveCommand.value, ...patch }
  }

  function syncAptHistoryCommand(commandId: string, patch: CommandPatch): void {
    const hostId = liveCommand.value?.id === commandId ? liveCommand.value.hostId : null
    if (!hostId) return
    const action = liveCommand.value?.action || patch.action
    upsertAptHistory(hostId, {
      id: commandId,
      action,
      output: liveCommand.value?.output || '',
      ...patch,
    })
    if ((patch.status === 'completed' || patch.status === 'failed') && action && ENRICHING_ACTIONS.has(action)) {
      startEnriching(hostId)
    }
  }

  function connectStreamWebSocket(commandId: string): void {
    closeStream()
    openCommandStream(commandId, {
      closeOnTerminalStatus: true,
      onInit: (payload) => {
        syncLiveCommand(commandId, { status: payload.status, output: payload.output || '' })
        syncAptHistoryCommand(commandId, { status: payload.status })
      },
      onChunk: (payload) => {
        const nextOutput = `${liveCommand.value?.output || ''}${payload.chunk || ''}`
        syncLiveCommand(commandId, { output: nextOutput })
      },
      onStatus: (payload) => {
        const patch: CommandPatch = { status: payload.status }
        if (typeof payload.output === 'string') patch.output = payload.output
        syncLiveCommand(commandId, patch)
        syncAptHistoryCommand(commandId, patch)
      },
    })
  }

  // ── Commandes par hôte ────────────────────────────────────────────────────────
  async function runAptCmdForHost(host: Host, command: string): Promise<void> {
    if (!canRunApt.value) return

    const confirmed = await confirmAptCommand(command, host.name || host.hostname || host.id)
    if (!confirmed) return

    hostCmdLoading.value = { ...hostCmdLoading.value, [host.id]: command }
    try {
      const response = await apiClient.sendAptCommand([host.id], command)
      const commandResults: AptCommandResult[] = Array.isArray(response.data?.commands) ? response.data.commands : []
      const launched = commandResults.filter((item: AptCommandResult) => item.command_id)
      const failed = commandResults.filter((item: AptCommandResult) => item.error)
      const createdAt = new Date().toISOString()

      launched.forEach((item: AptCommandResult) => {
        upsertAptHistory(host.id, {
          id: item.command_id!,
          action: command,
          status: item.status || 'pending',
          output: '',
          created_at: createdAt,
          triggered_by: auth.username || '',
        })
      })

      if (launched.length > 0) {
        watchCommand(
          { id: launched[0].command_id!, action: command, status: launched[0].status || 'pending', output: '' },
          host
        )
      } else if (failed.length > 0) {
        await dialog.confirm({
          title: 'Erreur',
          message: failed[0].error || 'Erreur lors de l\'envoi de la commande',
          variant: 'danger',
        })
      }
    } catch (e) {
      await dialog.confirm({ title: 'Erreur', message: getApiErrorMessage(e), variant: 'danger' })
    } finally {
      const next = { ...hostCmdLoading.value }
      delete next[host.id]
      hostCmdLoading.value = next
    }
  }

  // ── Commandes groupées ────────────────────────────────────────────────────────
  async function bulkAptCmd(command: string): Promise<void> {
    const hostnames = hosts.value
      .filter((h: Host) => selectedHosts.value.includes(h.id))
      .map((h: Host) => h.name || h.hostname)
      .join(', ')

    const confirmed = await confirmAptCommand(command, hostnames || 'les hôtes sélectionnés', selectedHosts.value.length)
    if (!confirmed) return

    aptBulkLoading.value = command
    try {
      const response = await apiClient.sendAptCommand(selectedHosts.value, command)
      const commandResults: AptCommandResult[] = Array.isArray(response.data?.commands) ? response.data.commands : []
      const hostNameById = new Map(hosts.value.map((host: Host) => [host.id, host.name || host.hostname || host.id]))
      const launchedCommands = commandResults.filter((item: AptCommandResult) => item.command_id)
      const failedCommands = commandResults.filter((item: AptCommandResult) => item.error)
      const createdAt = new Date().toISOString()

      launchedCommands.forEach((item: AptCommandResult) => {
        upsertAptHistory(item.host_id ?? '', {
          id: item.command_id!,
          action: command,
          status: item.status || 'pending',
          output: '',
          created_at: createdAt,
          started_at: null,
          ended_at: null,
          triggered_by: auth.username || '',
        })
      })

      if (selectedHosts.value.length === 1 && launchedCommands.length > 0) {
        const launchedCommand = launchedCommands[0]
        const host = hosts.value.find((h: Host) => h.id === launchedCommand.host_id)
        if (host) {
          watchCommand({ id: launchedCommand.command_id!, action: command, status: launchedCommand.status || 'pending', output: '' }, host)
        }
      }

      if (selectedHosts.value.length > 1 || failedCommands.length > 0) {
        const launched = launchedCommands.map((item: AptCommandResult) => hostNameById.get(item.host_id ?? "") || (item.host_id ?? ""))
        const failed = failedCommands.map((item: AptCommandResult) => hostNameById.get(item.host_id ?? "") || (item.host_id ?? ""))
        const launchedLabel = launched.length === 1 ? `sur ${launched[0]}` : `sur ${launched.length} hôtes`
        const msg = launched.length > 0
          ? `apt ${command} lancée ${launchedLabel}${failed.length ? ` — échec sur : ${failed.join(', ')}` : ''}`
          : `apt ${command} — aucune commande lancée`
        addToast(msg, failed.length > 0 ? 'warning' : 'success', 7000)
      }
    } catch (e) {
      await dialog.confirm({
        title: 'Erreur',
        message: getApiErrorMessage(e),
        variant: 'danger'
      })
    } finally {
      aptBulkLoading.value = null
    }
  }

  // ── Mise à jour des agents ────────────────────────────────────────────────────
  // Reuses the same per-host endpoint as the host-detail "Mettre à jour l'agent"
  // button (TriggerAgentUpdate), fired in parallel across every selected host
  // whose reported agent_version differs from latestAgentVersion. The server
  // already guards "already up to date" / "update already in progress" per
  // host (apperr.Conflict), so it's safe to fire even if the selection is a
  // mix of outdated/up-to-date hosts — outdatedSelectedHosts filters most of
  // that out client-side anyway, to keep the confirmation message accurate.
  const outdatedSelectedHosts = computed(() =>
    hosts.value.filter((h: Host) => selectedHosts.value.includes(h.id) && isAgentOutdated(h))
  )

  async function bulkAgentUpdate(): Promise<void> {
    const targets = outdatedSelectedHosts.value
    if (targets.length === 0) return

    const hostnames = targets.map((h: Host) => h.name || h.hostname).join(', ')
    const confirmed = await confirmBulkAction(
      'Mettre à jour les agents',
      targets.length,
      `Déployer la version ${latestAgentVersion.value} sur : ${hostnames}. L'agent sera redémarré pendant l'opération sur chaque hôte.`
    )
    if (!confirmed) return

    bulkAgentUpdateLoading.value = true
    try {
      const results = await Promise.allSettled(targets.map((h: Host) => apiClient.updateHostAgent(h.id)))
      const succeeded = results.filter((r) => r.status === 'fulfilled').length
      const failed = results.length - succeeded
      if (failed === 0) {
        addToast(`Mise à jour lancée sur ${succeeded} agent${succeeded > 1 ? 's' : ''}`, 'success', 7000)
      } else {
        addToast(
          `${succeeded} agent(s) lancé(s), ${failed} échec(s)`,
          failed === results.length ? 'error' : 'warning',
          7000
        )
      }
    } finally {
      bulkAgentUpdateLoading.value = false
    }
  }

  const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket<WSAptSnapshot>('/api/v1/ws/apt', (payload) => {
    if (payload.type !== 'apt') return
    // The generated WSAptSnapshot models cve_list as a JSON string, but the
    // server sends it pre-parsed; cast at this single boundary to the real shapes.
    hosts.value = (payload.hosts || []) as Host[]
    aptStatuses.value = (payload.apt_statuses || {}) as unknown as Record<string, AptStatusView>
    aptHistories.value = (payload.apt_histories || {}) as unknown as Record<string, AptCommand[]>
    uuStatuses.value = payload.uu_statuses || {}
    latestAgentVersion.value = payload.latest_agent_version || ''
    for (const hostId of Object.keys(enrichingHosts.value)) {
      const cveUpdatedAt = aptStatuses.value[hostId]?.cve_updated_at
      if (cveUpdatedAt && cveUpdatedAt !== enrichingSinceCveUpdatedAt[hostId]) {
        stopEnriching(hostId)
      }
    }
  }, { debounceMs: 750 })

  onUnmounted(() => {
    closeStream()
    Object.values(enrichingTimers).forEach(clearTimeout)
  })

  return {
    hosts,
    selectedHosts,
    hostExpanded,
    aptStatuses,
    aptHistories,
    uuStatuses,
    latestAgentVersion,
    hostCmdLoading,
    enrichingHosts,
    auth,
    canRunApt,
    selectAll,
    toggleSelected,
    scheduleHost,
    openScheduleModal,
    showConsole,
    liveCommand,
    aptBulkLoading,
    hostSearch,
    hostQuickFilter,
    hostSortKey,
    hostSortDir,
    hostFilterOptions,
    filteredHosts,
    isAgentOutdated,
    outdatedSelectedHosts,
    bulkAgentUpdateLoading,
    watchCommand,
    closeLiveConsole,
    runAptCmdForHost,
    bulkAptCmd,
    bulkAgentUpdate,
    wsStatus,
    wsError,
    retryCount,
    dataStaleAlert,
    reconnect,
  }
}

import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'
import apiClient from '../api'
import { addToast } from './useGlobalToast'
import { getApiErrorMessage } from '../api/client'
import { useStatusBadge } from './useStatusBadge'
import { commandStatusLabel } from '../utils/commandStatus'
import { moduleLabel, moduleClass, remoteCommandModuleOptions } from '../utils/moduleMeta'
import { useCommandStream } from './useCommandStream'
import type { RemoteCommand, RemoteCommandWithHost } from '../types/audit'
import type { CommandStreamInitMsg, CommandStreamChunkMsg, CommandStatusUpdateMsg } from '../types/ws'
import type { AuditLog, LoginEvent } from '../types/generated'
import { AuditCategoryAlert, AuditCategoryAuth, AuditCategoryCommand, AuditCategorySettings } from '../types/generated'
import type { SecurityData } from '../components/security/AuditSecurityPanel.vue'

// Values imported from the generated Go consts so they can't drift from
// server/internal/models/audit.go's category keys — only the French labels
// are local to this file.
export const JOURNAL_CATEGORY_OPTIONS = [
  { value: '', label: 'Toutes catégories' },
  { value: AuditCategoryAlert, label: 'Alertes' },
  { value: AuditCategoryAuth, label: 'Authentification' },
  { value: AuditCategorySettings, label: 'Réglages' },
  { value: AuditCategoryCommand, label: 'Commandes' },
]

// Backs AuditLogsView.vue (command history + admin connexions/security tabs).
// NOTE: this file previously held a small, unused `fetchAuditLogs` scaffold
// (GET /v1/audit/logs via apiClient.getAuditLogs) with no callers anywhere in
// the app — it predates AuditLogsView.vue's actual data needs (command
// history + login/security admin endpoints) and has been replaced below.
export function useAuditLogs() {
  const { getStatusBadgeClass } = useStatusBadge()

  const route = useRoute()
  const router = useRouter()
  const auth = useAuthStore()
  const canViewCommands = computed(() => auth.role === 'admin' || auth.role === 'operator')
  const moduleFilterOptions = computed(() => remoteCommandModuleOptions())

  const activeTab = ref((route.query.tab as string) || 'commandes')

  watch(activeTab, (tab) => {
    router.replace({ query: { ...route.query, tab } })
  })

  // ── Commands history ─────────────────────────────────────────────────────────
  const cmds = ref<RemoteCommandWithHost[]>([])
  const cmdsPage = ref(1)
  const cmdsLimit = 50
  const cmdsTotal = ref(0)
  const cmdsLoading = ref(false)
  const cmdsLoaded = ref(false)

  const totalCmdsPages = computed(() => Math.max(1, Math.ceil(cmdsTotal.value / cmdsLimit)))
  const cmdSearch = ref('')
  const cmdStatusFilter = ref('')
  const cmdModuleFilter = ref('')
  const cmdSortBy = ref('created_at')
  const cmdSortDir = ref('desc')

  // Client-side sort only (server already returns filtered+paginated, just re-sort current page)
  const sortedCmds = computed(() => {
    const arr = [...cmds.value]
    const dir = cmdSortDir.value === 'asc' ? 1 : -1
    arr.sort((a, b) => {
      const key = cmdSortBy.value
      if (key === 'created_at') {
        const av = new Date(a.created_at || 0).getTime()
        const bv = new Date(b.created_at || 0).getTime()
        return (av < bv ? -1 : av > bv ? 1 : 0) * dir
      }
      const av = key === 'command' ? cmdLabel(a) : ((a as Record<string, unknown>)[key] || '')
      const bv = key === 'command' ? cmdLabel(b) : ((b as Record<string, unknown>)[key] || '')
      return String(av).toLowerCase().localeCompare(String(bv).toLowerCase()) * dir
    })
    return arr
  })

  const hasActiveCommands = computed(() =>
    cmds.value.some((c) => c.status === 'pending' || c.status === 'running')
  )

  function toggleCmdSort(key: string): void {
    if (cmdSortBy.value === key) {
      cmdSortDir.value = cmdSortDir.value === 'asc' ? 'desc' : 'asc'
      return
    }
    cmdSortBy.value = key
    cmdSortDir.value = key === 'created_at' ? 'desc' : 'asc'
  }

  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
  function onSearchUpdate(val: string): void {
    cmdSearch.value = val
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
    searchDebounceTimer = setTimeout(() => {
      cmdsPage.value = 1
      fetchCmds()
    }, 350)
  }

  function onFilterChange(): void {
    cmdsPage.value = 1
    fetchCmds()
  }

  const selectedCmd = ref<RemoteCommand | null>(null)
  const showLogViewer = ref(false)
  const cancellingId = ref<string | null>(null)
  let auditPollTimer: ReturnType<typeof setInterval> | null = null

  const { openCommandStream, closeStream } = useCommandStream()

  // ── Connexions (admin) ───────────────────────────────────────────────────────
  const connexions = ref<LoginEvent[]>([])
  const connexionsPage = ref(1)
  const connexionsLimit = 50
  const connexionsTotal = ref(0)
  const connexionsLoading = ref(false)
  const connexionsLoaded = ref(false)
  const security = ref<SecurityData>({ stats: null, blocked_ips: [], top_failed_ips: [] })
  const lastCmdFetchAt = ref(0)
  const lastConnFetchAt = ref(0)

  const dialog = useConfirmDialog()
  const secPeriodOptions = [
    { hours: 24, label: '24h' },
    { hours: 168, label: '7j' },
    { hours: 720, label: '30j' },
  ]
  const securityPeriod = ref(24)
  const securityPeriodLabel = computed(() => secPeriodOptions.find((p) => p.hours === securityPeriod.value)?.label ?? '24h')
  const unblockingIP = ref('')

  const totalConnexionsPages = computed(() =>
    Math.max(1, Math.ceil(connexionsTotal.value / connexionsLimit))
  )

  function statusLabel(status: string): string {
    return commandStatusLabel(status)
  }

  function cmdLabel(cmd: RemoteCommand): string {
    const parts = [cmd.action]
    if (cmd.target) parts.push(cmd.target)
    return parts.filter(Boolean).join(' ')
  }

  function formatDuration(startedAt: string | null | undefined, endedAt: string | null | undefined): string {
    if (!startedAt || !endedAt) return '—'
    const diff = Math.max(0, Math.round((new Date(endedAt).getTime() - new Date(startedAt).getTime()) / 1000))
    if (diff < 60) return `${diff}s`
    const m = Math.floor(diff / 60), s = diff % 60
    return s > 0 ? `${m}m ${s}s` : `${m}m`
  }

  function statusClass(status: string | undefined): string {
    return getStatusBadgeClass(status, 'badge bg-warning-lt text-warning')
  }

  function openLogViewer(cmd: RemoteCommand): void {
    if (selectedCmd.value?.id === cmd.id) {
      showLogViewer.value = true
      return
    }
    closeLogViewer()
    selectedCmd.value = { ...cmd }
    showLogViewer.value = true

    if (cmd.status === 'running' || cmd.status === 'pending') {
      connectStream(cmd.id)
    }
  }

  function closeLogViewer(): void {
    closeStream()
    selectedCmd.value = null
    showLogViewer.value = false
  }

  function connectStream(commandId: string): void {
    const syncCmdInList = (patch: Partial<RemoteCommand>): void => {
      const idx = cmds.value.findIndex((c) => c.id === commandId)
      if (idx === -1) return
      const next = [...cmds.value]
      next[idx] = { ...next[idx], ...patch }
      cmds.value = next
    }

    openCommandStream(commandId, {
      onInit(p: CommandStreamInitMsg) {
        if (selectedCmd.value) { selectedCmd.value.status = p.status; selectedCmd.value.output = p.output || '' }
        syncCmdInList({ status: p.status, output: p.output || '' })
      },
      onChunk(p: CommandStreamChunkMsg) {
        if (selectedCmd.value) selectedCmd.value.output = (selectedCmd.value.output || '') + p.chunk
      },
      onStatus(p: CommandStatusUpdateMsg) {
        if (selectedCmd.value) { selectedCmd.value.status = p.status; if (p.output) selectedCmd.value.output = p.output }
        syncCmdInList({ status: p.status, ...(p.output ? { output: p.output } : {}) })
      },
    })
  }

  // ── Data fetching ─────────────────────────────────────────────────────────────
  async function fetchCmds(): Promise<void> {
    if (cmdsLoading.value) return
    cmdsLoading.value = true
    try {
      const filters = {
        search: cmdSearch.value.trim() || undefined,
        module: cmdModuleFilter.value || undefined,
        status: cmdStatusFilter.value || undefined,
      }
      const res = await apiClient.getCommandsHistory(cmdsPage.value, cmdsLimit, filters)
      const nextCmds = res.data?.commands || []
      cmds.value = nextCmds
      await reconcileCommandStatuses(nextCmds)
      cmdsTotal.value = res.data?.total || 0
      cmdsLoaded.value = true
      lastCmdFetchAt.value = Date.now()
    } catch { cmds.value = [] } finally { cmdsLoading.value = false }
  }

  async function reconcileCommandStatuses(list: RemoteCommandWithHost[]): Promise<void> {
    const ids: string[] = []
    for (const c of list) {
      if (c.status === 'pending' || c.status === 'running') {
        ids.push(c.id)
      }
    }
    if (selectedCmd.value?.id && !ids.includes(selectedCmd.value.id)) {
      ids.push(selectedCmd.value.id)
    }
    if (!ids.length) return

    const snapshots = await Promise.allSettled(ids.map((id) => apiClient.getCommandStatus(id)))
    if (!snapshots.length) return

    const patchById: Record<string, RemoteCommand> = {}
    snapshots.forEach((result, idx) => {
      if (result.status !== 'fulfilled') return
      const cmd = result.value?.data
      if (!cmd?.id) return
      patchById[ids[idx]] = cmd
    })

    if (!Object.keys(patchById).length) return

    cmds.value = list.map((c) => {
      const snap = patchById[c.id]
      if (!snap) return c
      const isActive = snap.status === 'running' || snap.status === 'pending'
      return {
        ...c,
        status: snap.status || c.status,
        // Don't overwrite streamed output with empty DB value while command is active
        output: isActive ? c.output : (snap.output ?? c.output),
        started_at: snap.started_at || c.started_at,
        ended_at: snap.ended_at || c.ended_at,
      }
    })

    const current = selectedCmd.value
    if (current?.id && patchById[current.id]) {
      const snap = patchById[current.id]
      const isActive = snap.status === 'running' || snap.status === 'pending'
      selectedCmd.value = {
        ...current,
        ...snap,
        // Preserve streamed output accumulated from WebSocket while command is still active
        output: isActive ? current.output : (snap.output ?? current.output),
      }
    }
  }

  async function fetchConnexions(): Promise<void> {
    if (connexionsLoading.value) return
    connexionsLoading.value = true
    try {
      const [evRes, secRes] = await Promise.allSettled([
        apiClient.getLoginEventsAdmin(connexionsPage.value, connexionsLimit),
        apiClient.getSecuritySummary(securityPeriod.value),
      ])

      if (evRes.status === 'fulfilled') {
        connexions.value = evRes.value.data?.events || []
        connexionsTotal.value = evRes.value.data?.total || 0
        connexionsLoaded.value = true
      } else {
        connexions.value = []
        connexionsTotal.value = 0
      }

      if (secRes.status === 'fulfilled') {
        security.value = secRes.value.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
      } else {
        security.value = { stats: null, blocked_ips: [], top_failed_ips: [] }
      }
      lastConnFetchAt.value = Date.now()
    } finally { connexionsLoading.value = false }
  }

  async function switchToCommandes(): Promise<void> {
    activeTab.value = 'commandes'
    if (!cmdsLoaded.value) await fetchCmds()
  }

  async function switchToConnexions(): Promise<void> {
    activeTab.value = 'connexions'
    if (!connexionsLoaded.value) await fetchConnexions()
  }

  // ── Journal d'audit (raw audit_logs, admin only) ────────────────────────────────
  const journalLogs = ref<AuditLog[]>([])
  const journalPage = ref(1)
  const journalLimit = 50
  const journalLoading = ref(false)
  const journalLoaded = ref(false)
  const journalExporting = ref(false)
  // No total count from the API (GetAuditLogs doesn't run a COUNT) — a full
  // page means "there might be more," same trade-off as a cursor without a
  // known end, cheap enough at this limit to just try the next page.
  const journalHasMore = ref(false)
  const journalCategoryFilter = ref('')
  const journalFrom = ref('')
  const journalTo = ref('')

  async function fetchJournal(): Promise<void> {
    journalLoading.value = true
    try {
      const res = await apiClient.getAuditLogs(journalPage.value, journalLimit, {
        category: journalCategoryFilter.value || undefined,
        from: journalFrom.value || undefined,
        to: journalTo.value || undefined,
      })
      journalLogs.value = res.data?.logs || []
      journalHasMore.value = journalLogs.value.length === journalLimit
      journalLoaded.value = true
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, "Impossible de charger le journal d'audit"), 'error')
    } finally {
      journalLoading.value = false
    }
  }

  async function switchToJournal(): Promise<void> {
    activeTab.value = 'journal'
    if (!journalLoaded.value) await fetchJournal()
  }

  function selectJournalPage(page: number): void {
    if (page < 1 || page === journalPage.value) return
    journalPage.value = page
    fetchJournal()
  }

  watch([journalCategoryFilter, journalFrom, journalTo], () => {
    journalPage.value = 1
    fetchJournal()
  })

  async function exportJournal(): Promise<void> {
    journalExporting.value = true
    try {
      const response = await apiClient.exportAuditLogs({
        category: journalCategoryFilter.value || undefined,
        from: journalFrom.value || undefined,
        to: journalTo.value || undefined,
      })
      const blob = response.data instanceof Blob ? response.data : new Blob([response.data as BlobPart], { type: 'text/csv' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `audit-logs-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.csv`
      link.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, "Échec de l'export du journal d'audit"), 'error')
    } finally {
      journalExporting.value = false
    }
  }

  // ── Pagination ────────────────────────────────────────────────────────────────
  function selectCmdsPage(page: number): void {
    if (page === cmdsPage.value) return
    cmdsPage.value = page
    closeLogViewer()
    fetchCmds()
  }

  function selectConnexionsPage(page: number): void {
    if (page === connexionsPage.value) return
    connexionsPage.value = page
    fetchConnexions()
  }

  async function setSecurityPeriod(hours: number): Promise<void> {
    securityPeriod.value = hours
    try {
      const res = await apiClient.getSecuritySummary(hours)
      security.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
    } catch { /* keep stale data */ }
  }

  async function cancelCmd(id: string): Promise<void> {
    cancellingId.value = id
    try {
      await apiClient.cancelCommand(id)
      cmds.value = cmds.value.map((c) => (c.id === id ? { ...c, status: 'cancelled' } : c))
      addToast('Commande annulée', 'success')
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, 'Impossible d\'annuler'), 'error')
    } finally {
      cancellingId.value = null
    }
  }

  async function unblockIP(ip: string): Promise<void> {
    const ok = await dialog.confirm({
      title: 'Débloquer cette IP',
      message: `Retirer l'IP ${ip} de la liste noire ?`,
      variant: 'warning',
    })
    if (!ok) return
    unblockingIP.value = ip
    try {
      await apiClient.unblockIP(ip)
      const res = await apiClient.getSecuritySummary(securityPeriod.value)
      security.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
    } catch { /* ignore */ } finally {
      unblockingIP.value = ''
    }
  }

  onMounted(async () => {
    if (route.query.module) cmdModuleFilter.value = String(route.query.module)
    await fetchCmds()
    const cmdId = route.query.command
    if (cmdId) {
      try {
        const res = await apiClient.getCommandStatus(String(cmdId))
        if (res.data?.id) openLogViewer(res.data)
      } catch { /* ignore — command may not exist */ }
    }
  })
  onMounted(() => {
    auditPollTimer = setInterval(() => {
      if (activeTab.value === 'commandes') {
        const now = Date.now()
        const refreshMs = hasActiveCommands.value ? 5000 : 30000
        if (now - lastCmdFetchAt.value >= refreshMs) {
          fetchCmds()
        }
      } else if (auth.role === 'admin') {
        const now = Date.now()
        if (now - lastConnFetchAt.value >= 30000) {
          fetchConnexions()
        }
      }
    }, 5000)
  })

  onUnmounted(() => {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
    if (auditPollTimer) {
      clearInterval(auditPollTimer)
      auditPollTimer = null
    }
  })

  return {
    auth,
    canViewCommands,
    activeTab,
    switchToCommandes,
    switchToConnexions,
    switchToJournal,
    journalLogs,
    journalPage,
    journalLoading,
    journalExporting,
    journalHasMore,
    journalCategoryFilter,
    journalFrom,
    journalTo,
    selectJournalPage,
    exportJournal,
    cmds,
    cmdsPage,
    cmdsTotal,
    cmdsLoading,
    totalCmdsPages,
    cmdSearch,
    cmdStatusFilter,
    cmdModuleFilter,
    cmdSortBy,
    cmdSortDir,
    sortedCmds,
    toggleCmdSort,
    onSearchUpdate,
    onFilterChange,
    selectedCmd,
    showLogViewer,
    openLogViewer,
    closeLogViewer,
    cancellingId,
    cancelCmd,
    selectCmdsPage,
    connexions,
    connexionsPage,
    connexionsTotal,
    connexionsLoading,
    totalConnexionsPages,
    selectConnexionsPage,
    security,
    secPeriodOptions,
    securityPeriod,
    securityPeriodLabel,
    setSecurityPeriod,
    unblockingIP,
    unblockIP,
    moduleLabel,
    moduleClass,
    moduleFilterOptions,
    statusLabel,
    cmdLabel,
    formatDuration,
    statusClass,
  }
}

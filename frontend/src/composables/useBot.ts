import { computed, onMounted, onUnmounted, ref } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { addToast } from './useGlobalToast'
import { useHostsStore } from '../stores/hosts'
import type { WebLogIPTimelineRow } from '../types/security'

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for aggregate web-logs data (no Go model)
type AnyRecord = Record<string, any>

export function useBot() {
  const period = ref('24h')
  const periodOptions = [
    { value: '1h', label: '1h' },
    { value: '24h', label: '24h' },
    { value: '168h', label: '7j' },
    { value: '720h', label: '30j' },
  ]
  const hostsStore = useHostsStore()

  const source = ref('')
  const hostId = ref('')

  const loading = ref(false)
  const summary = ref<AnyRecord>({ threats: {} })
  const autoRefresh = ref(false)
  const lastUpdatedAt = ref<Date | null>(null)
  const BOT_REFRESH_SEC = 60
  let refreshTimer: ReturnType<typeof setInterval> | null = null

  const showTimeline = ref(false)
  const timelineLoading = ref(false)
  const banState = ref<'idle' | 'loading' | 'error'>('idle')
  const selectedIP = ref('')
  const timelineHostId = ref('')
  const timeline = ref<WebLogIPTimelineRow[]>([])

  const threats = computed(() => summary.value.threats || {})
  const topPaths = computed(() => threats.value.top_paths || [])
  const mostTargetedHosts = computed(() => threats.value.most_targeted_hosts || [])
  const ipHostMatrix = computed(() => threats.value.ip_host_matrix || [])
  const unblockedIPs = ref(new Set<string>())
  const rowState = ref<Record<string, 'loading' | 'error'>>({})
  const optimisticBans = ref<AnyRecord[]>([])

  const crowdSecIPs = computed(() => {
    const fromSnapshot = (threats.value.crowdsec_top_blocked || [] as AnyRecord[]).filter(
      (e: AnyRecord) => !unblockedIPs.value.has(e.ip as string),
    )
    const snapshotIPs = new Set(fromSnapshot.map((e: AnyRecord) => e.ip as string))
    const extra = optimisticBans.value.filter((e) => !snapshotIPs.has(e.ip as string) && !unblockedIPs.value.has(e.ip as string))
    return [...extra, ...fromSnapshot]
  })
  const crowdSecTotal = computed(() => Number(threats.value.crowdsec_blocked_ips) || 0)

  // topIPs enriched: merge CrowdSec decision type from the active decisions list so the
  // "Blocage" column reflects the current state even for IPs with no recent blocked requests.
  const topIPs = computed(() => {
    const decisionMap = new Map(crowdSecIPs.value.map((e: AnyRecord) => [e.ip as string, e]))
    return (threats.value.top_ips || [] as AnyRecord[]).map((ip: AnyRecord) => {
      const decision = decisionMap.get(ip.ip as string)
      if (!decision) return ip
      if (ip.blocked && ip.blocked_type) return ip  // already enriched by agent, trust it
      const decType = ((decision.type as string) || 'ban').toLowerCase()
      const isBan = decType === 'ban'
      return {
        ...ip,
        blocked: isBan || Boolean(decision.blocked_until),
        blocked_source: 'crowdsec',
        blocked_type: decType || 'ban',
        blocked_until: ip.blocked_until || decision.blocked_until,
      }
    })
  })
  // host_id du snapshot CrowdSec renvoyé par l'API (présent même sans filtre hôte)
  const crowdSecHostId = computed(() => (threats.value.crowdsec_host_id as string) || '')
  const isSelectedIPBlocked = computed(() =>
    crowdSecIPs.value.some((e: AnyRecord) => e.ip === selectedIP.value),
  )
  // host_id effectif : filtre manuel > host_id du snapshot CrowdSec > déduit des lignes de la timeline
  const effectiveHostId = computed(() => hostId.value || crowdSecHostId.value || timelineHostId.value)

  function levelClass(level: string): string {
    switch (level) {
      case 'CRITICAL': return 'bg-red-lt text-red'
      case 'HIGH': return 'bg-orange-lt text-orange'
      case 'MEDIUM': return 'bg-yellow-lt text-yellow'
      default: return 'bg-azure-lt text-azure'
    }
  }

  function decisionLabel(type: string | undefined | null): string {
    const t = (type || 'ban').toLowerCase().trim()
    if (!t) return 'Ban'
    switch (t) {
      case 'ban': return 'Ban'
      case 'captcha': return 'Captcha'
      case 'audit': return 'Audit'
      default: return t.charAt(0).toUpperCase() + t.slice(1)
    }
  }

  function decisionBadgeClass(type: string, blockedUntil?: string): string {
    if (!type) return 'bg-secondary-lt text-secondary'

    const t = type.toLowerCase()
    let baseClass = 'bg-secondary-lt text-secondary'
    switch (t) {
      case 'ban': baseClass = 'bg-red-lt text-red'; break
      case 'captcha': baseClass = 'bg-yellow-lt text-yellow'; break
      case 'audit': baseClass = 'bg-azure-lt text-azure'; break
    }

    // Si blockedUntil est fourni et valide, c'est un blocage temporaire → orange
    if (blockedUntil) {
      const d = new Date(blockedUntil)
      if (!Number.isNaN(d.getTime()) && d > new Date()) {
        return 'bg-orange-lt text-orange'  // blocage temporaire en orange
      }
    }

    return baseClass
  }

  function truncate(s: string, max: number): string {
    return s.length > max ? s.slice(0, max) + '…' : s
  }

  function formatBlockedUntil(blockedUntil?: string): string {
    if (!blockedUntil) return 'Bloquée'
    const d = new Date(blockedUntil)
    if (Number.isNaN(d.getTime())) return `Bloquée (date invalide: ${blockedUntil})`
    const now = new Date()
    if (d <= now) return 'Bloquée (permanent)'
    const diff = d.getTime() - now.getTime()
    const totalSeconds = Math.floor(diff / 1000)
    const seconds = totalSeconds % 60
    const totalMinutes = Math.floor(totalSeconds / 60)
    const minutes = totalMinutes % 60
    const totalHours = Math.floor(totalMinutes / 60)
    const hours = totalHours % 24
    const days = Math.floor(totalHours / 24)

    if (days > 0) return `Bloquée ${days}j ${hours}h`
    if (totalHours > 0) return `Bloquée ${totalHours}h ${minutes}m`
    if (totalMinutes > 0) return `Bloquée ${totalMinutes}m`
    return `Bloquée ${seconds}s`
  }

  async function loadThreats() {
    loading.value = true
    try {
      // BotView reads only `threats`; request the threats-only scope so the server
      // skips the heavy (unindexed) traffic aggregates + geolocation that would
      // otherwise time the request out on long windows.
      const res = await apiClient.getWebLogsSummary(period.value, hostId.value || undefined, source.value || undefined, 'threats')
      summary.value = { threats: res.data?.threats || {} }
      // Purger les bans optimistes dont le snapshot réel prend le relais
      const snapshotIPs = new Set((res.data?.threats?.crowdsec_top_blocked || []).map((e: AnyRecord) => e.ip as string))
      optimisticBans.value = optimisticBans.value.filter((e) => !snapshotIPs.has(e.ip as string))
    } catch (err) {
      console.error('Failed to load threats summary', err)
    } finally {
      loading.value = false
      lastUpdatedAt.value = new Date()
    }
  }

  function setPeriod(value: string) {
    if (period.value === value) return
    period.value = value
    void loadThreats()
  }

  async function openTimeline(ip: string) {
    selectedIP.value = ip
    timelineHostId.value = ''
    banState.value = 'idle'
    showTimeline.value = true
    timelineLoading.value = true
    try {
      const res = await apiClient.getIPTimeline(ip, hostId.value || undefined, period.value, 500)
      timeline.value = res.data?.requests || []
      const rows: AnyRecord[] = timeline.value
      if (rows.length > 0) {
        const first = rows[0].host_id as string
        if (first && rows.every((r) => r.host_id === first)) {
          timelineHostId.value = first
        }
      }
    } catch (err) {
      console.error('Failed to load IP timeline', err)
      timeline.value = []
    } finally {
      timelineLoading.value = false
    }
  }

  function closeTimeline() {
    showTimeline.value = false
    timeline.value = []
    selectedIP.value = ''
    timelineHostId.value = ''
  }

  async function handleBanFromModal(duration: string) {
    banState.value = 'loading'
    const ip = selectedIP.value
    try {
      const res = await apiClient.blockCrowdSecIP(ip, effectiveHostId.value, duration)
      const commandId: string = res.data?.command_id
      const ms = duration.endsWith('h') ? parseInt(duration) * 3600000 : parseInt(duration) * 60000
      optimisticBans.value = [
        ...optimisticBans.value.filter((e) => e.ip !== ip),
        { ip, type: 'ban', reason: 'manual', origin: 'cscli', blocked_until: new Date(Date.now() + ms).toISOString() },
      ]
      banState.value = 'idle'
      addToast(`IP ${ip} bloquée par CrowdSec (${duration})`, 'success')
      closeTimeline()
      const { status, output } = await pollCommand(commandId)
      if (status === 'failed') {
        optimisticBans.value = optimisticBans.value.filter((e) => e.ip !== ip)
        addToast(`Échec blocage ${ip} : ${output}`, 'error')
      }
    } catch (error) {
      banState.value = 'error'
      addToast(`Impossible de bloquer l'IP : ${getApiErrorMessage(error)}`, 'error')
    }
  }

  async function pollCommand(commandId: string): Promise<{ status: string; output: string }> {
    for (let i = 0; i < 40; i++) {
      await new Promise((r) => setTimeout(r, 1500))
      try {
        const res = await apiClient.getCommand(commandId)
        const { status, output } = res.data ?? {}
        if (status === 'completed' || status === 'failed') return { status, output: output ?? '' }
      } catch {
        // ignore transient poll errors
      }
    }
    // Timeout après 60s (2× le cycle agent) — la commande est probablement passée
    return { status: 'timeout', output: '' }
  }

  async function unblockCrowdSecEntry(ip: string) {
    const matchedEntry = crowdSecIPs.value.find((entry: AnyRecord) => entry.ip === ip)
    const targetHost = hostId.value || (matchedEntry?.host_id as string) || crowdSecHostId.value
    if (!targetHost) {
      addToast('Impossible de déterminer l\'hôte cible — renseigne le filtre Hôte', 'error')
      return
    }
    rowState.value = { ...rowState.value, [ip]: 'loading' }
    try {
      const res = await apiClient.unblockCrowdSecIP(ip, targetHost)
      const commandId: string = res.data?.command_id
      const { status, output } = await pollCommand(commandId)
      if (status === 'completed' || status === 'timeout') {
        const next = new Set(unblockedIPs.value)
        next.add(ip)
        unblockedIPs.value = next
        const { [ip]: _, ...rest } = rowState.value
        rowState.value = rest
        addToast(`IP ${ip} débloquée`, 'success')
      } else {
        rowState.value = { ...rowState.value, [ip]: 'error' }
        addToast(`Échec déblocage ${ip} : ${output}`, 'error')
      }
    } catch (error) {
      rowState.value = { ...rowState.value, [ip]: 'error' }
      addToast(`Impossible de débloquer l'IP : ${getApiErrorMessage(error)}`, 'error')
    }
  }

  onMounted(() => {
    hostsStore.fetchHosts()
    loadThreats()
    refreshTimer = setInterval(() => { if (autoRefresh.value) loadThreats() }, BOT_REFRESH_SEC * 1000)
  })
  onUnmounted(() => {
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
  })

  return {
    period,
    periodOptions,
    hostsStore,
    source,
    hostId,
    loading,
    autoRefresh,
    lastUpdatedAt,
    BOT_REFRESH_SEC,
    showTimeline,
    timelineLoading,
    banState,
    selectedIP,
    timeline,
    threats,
    topPaths,
    mostTargetedHosts,
    ipHostMatrix,
    rowState,
    crowdSecIPs,
    crowdSecTotal,
    topIPs,
    isSelectedIPBlocked,
    effectiveHostId,
    levelClass,
    decisionLabel,
    decisionBadgeClass,
    truncate,
    formatBlockedUntil,
    loadThreats,
    setPeriod,
    openTimeline,
    closeTimeline,
    handleBanFromModal,
    unblockCrowdSecEntry,
  }
}

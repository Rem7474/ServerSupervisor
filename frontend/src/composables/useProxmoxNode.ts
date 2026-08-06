/* eslint-disable @typescript-eslint/no-explicit-any --
 * Verbatim move of ProxmoxNodeView.vue's business logic (see eslint.config.js's
 * TEMP Phase 7 exemption for the view itself). The guest/link/task/RRD payloads
 * carry runtime fields beyond the generated Proxmox models; typed properly when
 * this domain gets its own typed models in a follow-up. */
import { ref, computed, shallowRef, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import { getApiErrorMessage } from '../api/client'
import { useProxmoxGuestActions, type GuestPowerAction } from './useProxmoxGuestActions'

export function useProxmoxNode() {
  const route = useRoute()
  const router = useRouter()
  const guestActions = useProxmoxGuestActions()
  const node = ref<any>(null)
  const loading = ref(true)
  const error = ref('')
  const tab = ref('vms')
  watch(tab, (t) => {
    router.replace({ query: { ...route.query, tab: t } })
  })

  const guestLinks = ref<Record<string, any>>({})
  const linkMsg = ref('')
  const linkMsgOk = ref(false)

  const sensorSourceCandidates = ref<any[]>([])
  const sensorSourceHostId = ref('')
  const sensorSourceLoading = ref(false)
  const sensorSourceSaving = ref(false)
  const sensorSourceMsg = ref('')
  const sensorSourceOk = ref(false)
  const sensorSourceHostName = computed(() =>
    node.value?.cpu_temp_source_host_name || node.value?.fan_rpm_source_host_name || ''
  )

  const nodeTempLoading = ref(false)
  const nodeTempError = ref('')
  const nodeTempChart = shallowRef<any>(null)
  const nodeCpuTempCurrent = ref(0)

  const nodeFanLoading = ref(false)
  const nodeFanError = ref('')
  const nodeFanChart = shallowRef<any>(null)
  const nodeFanRPMCurrent = ref(0)

  // apt refresh
  const aptRefreshing = ref(false)
  const aptRefreshMsg = ref('')
  const aptRefreshOk = ref(false)

  // peer nodes for migration target list
  const peerNodes = ref<any[]>([])

  const migrateModal = ref<any>({
    open: false,
    guest: null,
    guestType: 'vm',
    target: '',
    online: false,
    loading: false,
    error: '',
  })

  const liveStatus = ref<any>(null)
  const liveStatusLoading = ref(false)
  const lastUpdatedAt = ref<Date | null>(null)
  const liveStatusError = ref('')
  const autoRefresh = ref(true)
  const LIVE_STATUS_REFRESH_SEC = 60

  // RRD charts
  const rrdTimeframe = ref('hour')
  const rrdTimeframeToHours: Record<string, number> = {
    hour: 1,
    day: 24,
    week: 24 * 7,
    month: 24 * 30,
    year: 24 * 365,
  }
  const rrdCpuChart = shallowRef<any>(null)
  const rrdRamChart = shallowRef<any>(null)
  const rrdIowaitChart = shallowRef<any>(null)
  const rrdNetChart = shallowRef<any>(null)
  const rrdLoading = ref(false)
  const rrdError = ref('')

  const showConsole = ref(false)
  const liveTask = ref<any>(null)
  const activeUpid = ref<string | null>(null)
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let liveStatusTimer: ReturnType<typeof setInterval> | null = null

  const guestNetworks = ref<Record<string, any[]>>({})
  const guestNetworksLoading = ref(false)

  async function loadGuestNetworks(): Promise<void> {
    if (guestNetworksLoading.value || Object.keys(guestNetworks.value).length > 0) return
    guestNetworksLoading.value = true
    try {
      const res = await api.getProxmoxNodeGuestNetworks(String(route.params.id))
      guestNetworks.value = res.data ?? {}
    } catch { /* non-bloquant */ }
    finally { guestNetworksLoading.value = false }
  }

  // NPM domains routing to each guest's own IP(s), keyed by vmid — same
  // lazy/load-once-per-mount pattern as guestNetworks above.
  const guestExposure = ref<Record<string, any>>({})
  const guestExposureLoading = ref(false)

  async function loadGuestExposure(): Promise<void> {
    if (guestExposureLoading.value || Object.keys(guestExposure.value).length > 0) return
    guestExposureLoading.value = true
    try {
      const res = await api.getProxmoxNodeGuestExposure(String(route.params.id))
      guestExposure.value = res.data ?? {}
    } catch { /* non-bloquant */ }
    finally { guestExposureLoading.value = false }
  }

  // services
  const services = ref<any[]>([])
  const servicesLoading = ref(false)
  const servicesError = ref('')
  const svcActionMsg = ref('')
  const svcActionOk = ref(false)
  const svcActionLoading = ref<Record<string, string | null>>({})

  const vms = computed(() => node.value?.guests?.filter((g: any) => g.guest_type === 'vm') ?? [])
  const lxcs = computed(() => node.value?.guests?.filter((g: any) => g.guest_type === 'lxc') ?? [])

  // "Dernier résultat par VM" — the backend (poller/db/service/handler/route)
  // already derives this from vzdump tasks; only the frontend consumer was
  // missing. Same lazy/load-once-per-mount pattern as guestNetworks/guestExposure
  // above, feeding both the guest-list column and the node's "Sauvegarde" tab.
  const backupRuns = ref<any[]>([])
  const backupRunsLoading = ref(false)

  async function loadBackupRuns(): Promise<void> {
    if (backupRunsLoading.value || backupRuns.value.length > 0) return
    const connectionId = node.value?.connection_id
    const nodeName = node.value?.node_name
    if (!connectionId) return
    backupRunsLoading.value = true
    try {
      const res = await api.getProxmoxBackupRuns(connectionId)
      backupRuns.value = (res.data ?? []).filter((r: any) => r.node_name === nodeName)
    } catch { /* non-bloquant */ }
    finally { backupRunsLoading.value = false }
  }

  const backupRunsByVmid = computed(() => {
    const map: Record<number, any> = {}
    for (const r of backupRuns.value) map[r.vmid] = r
    return map
  })

  // Re-fetches only the guest statuses after a start/shutdown/reboot — a full
  // load() also re-triggers sensor/live-status/RRD/peer-node fetches and
  // flips loading back to true (full-page skeleton), which is overkill for
  // "did this one guest's status change".
  async function refreshGuests(): Promise<void> {
    try {
      const res = await api.getProxmoxNode(String(route.params.id))
      if (node.value) node.value.guests = res.data?.guests ?? node.value.guests
    } catch {
      // best-effort; the next manual/periodic refresh will retry
    }
  }

  async function handleGuestAction(guest: any, action: GuestPowerAction): Promise<void> {
    await guestActions.performGuestAction(guest, action, refreshGuests)
  }
  const failedTaskCount = computed(() =>
    (node.value?.tasks ?? []).filter((t: any) => t.status === 'stopped' && t.exit_status && t.exit_status !== 'OK').length
  )
  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const requestedTab = String(route.query.tab || '')
      const validTabs = ['vms', 'lxc', 'storage', 'disks', 'tasks', 'updates', 'services', 'security', 'backups']
      if (validTabs.includes(requestedTab)) {
        tab.value = requestedTab
      }
      const res = await api.getProxmoxNode(String(route.params.id))
      node.value = res.data
      sensorSourceHostId.value = node.value?.cpu_temp_source_host_id || node.value?.fan_rpm_source_host_id || ''
      await loadSensorSourceCandidates()
      await loadGuestLinks()
      // fire-and-forget: live status + RRD charts + peer nodes load in parallel
      loadLiveStatus()
      loadRRD('hour')
      loadPeerNodes()
      // A tab restored from a ?tab= deep link (page refresh / direct link)
      // never goes through the shell's click handler (ProxmoxNodeView's
      // onTabClick) — the only other place these lazy per-tab loaders were
      // triggered from — so VMs/LXC's per-guest network/exposure info (IP,
      // domains) and the Services list silently stayed empty after a hard
      // refresh landing directly on one of those tabs. Mirror onTabClick's
      // own logic here for whichever tab was actually restored.
      if (tab.value === 'vms' || tab.value === 'lxc') { loadGuestNetworks(); loadGuestExposure(); loadBackupRuns() }
      else if (tab.value === 'services') loadServices()
      else if (tab.value === 'backups') loadBackupRuns()
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur lors du chargement.')
    } finally {
      loading.value = false
    }
  }

  async function loadNodeCpuTempHistory(hours: number = rrdTimeframeToHours[rrdTimeframe.value] ?? 24): Promise<void> {
    nodeTempLoading.value = true
    nodeTempError.value = ''
    nodeTempChart.value = null
    nodeCpuTempCurrent.value = 0

    try {
      const sourceHostId = sensorSourceHostId.value || node.value?.cpu_temp_source_host_id || node.value?.fan_rpm_source_host_id
      if (!sourceHostId) {
        return
      }

      const res = await api.getProxmoxNodeCpuTempHistory(String(route.params.id), hours)
      const points = (Array.isArray(res.data) ? res.data : []).filter((p: any) => Number(p?.cpu_temperature) > 0)
      if (!points.length) {
        return
      }

      const labels = points.map((p: any) => {
        const d = new Date(p.timestamp)
        if (hours <= 24) {
          return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
        }
        return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
      })
      const data = points.map((p: any) => Number(p.cpu_temperature))
      nodeCpuTempCurrent.value = data[data.length - 1] || 0
      nodeTempChart.value = {
        labels,
        datasets: [{
          data,
          borderColor: cssVar('--tblr-red'),
          backgroundColor: `rgba(${cssVar('--tblr-red-rgb')},0.12)`,
          fill: true,
          tension: 0.3,
          spanGaps: true,
        }],
      }
    } catch (e: unknown) {
      nodeTempError.value = getApiErrorMessage(e, 'Erreur lors du chargement de la température CPU.')
    } finally {
      nodeTempLoading.value = false
    }
  }

  async function loadNodeFanRPMHistory(hours: number = rrdTimeframeToHours[rrdTimeframe.value] ?? 24): Promise<void> {
    nodeFanLoading.value = true
    nodeFanError.value = ''
    nodeFanChart.value = null
    nodeFanRPMCurrent.value = 0

    try {
      const sourceHostId = sensorSourceHostId.value || node.value?.fan_rpm_source_host_id || node.value?.cpu_temp_source_host_id
      if (!sourceHostId) {
        return
      }

      const res = await api.getProxmoxNodeFanRPMHistory(String(route.params.id), hours)
      const points = (Array.isArray(res.data) ? res.data : []).filter((p: any) => Number(p?.fan_rpm) > 0)
      if (!points.length) {
        return
      }

      const labels = points.map((p: any) => {
        const d = new Date(p.timestamp)
        if (hours <= 24) {
          return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
        }
        return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
      })
      const data = points.map((p: any) => Number(p.fan_rpm))
      nodeFanRPMCurrent.value = data[data.length - 1] || 0
      nodeFanChart.value = {
        labels,
        datasets: [{
          data,
          borderColor: cssVar('--tblr-azure'),
          backgroundColor: `rgba(${cssVar('--tblr-azure-rgb')},0.12)`,
          fill: true,
          tension: 0.3,
          spanGaps: true,
        }],
      }
    } catch (e: unknown) {
      nodeFanError.value = getApiErrorMessage(e, 'Erreur lors du chargement des RPM ventilateurs.')
    } finally {
      nodeFanLoading.value = false
    }
  }

  async function loadSensorSourceCandidates(): Promise<void> {
    sensorSourceLoading.value = true
    try {
      const res = await api.getProxmoxNodeSensorSourceCandidates(String(route.params.id))
      sensorSourceCandidates.value = Array.isArray(res.data) ? res.data : []
    } catch {
      sensorSourceCandidates.value = []
    } finally {
      sensorSourceLoading.value = false
    }
  }

  async function refreshNodeSensorSource() {
    try {
      const res = await api.getProxmoxNode(String(route.params.id))
      const n = res.data || {}
      if (node.value) {
        node.value.cpu_temp_source_host_id = n.cpu_temp_source_host_id || ''
        node.value.cpu_temp_source_host_name = n.cpu_temp_source_host_name || ''
        node.value.fan_rpm_source_host_id = n.fan_rpm_source_host_id || ''
        node.value.fan_rpm_source_host_name = n.fan_rpm_source_host_name || ''
      }
      sensorSourceHostId.value = n.cpu_temp_source_host_id || n.fan_rpm_source_host_id || ''
    } catch {
      // non-bloquant
    }
  }

  async function saveSensorSource() {
    sensorSourceSaving.value = true
    sensorSourceMsg.value = ''
    try {
      const target = sensorSourceHostId.value || null
      const res = await api.setProxmoxNodeSensorSource(String(route.params.id), target)
      if (node.value) {
        node.value.cpu_temp_source_host_id = res.data?.cpu_temp_source_host_id || ''
        node.value.cpu_temp_source_host_name = res.data?.cpu_temp_source_host_name || ''
        node.value.fan_rpm_source_host_id = res.data?.fan_rpm_source_host_id || ''
        node.value.fan_rpm_source_host_name = res.data?.fan_rpm_source_host_name || ''
      }
      sensorSourceHostId.value = res.data?.cpu_temp_source_host_id || res.data?.fan_rpm_source_host_id || ''
      await loadSensorSourceCandidates()
      await loadNodeCpuTempHistory(rrdTimeframeToHours[rrdTimeframe.value] ?? 24)
      await loadNodeFanRPMHistory(rrdTimeframeToHours[rrdTimeframe.value] ?? 24)
      sensorSourceMsg.value = 'Source capteurs mise à jour (CPU + ventilateurs).'
      sensorSourceOk.value = true
    } catch (e: unknown) {
      sensorSourceMsg.value = getApiErrorMessage(e, 'Erreur lors de la mise à jour.')
      sensorSourceOk.value = false
    } finally {
      sensorSourceSaving.value = false
      setTimeout(() => { sensorSourceMsg.value = '' }, 4000)
    }
  }

  async function loadGuestLinks(): Promise<void> {
    const guests = node.value?.guests ?? []
    if (guests.length === 0) return
    try {
      const res = await api.getProxmoxLinks()
      const guestIds = new Set(guests.map((g: any) => g.id))
      const map: Record<string, any> = {}
      for (const link of res.data ?? []) {
        if (guestIds.has(link.guest_id)) {
          map[link.guest_id] = link
        }
      }
      guestLinks.value = map
    } catch {
      guestLinks.value = {}
    }
  }

  function linkForGuest(g: any): any {
    return guestLinks.value[g.id] ?? null
  }

  async function confirmGuestLink(g: any): Promise<void> {
    const link = linkForGuest(g)
    if (!link) return
    try {
      const res = await api.updateProxmoxLink(link.id, { status: 'confirmed' })
      guestLinks.value = { ...guestLinks.value, [g.id]: res.data }
      await loadSensorSourceCandidates()
      await refreshNodeSensorSource()
      showMsg(`[${g.name}] Lien confirmé.`, true)
    } catch (e: unknown) {
      showMsg(getApiErrorMessage(e, 'Erreur.'), false)
    }
  }

  async function ignoreGuestLink(g: any): Promise<void> {
    const link = linkForGuest(g)
    if (!link) return
    try {
      await api.deleteProxmoxLink(link.id)
      const m = { ...guestLinks.value }
      delete m[g.id]
      guestLinks.value = m
      showMsg(`[${g.name}] Suggestion ignorée.`, true)
    } catch (e: unknown) {
      showMsg(getApiErrorMessage(e, 'Erreur.'), false)
    }
  }

  // Guests linked to a ServerSupervisor host already get their domain/IP
  // correlation for free from that host's own Exposition tab (same IP, same
  // GetHostExposure query) — land there directly instead of the overview tab.
  function goToHost(link: any): void {
    if (link?.host_id) router.push(`/hosts/${link.host_id}?tab=exposition`)
  }

  function showMsg(msg: string, ok: boolean): void {
    linkMsg.value = msg
    linkMsgOk.value = ok
    setTimeout(() => { linkMsg.value = '' }, 4000)
  }

  async function loadRRD(timeframe: string = rrdTimeframe.value): Promise<void> {
    rrdTimeframe.value = timeframe
    void loadNodeCpuTempHistory(rrdTimeframeToHours[timeframe] ?? 24)
    void loadNodeFanRPMHistory(rrdTimeframeToHours[timeframe] ?? 24)
    rrdLoading.value = true
    rrdError.value = ''
    try {
      const res = await api.getProxmoxNodeRRD(String(route.params.id), timeframe)
      buildRRDCharts(res.data ?? [], timeframe)
    } catch (e: unknown) {
      rrdError.value = getApiErrorMessage(e, 'Erreur lors du chargement des métriques.')
      rrdCpuChart.value = null
      rrdRamChart.value = null
      rrdIowaitChart.value = null
      rrdNetChart.value = null
    } finally {
      rrdLoading.value = false
    }
  }

  function cssVar(name: string): string {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  }

  function buildRRDCharts(points: any[], timeframe: string): void {
    const labels = points.map((p: any) => {
      const d = new Date(p.time * 1000)
      if (timeframe === 'hour' || timeframe === 'day')
        return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      if (timeframe === 'week')
        return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' }) + ' ' + d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      return d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
    })

    rrdCpuChart.value = {
      labels,
      datasets: [{
        data: points.map((p: any) => p.cpu != null ? p.cpu * 100 : null),
        borderColor: cssVar('--tblr-blue'), backgroundColor: `rgba(${cssVar('--tblr-blue-rgb')},0.1)`,
        fill: true, tension: 0.3, spanGaps: true,
      }],
    }

    // RAM: memused / memtotal are raw bytes from PVE RRD (JSON keys: memused, memtotal)
    const ramData = points.map((p: any) =>
      (p.memused != null && p.memtotal != null && p.memtotal > 0)
        ? (p.memused / p.memtotal) * 100
        : null
    )
    rrdRamChart.value = ramData.some((v: any) => v != null) ? {
      labels,
      datasets: [{
        data: ramData,
        borderColor: cssVar('--tblr-green'), backgroundColor: `rgba(${cssVar('--tblr-green-rgb')},0.1)`,
        fill: true, tension: 0.3, spanGaps: true,
      }],
    } : null

    const hasIowait = points.some((p: any) => p.iowait != null)
    rrdIowaitChart.value = hasIowait ? {
      labels,
      datasets: [{
        data: points.map((p: any) => p.iowait != null ? p.iowait * 100 : null),
        borderColor: cssVar('--tblr-yellow'), backgroundColor: `rgba(${cssVar('--tblr-yellow-rgb')},0.1)`,
        fill: true, tension: 0.3, spanGaps: true,
      }],
    } : null

    const hasNet = points.some((p: any) => p.netin != null || p.netout != null)
    rrdNetChart.value = hasNet ? {
      labels,
      datasets: [
        {
          label: 'Entrante',
          data: points.map((p: any) => p.netin ?? null),
          borderColor: cssVar('--tblr-indigo'), backgroundColor: `rgba(${cssVar('--tblr-indigo-rgb')},0.1)`,
          fill: true, tension: 0.3, spanGaps: true,
        },
        {
          label: 'Sortante',
          data: points.map((p: any) => p.netout ?? null),
          borderColor: cssVar('--tblr-pink'), backgroundColor: `rgba(${cssVar('--tblr-pink-rgb')},0.05)`,
          fill: false, tension: 0.3, spanGaps: true,
        },
      ],
    } : null
  }

  async function loadLiveStatus(): Promise<void> {
    liveStatusLoading.value = true
    liveStatusError.value = ''
    try {
      const res = await api.getProxmoxNodeStatus(String(route.params.id))
      liveStatus.value = res.data
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      const ax = e as { response?: { data?: { error?: string }; status?: number } }
      liveStatusError.value = ax.response?.data?.error || `Erreur ${ax.response?.status ?? ''} — vérifiez la connectivité au nœud.`
    } finally {
      liveStatusLoading.value = false
    }
  }


  function stopPolling(): void {
    if (pollTimer) clearTimeout(pollTimer)
    pollTimer = null
  }

  function closeConsole(): void {
    stopPolling()
    showConsole.value = false
    liveTask.value = null
    activeUpid.value = null
  }

  async function startPollingTask(upid: string, { action = '', label = '' }: { action?: string; label?: string } = {}): Promise<void> {
    stopPolling()
    activeUpid.value = upid
    liveTask.value = {
      host_name: node.value?.node_name ?? '',
      module: 'proxmox',
      action: action || upid,
      target: label || '',   // short display label, not the raw UPID
      status: 'running',
      output: '',
    }
    showConsole.value = true

    const poll = async (): Promise<void> => {
      try {
        const res = await api.getProxmoxTaskLog(String(route.params.id), upid)
        const lines = (res.data ?? []).map((l: any) => l.t).join('\n')
        const lastLine = res.data?.[res.data.length - 1]?.t ?? ''
        const done = lastLine.startsWith('TASK OK') || lastLine.startsWith('TASK ERROR')
        const status = done
          ? (lastLine.startsWith('TASK OK') ? 'completed' : 'failed')
          : 'running'
        liveTask.value = { ...liveTask.value, output: lines, status }
        if (!done) pollTimer = setTimeout(poll, 2000)
      } catch {
        pollTimer = setTimeout(poll, 3000)
      }
    }
    await poll()
  }

  async function triggerAptRefresh(): Promise<void> {
    aptRefreshing.value = true
    aptRefreshMsg.value = ''
    try {
      const res = await api.refreshProxmoxNodeApt(String(route.params.id))
      const upid = res.data?.upid
      aptRefreshMsg.value = upid ? 'Tâche lancée — logs en cours…' : (res.data?.message || 'Tâche lancée.')
      aptRefreshOk.value = true
      if (upid) startPollingTask(upid, { action: 'apt update' })
    } catch (e: unknown) {
      aptRefreshMsg.value = getApiErrorMessage(e, 'Erreur lors du lancement de apt update.')
      aptRefreshOk.value = false
    } finally {
      aptRefreshing.value = false
      setTimeout(() => { aptRefreshMsg.value = '' }, 6000)
    }
  }

  async function loadServices(): Promise<void> {
    if (servicesLoading.value || services.value.length > 0) return
    servicesLoading.value = true
    servicesError.value = ''
    try {
      const res = await api.getProxmoxNodeServices(String(route.params.id))
      services.value = res.data ?? []
    } catch (e: unknown) {
      servicesError.value = getApiErrorMessage(e, 'Erreur lors du chargement des services.')
    } finally {
      servicesLoading.value = false
    }
  }

  async function svcAction(name: string, action: string): Promise<void> {
    svcActionMsg.value = ''
    svcActionLoading.value[name] = action
    try {
      const res = await api.proxmoxNodeServiceAction(String(route.params.id), name, action)
      const upid = res.data?.upid
      svcActionMsg.value = upid ? `${action} ${name} lancé — logs en cours…` : `${action} ${name} lancé.`
      svcActionOk.value = true
      if (upid) startPollingTask(upid, { action: `service ${action}`, label: name })
      else setTimeout(() => loadServices(), 2000)
    } catch (e: unknown) {
      svcActionMsg.value = getApiErrorMessage(e, `Erreur lors de ${action} ${name}.`)
      svcActionOk.value = false
    } finally {
      svcActionLoading.value[name] = null
    }
    setTimeout(() => { svcActionMsg.value = '' }, 6000)
  }

  async function loadPeerNodes(): Promise<void> {
    if (!node.value?.connection_id) return
    try {
      const res = await api.getProxmoxNodes(node.value.connection_id)
      peerNodes.value = (res.data ?? []).filter((n: any) => n.node_name !== node.value?.node_name && n.status === 'online')
    } catch {
      peerNodes.value = []
    }
  }

  function openMigrateModal(guest: any, guestType: string = 'vm'): void {
    migrateModal.value = {
      open: true,
      guest,
      guestType,
      target: peerNodes.value[0]?.node_name ?? '',
      online: false,
      loading: false,
      error: '',
    }
  }

  async function submitMigration(): Promise<void> {
    const m = migrateModal.value
    if (!m.target || !m.guest) return
    m.loading = true
    m.error = ''
    try {
      const res = await api.migrateProxmoxGuest(String(route.params.id), m.guest.vmid, {
        target: m.target,
        guest_type: m.guestType,
        online: m.online,
      })
      const upid = res.data?.upid
      migrateModal.value.open = false
      if (upid) {
        startPollingTask(upid, { action: 'migrate', label: `${m.guest.name || m.guest.vmid} → ${m.target}` })
      }
    } catch (e: unknown) {
      m.error = getApiErrorMessage(e, 'Erreur lors du lancement de la migration.')
    } finally {
      m.loading = false
    }
  }

  onMounted(() => {
    load()
    liveStatusTimer = setInterval(() => { if (autoRefresh.value) loadLiveStatus() }, LIVE_STATUS_REFRESH_SEC * 1000)
  })
  onUnmounted(() => {
    stopPolling()
    if (liveStatusTimer) clearInterval(liveStatusTimer)
  })

  return {
    node,
    loading,
    error,
    tab,
    guestLinks,
    linkMsg,
    linkMsgOk,
    sensorSourceCandidates,
    sensorSourceHostId,
    sensorSourceLoading,
    sensorSourceSaving,
    sensorSourceMsg,
    sensorSourceOk,
    sensorSourceHostName,
    saveSensorSource,
    nodeTempLoading,
    nodeTempError,
    nodeTempChart,
    nodeCpuTempCurrent,
    nodeFanLoading,
    nodeFanError,
    nodeFanChart,
    nodeFanRPMCurrent,
    aptRefreshing,
    aptRefreshMsg,
    aptRefreshOk,
    triggerAptRefresh,
    peerNodes,
    migrateModal,
    openMigrateModal,
    submitMigration,
    liveStatus,
    liveStatusLoading,
    lastUpdatedAt,
    liveStatusError,
    autoRefresh,
    LIVE_STATUS_REFRESH_SEC,
    rrdTimeframe,
    rrdCpuChart,
    rrdRamChart,
    rrdIowaitChart,
    rrdNetChart,
    rrdLoading,
    rrdError,
    loadRRD,
    showConsole,
    liveTask,
    activeUpid,
    closeConsole,
    startPollingTask,
    guestNetworks,
    guestNetworksLoading,
    loadGuestNetworks,
    guestExposure,
    guestExposureLoading,
    loadGuestExposure,
    services,
    servicesLoading,
    servicesError,
    svcActionMsg,
    svcActionOk,
    svcActionLoading,
    loadServices,
    svcAction,
    vms,
    lxcs,
    failedTaskCount,
    backupRuns,
    backupRunsLoading,
    backupRunsByVmid,
    loadBackupRuns,
    confirmGuestLink,
    ignoreGuestLink,
    goToHost,
    guestActionLoading: guestActions.actionLoading,
    handleGuestAction,
  }
}

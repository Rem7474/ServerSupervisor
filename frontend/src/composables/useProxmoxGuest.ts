import { computed, ref, shallowRef, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useRoute } from 'vue-router'
import type { ApexOptions } from 'apexcharts'
import dayjs from '../utils/dayjs'
import api from '../api'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import { useProxmoxGuestActions, type GuestPowerAction } from './useProxmoxGuestActions'
import { getApexChartPalette } from '../utils/apexChartTheme'
import { getMinPointTimestamp, getMaxPointTimestamp } from '../utils/chartTimeAxis'
import type { ProxmoxGuestLink } from '../types/generated'

export type { GuestPowerAction }

const GUEST_REFRESH_SEC = 30

interface GuestMetricPoint { timestamp: string; cpu_avg?: number; memory_avg?: number; [key: string]: unknown }
interface ChartPoint { x: number; y: number }
type GuestChartSeries = { name: string; data: ChartPoint[]; color: string }[]

interface ProxmoxGuest {
  id: string
  connection_id: string
  node_name: string
  guest_type: string
  vmid: number
  name: string
  status: string
  cpu_alloc: number
  cpu_usage: number
  mem_alloc: number
  mem_usage: number
  disk_alloc: number
  disk_usage: number
  uptime: number
}

interface GuestNetworkIface { name: string; ips: string[] }

// vue3-apexcharts' own exposed instance API (its .d.ts declares this but doesn't
// export it under a name that resolves cleanly through defineAsyncComponent's
// template-ref typing — restated locally for the one method actually needed).
export interface ApexChartInstance {
  updateOptions(options: ApexOptions, redrawPaths?: boolean, animate?: boolean, updateSyncedCharts?: boolean): Promise<void>
}

// chartRef is owned by the view (bound via `ref="chartRef"` on its
// <ApexChart>) and passed in here rather than being created and returned by
// this composable, so the view's own script actually reads the binding
// (passing it as an argument counts) instead of only handing it to the
// template — vue-tsc's noUnusedLocals doesn't trace string `ref="x"`
// template bindings as a "read" of `x`, so a destructure-and-template-only
// version of this trips a false "declared but never read".
export function useProxmoxGuest(chartRef: Ref<ApexChartInstance | null>) {
  const route = useRoute()
  const signal = useAbortSignal()
  const guestActions = useProxmoxGuestActions()
  const guest = ref<ProxmoxGuest | null>(null)
  const guestLink = ref<ProxmoxGuestLink | null>(null)
  const loading = ref(true)
  const summaryLoading = ref(false)
  const error = ref('')
  const hours = ref(24)
  const series = shallowRef<GuestChartSeries | null>(null)
  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  const actionLoading = computed(() => guestActions.isLoading(guest.value?.id))

  // Full per-interface network detail (all interfaces, all IPs) — only the
  // primary ethX IP is shown in the KPI row; this backs the collapsible
  // "detail réseau" section. Keyed by node, same endpoint the node's guest
  // table uses, since there is no single-guest network endpoint — see
  // nodeId below for why this depends on resolving the node's DB id.
  const guestNetworks = ref<GuestNetworkIface[]>([])
  const guestNetworksLoading = ref(false)

  // The node's own DB id — needed for the guest-networks fetch below *and*
  // the view's "back to node" breadcrumb link. Resolved once per mount (see
  // resolveNodeId), so both consumers share the same value instead of each
  // re-deriving it (or, before this existed, the breadcrumb link reading
  // route.query.nodeId directly and breaking exactly like the network fetch
  // used to for any guest reached without that query param).
  const nodeId = ref('')

  const linkSaving = ref(false)
  const linkMsg = ref('')
  const linkMsgOk = ref(false)

  function bucketMinutesFor(inputHours: number): number {
    if (inputHours <= 6) return 1
    if (inputHours <= 24) return 5
    if (inputHours <= 168) return 15
    return 60
  }

  async function loadGuest(): Promise<void> {
    // Only show the full-page skeleton on the very first load — periodic
    // auto-refresh ticks update guest/guestLink in place instead of flashing
    // the whole page back to a loading state every GUEST_REFRESH_SEC.
    if (!guest.value) loading.value = true
    error.value = ''
    try {
      const res = await api.getProxmoxGuests(undefined, signal)
      const list = Array.isArray(res.data) ? res.data : []
      const found = list.find((g: ProxmoxGuest) => g.id === route.params.id)
      if (!found) {
        error.value = 'Guest introuvable'
        return
      }
      guest.value = found
      const linkRes = await api.getProxmoxGuestLink(found.id, signal)
      guestLink.value = linkRes.data
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, 'Erreur lors du chargement du guest Proxmox.')
    } finally {
      loading.value = false
    }
  }

  // The node-drilldown guests tab passes the node's DB id via ?nodeId=, but
  // every other link to a guest (dashboard, host-detail Proxmox panel,
  // network graph, command palette, a bookmark) doesn't carry it — those used
  // to just show an empty network/IP section, and the breadcrumb's "back to
  // node" link used to point at /proxmox/nodes/ (no id) instead. Resolve it
  // ourselves from the guest's own connection_id/node_name when the query
  // param is absent, instead of depending on the user having drilled in via
  // /proxmox first.
  async function resolveNodeId(): Promise<void> {
    const fromQuery = String(route.query.nodeId || '')
    if (fromQuery) {
      nodeId.value = fromQuery
      return
    }
    if (!guest.value) return
    try {
      const res = await api.getProxmoxNodes(guest.value.connection_id, signal)
      const nodes = Array.isArray(res.data) ? res.data : []
      nodeId.value = nodes.find((n) => n.node_name === guest.value?.node_name)?.id || ''
    } catch {
      nodeId.value = ''
    }
  }

  // The node's guest-networks endpoint is the only source for per-guest
  // interface detail — there's no single-guest equivalent — so this needs
  // nodeId resolved first.
  async function loadGuestNetworks(): Promise<void> {
    if (!guest.value) return
    await resolveNodeId()
    if (!nodeId.value || !guest.value) return
    guestNetworksLoading.value = true
    try {
      const res = await api.getProxmoxNodeGuestNetworks(nodeId.value)
      guestNetworks.value = res.data?.[guest.value.vmid] ?? []
    } catch {
      guestNetworks.value = []
    } finally {
      guestNetworksLoading.value = false
    }
  }

  async function confirmLink(): Promise<void> {
    if (!guestLink.value) return
    linkSaving.value = true
    try {
      const res = await api.updateProxmoxLink(guestLink.value.id, { status: 'confirmed' })
      guestLink.value = res.data
      linkMsg.value = 'Lien confirmé.'
      linkMsgOk.value = true
    } catch (e: unknown) {
      linkMsg.value = getApiErrorMessage(e, 'Erreur.')
      linkMsgOk.value = false
    } finally {
      linkSaving.value = false
      setTimeout(() => { linkMsg.value = '' }, 4000)
    }
  }

  async function ignoreLink(): Promise<void> {
    if (!guestLink.value) return
    linkSaving.value = true
    try {
      await api.deleteProxmoxLink(guestLink.value.id)
      guestLink.value = null
      linkMsg.value = 'Suggestion ignorée.'
      linkMsgOk.value = true
    } catch (e: unknown) {
      linkMsg.value = getApiErrorMessage(e, 'Erreur.')
      linkMsgOk.value = false
    } finally {
      linkSaving.value = false
      setTimeout(() => { linkMsg.value = '' }, 4000)
    }
  }

  function formatChartTime(timestampMs: number): string {
    const d = dayjs(timestampMs)
    if (!d.isValid()) return ''
    return hours.value >= 24 ? d.format('DD/MM HH:mm') : d.format('HH:mm')
  }

  // Built once (on first data load) rather than as a `computed` over
  // `series`: vue3-apexcharts clones the whole `:options` prop via
  // JSON.parse(JSON.stringify(...)) on every *reactive* update after mount
  // — which silently drops every function (labels.formatter, tooltip.custom)
  // since JSON can't represent them. Keeping this object's identity stable
  // after the initial build means later data refreshes (this guest polls
  // every GUEST_REFRESH_SEC, plus manual changeRange() calls) flow through
  // the (function-free, always-safe) `:series` prop only; the time-range
  // window (xaxis.min/max) is pushed via the exposed updateOptions() method
  // directly in loadGuestSummary() instead (bypasses the wrapper's buggy
  // reactive watcher).
  const chartOptions = shallowRef<ApexOptions | null>(null)

  function buildChartOptions(): ApexOptions {
    const palette = getApexChartPalette()
    const allPoints = (series.value ?? []).flatMap((s) => s.data)
    return {
      chart: { type: 'area', background: 'transparent', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
      theme: { mode: 'dark' },
      fill: { type: 'solid', opacity: 0.1 },
      stroke: { curve: 'smooth', width: 2 },
      markers: { size: 0, hover: { size: 5 } },
      dataLabels: { enabled: false },
      legend: { show: true, position: 'top', labels: { colors: palette.legendText }, markers: { size: 5 } },
      grid: { borderColor: palette.grid },
      xaxis: {
        type: 'datetime',
        min: getMinPointTimestamp(allPoints),
        max: getMaxPointTimestamp(allPoints),
        labels: { style: { colors: palette.tickText }, formatter: (v: string) => formatChartTime(Number(v)) },
        axisBorder: { show: false },
        axisTicks: { show: false },
        tooltip: { enabled: false },
      },
      yaxis: { min: 0, max: 100, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}%` } },
      tooltip: {
        shared: true,
        x: { formatter: (v: number) => formatChartTime(v) },
        y: { formatter: (v: number) => (v != null ? `${Number(v).toFixed(1)}%` : '—') },
      },
    }
  }

  async function loadGuestSummary(): Promise<void> {
    if (!guest.value) return
    summaryLoading.value = true
    try {
      const bucketMinutes = bucketMinutesFor(hours.value)
      const res = await api.getProxmoxGuestMetrics(guest.value.id, hours.value, bucketMinutes, signal)
      const points = Array.isArray(res.data) ? res.data : []
      if (!points.length) {
        series.value = null
        return
      }
      const palette = getApexChartPalette()
      series.value = [
        {
          name: 'CPU %',
          data: points.map((p: GuestMetricPoint) => ({ x: dayjs(p.timestamp).valueOf(), y: Number(p.cpu_avg ?? 0) })),
          color: palette.cpu,
        },
        {
          name: 'RAM %',
          data: points.map((p: GuestMetricPoint) => ({ x: dayjs(p.timestamp).valueOf(), y: Number(p.memory_avg ?? 0) })),
          color: palette.ram,
        },
      ]
      if (!chartOptions.value) {
        chartOptions.value = buildChartOptions()
      } else {
        const allPoints = series.value.flatMap((s) => s.data)
        chartRef.value?.updateOptions(
          { xaxis: { min: getMinPointTimestamp(allPoints), max: getMaxPointTimestamp(allPoints) } },
          false, false,
        )
      }
    } catch {
      series.value = null
    } finally {
      summaryLoading.value = false
    }
  }

  function changeRange(value: number): void {
    hours.value = value
    loadGuestSummary()
  }

  async function performGuestAction(action: GuestPowerAction): Promise<void> {
    if (!guest.value) return
    await guestActions.performGuestAction(guest.value, action, loadGuest)
  }

  // null = not checked yet (or the check itself failed — fail open, don't
  // block the console button on a fetch error). Only meaningful for lxc
  // guests; PVE console credentials live on the guest's Proxmox connection,
  // not the guest itself, so this needs a separate lookup.
  const consoleConfigured = ref<boolean | null>(null)

  watch(guest, (g) => {
    consoleConfigured.value = null
    if (!g || g.guest_type !== 'lxc') return
    api.getProxmoxInstance(g.connection_id)
      .then((res) => { consoleConfigured.value = res.data.console_configured })
      .catch(() => { consoleConfigured.value = null })
  }, { immediate: true })

  const consoleButtonTitle = computed(() => {
    if (!guest.value || guest.value.guest_type !== 'lxc') return 'VM QEMU : bientôt disponible'
    if (consoleConfigured.value === false) return 'Console PVE non configurée pour cette connexion (Paramètres → Proxmox VE)'
    return 'Ouvrir une console interactive'
  })

  let refreshTimer: ReturnType<typeof setInterval> | undefined
  onMounted(async () => {
    await loadGuest()
    await loadGuestSummary()
    await loadGuestNetworks()
    refreshTimer = setInterval(() => {
      if (autoRefresh.value) loadGuest()
    }, GUEST_REFRESH_SEC * 1000)
  })
  onUnmounted(() => {
    if (refreshTimer) clearInterval(refreshTimer)
  })

  return {
    guest,
    guestLink,
    loading,
    summaryLoading,
    error,
    hours,
    series,
    chartOptions,
    autoRefresh,
    lastUpdatedAt,
    GUEST_REFRESH_SEC,
    changeRange,
    actionLoading,
    performGuestAction,
    guestNetworks,
    guestNetworksLoading,
    nodeId,
    linkSaving,
    linkMsg,
    linkMsgOk,
    confirmLink,
    ignoreLink,
    consoleConfigured,
    consoleButtonTitle,
  }
}

import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import dayjs from '../utils/dayjs'
import api from '../api'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import { useProxmoxGuestActions, type GuestPowerAction } from './useProxmoxGuestActions'
import type { ChartData } from 'chart.js'
import type { ProxmoxGuestLink } from '../types/generated'

export type { GuestPowerAction }

const GUEST_REFRESH_SEC = 30

interface GuestMetricPoint { timestamp: string; cpu_avg?: number; memory_avg?: number; [key: string]: unknown }

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

export function useProxmoxGuest() {
  const route = useRoute()
  const signal = useAbortSignal()
  const guestActions = useProxmoxGuestActions()
  const guest = ref<ProxmoxGuest | null>(null)
  const guestLink = ref<ProxmoxGuestLink | null>(null)
  const loading = ref(true)
  const summaryLoading = ref(false)
  const error = ref('')
  const hours = ref(24)
  const chartData = ref<ChartData<'line'> | null>(null)
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

  function cssVar(name: string): string {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  }

  async function loadGuestSummary(): Promise<void> {
    if (!guest.value) return
    summaryLoading.value = true
    try {
      const bucketMinutes = bucketMinutesFor(hours.value)
      const res = await api.getProxmoxGuestMetrics(guest.value.id, hours.value, bucketMinutes, signal)
      const points = Array.isArray(res.data) ? res.data : []
      if (!points.length) {
        chartData.value = null
        return
      }
      const labels = points.map((p: GuestMetricPoint) =>
        hours.value >= 24 ? dayjs(p.timestamp).format('DD/MM HH:mm') : dayjs(p.timestamp).format('HH:mm')
      )
      chartData.value = {
        labels,
        datasets: [
          {
            label: 'CPU %',
            data: points.map((p: GuestMetricPoint) => Number(p.cpu_avg ?? 0)),
            borderColor: cssVar('--tblr-blue'),
            backgroundColor: `rgba(${cssVar('--tblr-blue-rgb')},0.10)`,
            fill: true,
          },
          {
            label: 'RAM %',
            data: points.map((p: GuestMetricPoint) => Number(p.memory_avg ?? 0)),
            borderColor: cssVar('--tblr-green'),
            backgroundColor: `rgba(${cssVar('--tblr-green-rgb')},0.10)`,
            fill: true,
          },
        ],
      }
    } catch {
      chartData.value = null
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
    chartData,
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
  }
}

import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import dayjs from '../utils/dayjs'
import api from '../api'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { ChartData } from 'chart.js'
import type { ProxmoxGuestLink } from '../types/generated'

interface GuestMetricPoint { timestamp: string; cpu_avg?: number; memory_avg?: number; [key: string]: unknown }

interface ProxmoxGuest {
  id: string
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
  uptime: number
}

export function useProxmoxGuest() {
  const route = useRoute()
  const signal = useAbortSignal()
  const guest = ref<ProxmoxGuest | null>(null)
  const guestLink = ref<ProxmoxGuestLink | null>(null)
  const loading = ref(true)
  const summaryLoading = ref(false)
  const error = ref('')
  const hours = ref(24)
  const chartData = ref<ChartData<'line'> | null>(null)

  function bucketMinutesFor(inputHours: number): number {
    if (inputHours <= 6) return 1
    if (inputHours <= 24) return 5
    if (inputHours <= 168) return 15
    return 60
  }

  async function loadGuest(): Promise<void> {
    loading.value = true
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
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, 'Erreur lors du chargement du guest Proxmox.')
    } finally {
      loading.value = false
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

  onMounted(async () => {
    await loadGuest()
    await loadGuestSummary()
  })

  return {
    guest,
    guestLink,
    loading,
    summaryLoading,
    error,
    hours,
    chartData,
    changeRange,
  }
}

import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import api from '../api'
import { addToast } from './useGlobalToast'
import type { RemoteCommandWithHost } from '../types/audit'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'

const PAGE_SIZE = 50
const POLL_INTERVAL = 10_000

export function useActiveCommands() {
  const signal = useAbortSignal()

  const commands = ref<RemoteCommandWithHost[]>([])
  const total = ref(0)
  const page = ref(1)
  const loading = ref(false)
  const error = ref('')
  const cancellingId = ref<string | null>(null)
  const statusFilter = ref('')
  const moduleFilter = ref('')
  let pollTimer: ReturnType<typeof setInterval> | null = null

  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
  const activeCount = computed(() => commands.value.filter((c) => c.status === 'pending' || c.status === 'running').length)

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await api.getCommandsHistory(page.value, PAGE_SIZE, {
        status: statusFilter.value || undefined,
        module: moduleFilter.value || undefined,
      }, signal)
      commands.value = res.data.commands || []
      total.value = res.data.total || 0
    } catch (err: unknown) {
      if (isApiAbort(err)) return
      error.value = getApiErrorMessage(err, 'Erreur de chargement')
    } finally {
      loading.value = false
    }
  }

  async function cancelCmd(id: string): Promise<void> {
    cancellingId.value = id
    try {
      await api.cancelCommand(id)
      commands.value = commands.value.map((c) =>
        c.id === id ? { ...c, status: 'cancelled' } : c
      )
      addToast('Commande annulée', 'success')
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, 'Impossible d\'annuler'), 'error')
    } finally {
      cancellingId.value = null
    }
  }

  function setPage(p: number): void {
    page.value = p
    load()
  }

  function moduleBadge(module: string): string {
    const map: Record<string, string> = {
      docker: 'bg-blue-lt text-blue',
      apt: 'bg-yellow-lt text-yellow',
      systemd: 'bg-cyan-lt text-cyan',
      journal: 'bg-purple-lt text-purple',
      custom: 'bg-teal-lt text-teal',
    }
    return map[module] || 'bg-secondary-lt text-secondary'
  }

  function statusBadge(status: string): string {
    const map: Record<string, string> = {
      pending: 'bg-secondary-lt text-secondary',
      running: 'bg-blue-lt text-blue',
      completed: 'bg-green-lt text-green',
      failed: 'bg-red-lt text-red',
      cancelled: 'bg-orange-lt text-orange',
    }
    return map[status] || 'bg-secondary-lt text-secondary'
  }

  watch([statusFilter, moduleFilter], () => {
    page.value = 1
    load()
  })

  onMounted(() => {
    load()
    pollTimer = setInterval(load, POLL_INTERVAL)
  })

  onUnmounted(() => {
    if (pollTimer) clearInterval(pollTimer)
  })

  return {
    commands,
    total,
    page,
    loading,
    error,
    cancellingId,
    statusFilter,
    moduleFilter,
    totalPages,
    activeCount,
    load,
    cancelCmd,
    setPage,
    moduleBadge,
    statusBadge,
  }
}

<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>APT</span>
      </div>
      <div class="d-flex align-items-center justify-content-between flex-wrap gap-2">
        <h2 class="page-title">
          APT — Mises à jour système
        </h2>
        <router-link
          to="/audit?module=apt"
          class="btn btn-sm btn-outline-secondary"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="icon icon-sm me-1"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          ><path
            stroke="none"
            d="M0 0h24v24H0z"
            fill="none"
          /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
          Historique des commandes
        </router-link>
      </div>
      <div class="text-secondary">
        Gérer les mises à jour APT sur tous les hôtes
      </div>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      :data-stale-alert="dataStaleAlert"
      @reconnect="reconnect"
      @dismiss-stale-alert="dataStaleAlert = false"
    />

    <AptToolbar
      v-model:search="hostSearch"
      v-model:quick-filter="hostQuickFilter"
      v-model:sort-key="hostSortKey"
      v-model:sort-dir="hostSortDir"
      v-model:all-selected="selectAll"
      :filter-options="hostFilterOptions"
      :can-run-apt="canRunApt"
      :selected-count="selectedHosts.length"
      :bulk-loading="aptBulkLoading"
      @bulk-cmd="bulkAptCmd"
    />

    <div class="side-layout">
      <div class="side-main">
        <div class="row row-cards">
          <template v-if="wsStatus === 'connecting' && hosts.length === 0">
            <div
              v-for="n in 3"
              :key="`sk-${n}`"
              class="col-12"
            >
              <LoadingSkeleton
                variant="card"
                :lines="4"
              />
            </div>
          </template>
          <div
            v-else-if="filteredHosts.length === 0"
            class="col-12"
          >
            <div class="card">
              <div class="card-body text-center text-secondary py-4">
                Aucun hôte ne correspond aux filtres.
              </div>
            </div>
          </div>

          <div
            v-for="host in filteredHosts"
            :key="host.id"
            class="col-12"
          >
            <AptHostCard
              :host="host"
              :apt-status="aptStatuses[host.id]"
              :history="aptHistories[host.id]"
              :expanded="!!hostExpanded[host.id]"
              :selected="selectedHosts.includes(host.id)"
              :can-run-apt="canRunApt"
              :cmd-loading="hostCmdLoading[host.id]"
              @update:selected="val => toggleSelected(host.id, val)"
              @update:expanded="val => hostExpanded[host.id] = val"
              @run-cmd="cmd => runAptCmdForHost(host, cmd)"
              @schedule="openScheduleModal(host)"
              @watch-command="cmd => watchCommand(cmd, host)"
            />
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="liveCommand"
        :show="showConsole"
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="side-panel"
        @open="showConsole = true"
        @close="closeLiveConsole"
      />
    </div>

    <AptScheduleModal
      :host="scheduleHost"
      @close="scheduleHost = null"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted, computed } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from '../composables/useWebSocket'
import type { WSAptSnapshot } from '../types/ws'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { confirmBulkAction } from '../utils/bulkActionHelpers'
import { addToast } from '../composables/useGlobalToast'
import { useCommandStream } from '../composables/useCommandStream'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import AptToolbar from '../components/apt/AptToolbar.vue'
import AptHostCard from '../components/apt/AptHostCard.vue'
import AptScheduleModal from '../components/apt/AptScheduleModal.vue'

// ── État hôtes / APT ─────────────────────────────────────────────────────────
const hosts = ref<any[]>([])
const selectedHosts = ref<string[]>([])
const hostExpanded = ref<Record<string, boolean>>({})
const aptStatuses = ref<Record<string, any>>({})
const aptHistories = ref<Record<string, any[]>>({})
const hostCmdLoading = ref<Record<string, string | null>>({})
const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

const selectAll = computed({
  get() {
    const ids = filteredHosts.value.map((h: any) => h.id)
    return ids.length > 0 && ids.every((id: string) => selectedHosts.value.includes(id))
  },
  set(val: boolean) {
    selectedHosts.value = val ? filteredHosts.value.map((h: any) => h.id) : []
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
const scheduleHost = ref<any | null>(null)

function openScheduleModal(host: any): void {
  scheduleHost.value = host
}

// ── Console ───────────────────────────────────────────────────────────────────
const showConsole = ref(false)
const liveCommand = ref<any>(null)
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
]

const filteredHosts = computed(() => {
  let list = [...hosts.value]

  const q = hostSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter((h: any) => {
      const primary = (h.name || h.hostname || '').toLowerCase()
      const secondary = (h.hostname || '').toLowerCase()
      return primary.includes(q) || secondary.includes(q) || (h.ip_address || '').includes(q)
    })
  }

  if (hostQuickFilter.value === 'critical') {
    list = list.filter((h: any) => {
      const cves = aptStatuses.value[h.id]?.cve_list
      return Array.isArray(cves) && cves.some((c: any) => c.severity === 'CRITICAL')
    })
  } else if (hostQuickFilter.value === 'security') {
    list = list.filter((h: any) => (aptStatuses.value[h.id]?.security_updates || 0) > 0)
  }

  list.sort((a: any, b: any) => {
    let va: any, vb: any
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
function watchCommand(cmd: any, host: any): void {
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

function upsertAptHistory(hostId: string, nextCommand: any): void {
  if (!hostId || !nextCommand?.id) return
  const currentHistory = Array.isArray(aptHistories.value[hostId]) ? [...aptHistories.value[hostId]] : []
  const currentIndex = currentHistory.findIndex((cmd: any) => cmd.id === nextCommand.id)
  if (currentIndex >= 0) {
    currentHistory[currentIndex] = { ...currentHistory[currentIndex], ...nextCommand }
  } else {
    currentHistory.unshift(nextCommand)
  }
  currentHistory.sort((left: any, right: any) => new Date(right.created_at || 0).getTime() - new Date(left.created_at || 0).getTime())
  aptHistories.value = { ...aptHistories.value, [hostId]: currentHistory }
}

function syncLiveCommand(commandId: string, patch: any): void {
  if (!liveCommand.value || liveCommand.value.id !== commandId) return
  liveCommand.value = { ...liveCommand.value, ...patch }
}

function syncAptHistoryCommand(commandId: string, patch: any): void {
  const hostId = liveCommand.value?.id === commandId ? liveCommand.value.hostId : null
  if (!hostId) return
  upsertAptHistory(hostId, {
    id: commandId,
    action: liveCommand.value?.action || patch.action,
    output: liveCommand.value?.output || '',
    ...patch,
  })
}

function connectStreamWebSocket(commandId: string): void {
  closeStream()
  openCommandStream(commandId, {
    closeOnTerminalStatus: true,
    onInit: (payload: any) => {
      syncLiveCommand(commandId, { status: payload.status, output: payload.output || '' })
      syncAptHistoryCommand(commandId, { status: payload.status })
    },
    onChunk: (payload: any) => {
      const nextOutput = `${liveCommand.value?.output || ''}${payload.chunk || ''}`
      syncLiveCommand(commandId, { output: nextOutput })
    },
    onStatus: (payload: any) => {
      const patch: any = { status: payload.status }
      if (typeof payload.output === 'string') patch.output = payload.output
      syncLiveCommand(commandId, patch)
      syncAptHistoryCommand(commandId, patch)
    },
  })
}


// ── Commandes par hôte ────────────────────────────────────────────────────────
async function runAptCmdForHost(host: any, command: string): Promise<void> {
  if (!canRunApt.value) return

  const confirmed = await confirmBulkAction(
    `apt ${command}`,
    1,
    command === 'dist-upgrade'
      ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${host.name || host.hostname} ?`
      : `Exécuter sur : ${host.name || host.hostname} ?`
  )
  if (!confirmed) return

  hostCmdLoading.value = { ...hostCmdLoading.value, [host.id]: command }
  try {
    const response = await apiClient.sendAptCommand([host.id], command)
    const commandResults: any[] = Array.isArray(response.data?.commands) ? response.data.commands : []
    const launched = commandResults.filter((item: any) => item.command_id)
    const failed = commandResults.filter((item: any) => item.error)
    const createdAt = new Date().toISOString()

    launched.forEach((item: any) => {
      upsertAptHistory(host.id, {
        id: item.command_id,
        action: command,
        status: item.status || 'pending',
        output: '',
        created_at: createdAt,
        triggered_by: auth.username || '',
      })
    })

    if (launched.length > 0) {
      watchCommand(
        { id: launched[0].command_id, action: command, status: launched[0].status || 'pending', output: '' },
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
    .filter((h: any) => selectedHosts.value.includes(h.id))
    .map((h: any) => h.name || h.hostname)
    .join(', ')

  const confirmed = await confirmBulkAction(
    `apt ${command}`,
    selectedHosts.value.length,
    command === 'dist-upgrade'
      ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${hostnames || 'les hôtes sélectionnés'} ?`
      : `Exécuter sur : ${hostnames || 'les hôtes sélectionnés'} ?`
  )
  if (!confirmed) return

  aptBulkLoading.value = command
  try {
    const response = await apiClient.sendAptCommand(selectedHosts.value, command)
    const commandResults: any[] = Array.isArray(response.data?.commands) ? response.data.commands : []
    const hostNameById = new Map(hosts.value.map((host: any) => [host.id, host.name || host.hostname || host.id]))
    const launchedCommands = commandResults.filter((item: any) => item.command_id)
    const failedCommands = commandResults.filter((item: any) => item.error)
    const createdAt = new Date().toISOString()

    launchedCommands.forEach((item: any) => {
      upsertAptHistory(item.host_id, {
        id: item.command_id,
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
      const host = hosts.value.find((h: any) => h.id === launchedCommand.host_id)
      if (host) {
        watchCommand({ id: launchedCommand.command_id, action: command, status: launchedCommand.status || 'pending', output: '' }, host)
      }
    }

    if (selectedHosts.value.length > 1 || failedCommands.length > 0) {
      const launched = launchedCommands.map((item: any) => hostNameById.get(item.host_id) || item.host_id)
      const failed = failedCommands.map((item: any) => hostNameById.get(item.host_id) || item.host_id)
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

const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket<WSAptSnapshot>('/api/v1/ws/apt', (payload) => {
  if (payload.type !== 'apt') return
  hosts.value = payload.hosts || []
  aptStatuses.value = payload.apt_statuses || {}
  aptHistories.value = payload.apt_histories || {}
}, { debounceMs: 750 })

onUnmounted(() => {
  closeStream()
})
</script>


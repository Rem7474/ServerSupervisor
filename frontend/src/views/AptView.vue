<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">APT - Mises a jour systeme</h2>
      <div class="text-secondary">Gerer les mises a jour APT sur tous les hotes</div>
    </div>

    <div v-if="liveCommand" class="card mb-4">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title">Console Live - {{ liveCommand.hostname }}</h3>
        <button @click="closeLiveConsole" class="btn btn-sm btn-outline-secondary">Fermer</button>
      </div>
      <div class="card-body">
        <div class="text-secondary small mb-2">
          Commande: <code>apt {{ liveCommand.command }}</code>
          <span class="mx-2">•</span>
          Statut: <span :class="statusClass(liveCommand.status)">{{ liveCommand.status }}</span>
        </div>
        <pre
          ref="consoleOutput"
          class="bg-dark text-light p-3 rounded mb-0"
          style="max-height: 20rem; overflow-y: auto; font-size: 0.85rem; line-height: 1.5;"
        >{{ liveCommand.output }}</pre>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-body">
        <div class="d-flex flex-wrap align-items-center gap-3">
          <label class="form-check">
            <input type="checkbox" class="form-check-input" v-model="selectAll" @change="toggleSelectAll" />
            <span class="form-check-label">Selectionner tous les hotes</span>
          </label>
          <div class="ms-auto d-flex flex-wrap gap-2">
            <template v-if="canRunApt">
              <button @click="bulkAptCmd('update')" class="btn btn-outline-secondary" :disabled="selectedHosts.length === 0">
                apt update ({{ selectedHosts.length }})
              </button>
              <button @click="bulkAptCmd('upgrade')" class="btn btn-primary" :disabled="selectedHosts.length === 0">
                apt upgrade ({{ selectedHosts.length }})
              </button>
              <button @click="bulkAptCmd('dist-upgrade')" class="btn btn-outline-danger" :disabled="selectedHosts.length === 0">
                apt dist-upgrade ({{ selectedHosts.length }})
              </button>
            </template>
            <div v-else class="text-secondary small">Mode lecture seule</div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards">
      <div v-for="host in hosts" :key="host.id" class="col-12">
        <div class="card">
          <div class="card-body">
            <div class="d-flex align-items-center gap-3 mb-3">
              <label class="form-check">
                <input type="checkbox" class="form-check-input" :value="host.id" v-model="selectedHosts" />
                <span class="form-check-label"></span>
              </label>
              <div class="flex-fill">
                <div class="fw-semibold">{{ host.hostname || host.name }}</div>
                <div class="text-secondary small">{{ host.ip_address }}</div>
              </div>
              <span :class="host.status === 'online' ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
                {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
              </span>
            </div>

            <div v-if="aptStatuses[host.id]" class="row row-cards mb-3">
              <div class="col-sm-6 col-md-3">
                <div class="card card-sm">
                  <div class="card-body text-center">
                    <div class="h2 mb-0" :class="aptStatuses[host.id].pending_packages > 0 ? 'text-yellow' : 'text-green'">
                      {{ aptStatuses[host.id].pending_packages }}
                    </div>
                    <div class="text-secondary small">En attente</div>
                  </div>
                </div>
              </div>
              <div class="col-sm-6 col-md-3">
                <div class="card card-sm">
                  <div class="card-body text-center">
                    <div class="h2 mb-0 text-red">{{ aptStatuses[host.id].security_updates }}</div>
                    <div class="text-secondary small">Securite</div>
                  </div>
                </div>
              </div>
              <div class="col-sm-6 col-md-3">
                <div class="card card-sm">
                  <div class="card-body text-center">
                    <div class="fw-semibold">{{ formatDate(aptStatuses[host.id].last_update) }}</div>
                    <div class="text-secondary small">Dernier update</div>
                  </div>
                </div>
              </div>
              <div class="col-sm-6 col-md-3">
                <div class="card card-sm">
                  <div class="card-body text-center">
                    <div class="fw-semibold">{{ formatDate(aptStatuses[host.id].last_upgrade) }}</div>
                    <div class="text-secondary small">Dernier upgrade</div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="aptHistories[host.id]?.length">
              <button @click="toggleHistory(host.id)" class="btn btn-link p-0">
                {{ expandedHistories[host.id] ? 'Masquer' : 'Voir' }} l'historique ({{ aptHistories[host.id].length }})
              </button>
              <div v-if="expandedHistories[host.id]" class="mt-3">
                <div v-for="cmd in aptHistories[host.id]" :key="cmd.id" class="border rounded p-3 mb-2">
                  <div class="d-flex align-items-center justify-content-between">
                    <div class="fw-semibold">apt {{ cmd.command }}</div>
                    <div class="d-flex align-items-center gap-2">
                      <span :class="cmd.status === 'completed' ? 'badge bg-green-lt text-green' : cmd.status === 'failed' ? 'badge bg-red-lt text-red' : 'badge bg-yellow-lt text-yellow'">
                        {{ cmd.status }}
                      </span>
                      <button
                        v-if="cmd.status === 'running' || cmd.status === 'pending'"
                        @click="watchCommand(cmd, host)"
                        class="btn btn-sm btn-outline-primary"
                      >
                        Voir les logs
                      </button>
                    </div>
                  </div>
                  <div class="text-secondary small mt-1">
                    {{ formatDate(cmd.created_at) }}
                    <span v-if="cmd.triggered_by">• par {{ cmd.triggered_by }}</span>
                  </div>
                  <pre v-if="cmd.output" class="bg-dark text-light p-2 rounded mt-2" style="max-height: 8rem; overflow-y: auto;">{{ cmd.output }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

const hosts = ref([])
const selectedHosts = ref([])
const selectAll = ref(false)
const aptStatuses = ref({})
const aptHistories = ref({})
const expandedHistories = ref({})
const auth = useAuthStore()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')
const liveCommand = ref(null)
const consoleOutput = ref(null)
let ws = null
let streamWs = null

function toggleSelectAll() {
  if (selectAll.value) {
    selectedHosts.value = hosts.value.map(h => h.id)
  } else {
    selectedHosts.value = []
  }
}

function toggleHistory(hostId) {
  expandedHistories.value[hostId] = !expandedHistories.value[hostId]
}

function watchCommand(cmd, host) {
  liveCommand.value = {
    id: cmd.id,
    command: cmd.command,
    status: cmd.status,
    hostname: host.hostname || host.name,
    output: cmd.output || '',
  }
  connectStreamWebSocket(cmd.id)
  nextTick(() => scrollToBottom())
}

function closeLiveConsole() {
  if (streamWs) {
    streamWs.close()
    streamWs = null
  }
  liveCommand.value = null
}

function scrollToBottom() {
  if (consoleOutput.value) {
    consoleOutput.value.scrollTop = consoleOutput.value.scrollHeight
  }
}

function connectStreamWebSocket(commandId) {
  if (streamWs) {
    streamWs.close()
  }
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/apt/stream/${commandId}?token=${auth.token}`
  streamWs = new WebSocket(wsUrl)

  streamWs.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload.type === 'apt_stream_init') {
        liveCommand.value.status = payload.status
        liveCommand.value.output = payload.output || ''
        nextTick(() => scrollToBottom())
      } else if (payload.type === 'apt_stream') {
        liveCommand.value.output += payload.chunk
        nextTick(() => scrollToBottom())
      }
    } catch (e) {
      // Ignore malformed payloads
    }
  }

  streamWs.onclose = () => {
    // Connection closed, don't reconnect automatically
  }
}

async function bulkAptCmd(command) {
  const hostnames = hosts.value.filter(h => selectedHosts.value.includes(h.id)).map(h => h.hostname).join(', ')
  if (!confirm(`Exécuter 'apt ${command}' sur: ${hostnames} ?`)) return
  try {
    await apiClient.sendAptCommand(selectedHosts.value, command)
    alert(`Commande envoyée à ${selectedHosts.value.length} hôte(s)`)
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

function connectWebSocket() {
  if (!auth.token) return
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/apt?token=${auth.token}`
  ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload.type !== 'apt') return
      hosts.value = payload.hosts || []
      aptStatuses.value = payload.apt_statuses || {}
      aptHistories.value = payload.apt_histories || {}
    } catch (e) {
      // Ignore malformed payloads
    }
  }

  ws.onclose = () => {
    setTimeout(connectWebSocket, 2000)
  }
}

onMounted(() => {
  connectWebSocket()
})

onUnmounted(() => {
  if (ws) ws.close()
  if (streamWs) streamWs.close()
})
</script>

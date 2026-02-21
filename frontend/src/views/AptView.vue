<template>
  <div class="d-flex flex-column" style="height: calc(100vh - 120px);">
    <div class="page-header mb-3">
      <h2 class="page-title">APT - Mises a jour systeme</h2>
      <div class="text-secondary">Gerer les mises a jour APT sur tous les hotes</div>
    </div>

    <div class="d-flex flex-fill" style="gap: 1rem; overflow: hidden; min-height: 0;">
      <!-- Colonne gauche: Liste des hôtes -->
      <div style="flex: 1; overflow-y: auto; min-width: 0;">
        <div class="card mb-3">
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

            <!-- CVE Information -->
            <div v-if="aptStatuses[host.id]?.cve_list" class="mb-3">
              <CVEList 
                :cveList="aptStatuses[host.id].cve_list" 
                :showMaxSeverity="true"
                :alwaysExpanded="false"
                :limit="5"
              />
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
                      <span :class="statusClass(cmd.status)">
                        {{ cmd.status }}
                      </span>
                      <button
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
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
      </div>

      <!-- Colonne droite: Console Live -->
      <div style="width: 38%; min-width: 450px; display: flex; flex-direction: column;">
        <div class="card" style="display: flex; flex-direction: column; height: 100%;">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M8 9l3 3l-3 3" />
                <path d="M13 15l3 0" />
                <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
              </svg>
              Console Live
            </h3>
            <button 
              v-if="liveCommand" 
              @click="closeLiveConsole" 
              class="btn btn-sm btn-ghost-secondary"
              title="Fermer la console"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M18 6l-12 12" />
                <path d="M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="card-body d-flex flex-column" style="flex: 1; min-height: 0; padding: 0;">
            <!-- État vide -->
            <div v-if="!liveCommand" class="d-flex align-items-center justify-content-center flex-fill text-secondary" style="background: #1e293b; border-radius: 0 0 0.5rem 0.5rem;">
              <div class="text-center p-4">
                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler mb-2" width="48" height="48" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.5;">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M8 9l3 3l-3 3" />
                  <path d="M13 15l3 0" />
                  <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
                </svg>
                <div style="opacity: 0.7;">Aucune console active</div>
                <div class="small mt-1" style="opacity: 0.5;">Cliquez sur "Voir les logs" pour afficher la sortie d'une commande</div>
              </div>
            </div>

            <!-- Console active -->
            <div v-else style="display: flex; flex-direction: column; height: 100%;">
              <div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
                <div class="d-flex align-items-start justify-content-between mb-2">
                  <div class="flex-fill" style="min-width: 0;">
                    <div class="fw-semibold text-light" style="font-size: 0.95rem;">{{ liveCommand.hostname }}</div>
                    <div class="text-secondary small mt-1">
                      <code style="background: rgba(0,0,0,0.3); padding: 0.15rem 0.4rem; border-radius: 0.25rem; color: #94a3b8;">apt {{ liveCommand.command }}</code>
                    </div>
                  </div>
                  <span :class="statusClass(liveCommand.status)" style="margin-left: 0.5rem;">{{ liveCommand.status }}</span>
                </div>
              </div>
              <pre
                ref="consoleOutput"
                class="mb-0 flex-fill"
                style="
                  background: #0f172a;
                  color: #e2e8f0;
                  padding: 1rem;
                  margin: 0;
                  overflow-y: auto;
                  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
                  font-size: 0.813rem;
                  line-height: 1.5;
                  border-radius: 0 0 0.5rem 0.5rem;
                "
              >{{ renderedConsoleOutput || 'En attente de sortie...' }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import CVEList from '../components/CVEList.vue'
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

const renderedConsoleOutput = computed(() => {
  if (!liveCommand.value) return ''
  const raw = liveCommand.value.output || ''
  return renderConsoleOutput(raw)
})
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
      } else if (payload.type === 'apt_status_update') {
        liveCommand.value.status = payload.status
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
    const response = await apiClient.sendAptCommand(selectedHosts.value, command)
    alert(`Commande envoyée à ${selectedHosts.value.length} hôte(s)`)
    
    // Auto-open console if only 1 host selected
    if (selectedHosts.value.length === 1 && response.data?.commands?.length > 0) {
      const cmd = response.data.commands[0]
      const host = hosts.value.find(h => h.id === selectedHosts.value[0])
      if (cmd.command_id && host) {
        watchCommand({
          id: cmd.command_id,
          command: command,
          status: 'pending',
          output: ''
        }, host)
      }
    }
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

function renderConsoleOutput(raw) {
  if (!raw) return ''
  const lines = ['']
  let currentLine = ''

  for (let i = 0; i < raw.length; i++) {
    const ch = raw[i]
    if (ch === '\r') {
      currentLine = ''
      lines[lines.length - 1] = ''
      continue
    }
    if (ch === '\n') {
      currentLine = ''
      lines.push('')
      continue
    }
    currentLine += ch
    lines[lines.length - 1] = currentLine
  }

  return lines.join('\n')
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

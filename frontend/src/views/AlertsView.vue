<template>
  <div class="page-wrapper">
    <div class="page-header d-print-none">
      <div class="container-xl">
        <div class="row g-2 align-items-center">
          <div class="col">
            <div class="page-pretitle">
              <router-link to="/" class="text-decoration-none">Dashboard</router-link>
              <span class="text-muted mx-1">/</span>
              <span>Alertes</span>
            </div>
            <h2 class="page-title">Alertes</h2>
          </div>
          <div class="col-auto ms-auto d-flex gap-2">
            <button v-if="alertsTab === 'rules'" @click="startAddAlert" class="btn btn-primary">
              <svg class="icon me-1" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
              </svg>
              Nouvelle alerte
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="page-body">
      <div class="container-xl">
        <ul class="nav nav-tabs mb-4">
          <li class="nav-item">
            <a class="nav-link" :class="{ active: alertsTab === 'rules' }" href="#" @click.prevent="alertsTab = 'rules'">
              Regles
              <span class="badge bg-azure-lt text-azure ms-1">{{ rules.length }}</span>
            </a>
          </li>
          <li class="nav-item">
            <a class="nav-link" :class="{ active: alertsTab === 'incidents' }" href="#" @click.prevent="switchToIncidents">
              Incidents
              <span v-if="activeIncidentCount > 0" class="badge bg-red-lt text-red ms-1">{{ activeIncidentCount }}</span>
            </a>
          </li>
        </ul>

        <div v-show="alertsTab === 'rules'">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title">Regles actives</h3>
            </div>
            <div v-if="loading" class="card-body text-center py-5">
              <div class="spinner-border text-primary" role="status"></div>
              <div class="mt-2">Chargement...</div>
            </div>
            <div v-else-if="rules.length === 0" class="card-body text-center py-5 text-muted">
              <svg class="icon icon-lg mb-3 text-muted" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
              </svg>
              <div>Aucune regle d'alerte configuree</div>
              <button @click="startAddAlert" class="btn btn-primary mt-3">Creer ma premiere alerte</button>
            </div>
            <div v-else class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th>Etat</th>
                    <th>Nom</th>
                    <th>Hote</th>
                    <th>Metrique</th>
                    <th>Condition</th>
                    <th>Duree</th>
                    <th>Canaux</th>
                    <th class="w-1">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="rule in rules" :key="rule.id">
                    <td>
                      <label class="form-check form-switch m-0">
                        <input class="form-check-input" type="checkbox" :checked="rule.enabled" @change="toggleEnabled(rule)" />
                      </label>
                    </td>
                    <td>
                      <div class="fw-bold">{{ rule.name || 'Sans nom' }}</div>
                      <div v-if="rule.last_fired" class="text-muted small">Derniere alerte: {{ formatDate(rule.last_fired) }}</div>
                    </td>
                    <td>
                      <span v-if="rule.host_id" class="badge bg-secondary-lt text-secondary">{{ getHostName(rule.host_id) }}</span>
                      <span v-else class="badge bg-info-lt text-info">Tous les hotes</span>
                    </td>
                    <td>
                      <span class="badge" :class="getMetricBadgeClass(rule.metric)">{{ getMetricLabel(rule.metric) }}</span>
                    </td>
                    <td><code>{{ rule.operator }} {{ rule.threshold }}{{ getMetricUnit(rule.metric) }}</code></td>
                    <td>{{ formatDurationSecs(rule.duration_seconds) }}</td>
                    <td>
                      <span v-for="channel in rule.actions?.channels" :key="channel" class="badge me-1" :class="channel === 'browser' ? 'bg-green-lt text-green' : 'bg-azure-lt text-azure'">
                        {{ channel === 'browser' ? 'Navigateur' : channel }}
                      </span>
                      <span v-if="rule.actions?.command_trigger" class="badge bg-orange-lt text-orange me-1" :title="`${rule.actions.command_trigger.module}/${rule.actions.command_trigger.action}${rule.actions.command_trigger.target ? ' -> ' + rule.actions.command_trigger.target : ''}`">
                        cmd
                      </span>
                    </td>
                    <td>
                      <div class="btn-group">
                        <button @click="startEditAlert(rule)" class="btn btn-sm btn-ghost-secondary" title="Modifier">
                          <svg class="icon" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
                          </svg>
                        </button>
                        <button @click="deleteAlert(rule)" class="btn btn-sm btn-ghost-danger" title="Supprimer">
                          <svg class="icon" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                          </svg>
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div v-show="alertsTab === 'incidents'">
          <AlertIncidentList
            :incidents="incidents"
            :loading="incidentsLoading"
            :error="incidentsError"
            :active-incident-count="activeIncidentCount"
            @refresh="loadIncidents"
          />
        </div>
      </div>
    </div>

    <AlertRuleModal
      :visible="showModal"
      :rule="editingRule"
      :hosts="hosts"
      :saving="saving"
      :error="saveError"
      @close="closeModal"
      @submit="saveAlert"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AlertIncidentList from '../components/alerts/AlertIncidentList.vue'
import AlertRuleModal from '../components/alerts/AlertRuleModal.vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useDateFormatter } from '../composables/useDateFormatter'
import { useWebSocket } from '../composables/useWebSocket'
import apiClient from '../api'
import { formatDurationSecs } from '../utils/formatters'

const { confirm } = useConfirmDialog()
const route = useRoute()

const alertsTab = ref('rules')
const incidents = ref([])
const incidentsLoading = ref(false)
const incidentsError = ref('')
const incidentsLoaded = ref(false)
const rules = ref([])
const hosts = ref([])
const loading = ref(true)
const showModal = ref(false)
const saving = ref(false)
const saveError = ref('')
const editingRule = ref(null)
const { formatLocaleDateTime } = useDateFormatter()
let incidentsPollTimer = null

const activeIncidentCount = computed(() => incidents.value.filter((incident) => !incident.resolved_at).length)

onMounted(async () => {
  if (route.query.tab === 'incidents') {
    await switchToIncidents()
  }
  await loadRules()
  await loadHosts()

  // Fallback safety net in case WS notifications are missed.
  incidentsPollTimer = setInterval(loadIncidents, 30_000)
})

onUnmounted(() => {
  if (incidentsPollTimer) {
    clearInterval(incidentsPollTimer)
    incidentsPollTimer = null
  }
})

async function loadIncidents() {
  incidentsLoading.value = true
  incidentsError.value = ''
  try {
    const response = await apiClient.getNotifications()
    incidents.value = response.data?.notifications || []
    incidentsLoaded.value = true
  } catch {
    incidentsError.value = 'Impossible de charger les incidents'
  } finally {
    incidentsLoading.value = false
  }
}

async function switchToIncidents() {
  alertsTab.value = 'incidents'
  if (!incidentsLoaded.value) await loadIncidents()
}

async function loadRules() {
  try {
    loading.value = true
    const response = await apiClient.getAlertRules()
    rules.value = response.data || []
  } finally {
    loading.value = false
  }
}

async function loadHosts() {
  try {
    const response = await apiClient.getHosts()
    hosts.value = response.data || []
  } catch {
    hosts.value = []
  }
}

function startAddAlert() {
  editingRule.value = null
  saveError.value = ''
  showModal.value = true
}

function startEditAlert(rule) {
  editingRule.value = rule
  saveError.value = ''
  showModal.value = true
}

async function saveAlert(payload) {
  saveError.value = ''
  saving.value = true
  try {
    if (editingRule.value) {
      await apiClient.updateAlertRule(editingRule.value.id, payload)
    } else {
      await apiClient.createAlertRule(payload)
    }
    await loadRules()
    closeModal()
  } catch (err) {
    saveError.value = 'Erreur : ' + (err.response?.data?.error || err.message)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(rule) {
  try {
    await apiClient.updateAlertRule(rule.id, { enabled: !rule.enabled })
    await loadRules()
  } catch {
    // ignore
  }
}

async function deleteAlert(rule) {
  const confirmed = await confirm({
    title: 'Supprimer l\'alerte ?',
    message: `Voulez-vous vraiment supprimer la regle "${rule.name || 'Sans nom'}" ?\n\nCette action est irreversible.`,
    variant: 'danger',
  })
  if (!confirmed) return

  try {
    await apiClient.deleteAlertRule(rule.id)
    await loadRules()
  } catch (err) {
    saveError.value = 'Erreur lors de la suppression : ' + (err.response?.data?.error || err.message)
  }
}

function closeModal() {
  showModal.value = false
  editingRule.value = null
  saveError.value = ''
}

function getHostName(hostId) {
  return hosts.value.find((host) => host.id === hostId)?.name || hostId
}

function getMetricLabel(metric) {
  const labels = {
    cpu: 'CPU',
    memory: 'Memoire',
    disk: 'Disque',
    load: 'Load',
    heartbeat_timeout: 'Heartbeat',
  }
  return labels[metric] || metric
}

function getMetricBadgeClass(metric) {
  const classes = {
    cpu: 'bg-red-lt',
    memory: 'bg-blue-lt',
    disk: 'bg-yellow-lt',
    load: 'bg-purple-lt',
    heartbeat_timeout: 'bg-orange-lt',
  }
  return classes[metric] || 'bg-secondary-lt'
}

function getMetricUnit(metric) {
  const units = {
    cpu: '%',
    memory: '%',
    disk: '%',
    load: '',
    heartbeat_timeout: 's',
  }
  return units[metric] || ''
}

function formatDate(dateStr) {
  return formatLocaleDateTime(dateStr)
}

useWebSocket('/api/v1/ws/notifications', (payload) => {
  if (payload.type !== 'new_alert' || !payload.notification) return

  const incoming = payload.notification
  const idx = incidents.value.findIndex((item) => item.id === incoming.id)

  if (idx >= 0) {
    incidents.value = [
      { ...incidents.value[idx], ...incoming },
      ...incidents.value.slice(0, idx),
      ...incidents.value.slice(idx + 1),
    ]
  } else {
    incidents.value = [incoming, ...incidents.value]
  }

  incidentsLoaded.value = true

  // Refresh from API to capture resolution transitions and normalized server shape.
  loadIncidents()
})
</script>



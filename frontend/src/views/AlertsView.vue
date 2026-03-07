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
            <button v-if="alertsTab === 'incidents'" @click="loadIncidents" class="btn btn-ghost-secondary">
              <svg class="icon" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
              </svg>
              Actualiser
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="page-body">
      <div class="container-xl">
        <!-- Tabs -->
        <ul class="nav nav-tabs mb-4">
          <li class="nav-item">
            <a class="nav-link" :class="{ active: alertsTab === 'rules' }" href="#" @click.prevent="alertsTab = 'rules'">
              Règles
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

        <!-- Rules Tab -->
        <div v-show="alertsTab === 'rules'">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Règles actives</h3>
          </div>
          <div v-if="loading" class="card-body text-center py-5">
            <div class="spinner-border text-primary" role="status"></div>
            <div class="mt-2">Chargement...</div>
          </div>
          <div v-else-if="rules.length === 0" class="card-body text-center py-5 text-muted">
            <svg class="icon icon-lg mb-3 text-muted" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
            </svg>
            <div>Aucune règle d'alerte configurée</div>
            <button @click="startAddAlert" class="btn btn-primary mt-3">Créer ma première alerte</button>
          </div>
          <div v-else class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>État</th>
                  <th>Nom</th>
                  <th>Hôte</th>
                  <th>Métrique</th>
                  <th>Condition</th>
                  <th>Durée</th>
                  <th>Canaux</th>
                  <th class="w-1">Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="rule in rules" :key="rule.id">
                  <td>
                    <label class="form-check form-switch m-0">
                      <input 
                        class="form-check-input" 
                        type="checkbox" 
                        :checked="rule.enabled"
                        @change="toggleEnabled(rule)"
                      />
                    </label>
                  </td>
                  <td>
                    <div class="fw-bold">{{ rule.name || 'Sans nom' }}</div>
                    <div v-if="rule.last_fired" class="text-muted small">
                      Dernière alerte: {{ formatDate(rule.last_fired) }}
                    </div>
                  </td>
                  <td>
                    <span v-if="rule.host_id" class="badge bg-secondary-lt text-secondary">{{ getHostName(rule.host_id) }}</span>
                    <span v-else class="badge bg-info-lt text-info">Tous les hôtes</span>
                  </td>
                  <td>
                    <span class="badge" :class="getMetricBadgeClass(rule.metric)">
                      {{ getMetricLabel(rule.metric) }}
                    </span>
                  </td>
                  <td>
                    <code>{{ rule.operator }} {{ rule.threshold }}{{ getMetricUnit(rule.metric) }}</code>
                  </td>
                  <td>{{ formatDurationSecs(rule.duration_seconds) }}</td>
                  <td>
                    <span v-for="channel in rule.actions?.channels" :key="channel" class="badge me-1"
                      :class="channel === 'browser' ? 'bg-green-lt text-green' : 'bg-azure-lt text-azure'">
                      {{ channel === 'browser' ? 'Navigateur' : channel }}
                    </span>
                    <span v-if="rule.actions?.command_trigger" class="badge bg-orange-lt text-orange me-1" :title="`${rule.actions.command_trigger.module}/${rule.actions.command_trigger.action}${rule.actions.command_trigger.target ? ' → ' + rule.actions.command_trigger.target : ''}`">
                      ⚡ cmd
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
        </div> <!-- end rules tab -->

        <!-- Incidents Tab -->
        <div v-show="alertsTab === 'incidents'">
          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between">
              <h3 class="card-title">Incidents récents</h3>
              <div class="d-flex align-items-center gap-2">
                <span v-if="activeIncidentCount > 0" class="badge bg-red-lt text-red">{{ activeIncidentCount }} actif{{ activeIncidentCount > 1 ? 's' : '' }}</span>
                <span class="text-secondary small">{{ incidents.length }} incident{{ incidents.length !== 1 ? 's' : '' }}</span>
              </div>
            </div>
            <div v-if="incidentsLoading" class="card-body text-center py-5">
              <div class="spinner-border text-primary" role="status"></div>
              <div class="mt-2 text-muted">Chargement...</div>
            </div>
            <div v-else-if="incidentsError" class="card-body text-center py-5 text-danger">{{ incidentsError }}</div>
            <div v-else-if="incidents.length === 0" class="card-body text-center py-5 text-muted">
              <svg class="icon icon-lg mb-3" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
              </svg>
              <div>Aucun incident enregistré</div>
              <div class="text-muted small mt-1">Les incidents apparaîtront ici lorsqu'une règle d'alerte se déclenchera</div>
            </div>
            <div v-else class="table-responsive">
              <table class="table table-vcenter card-table">
                <thead>
                  <tr>
                    <th style="width: 90px;">État</th>
                    <th>Règle</th>
                    <th>Hôte</th>
                    <th>Valeur</th>
                    <th>Déclenché</th>
                    <th>Résolu</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="inc in incidents" :key="inc.id">
                    <td>
                      <span v-if="inc.resolved_at" class="badge bg-green-lt text-green">Résolu</span>
                      <span v-else class="badge bg-red-lt text-red">Actif</span>
                    </td>
                    <td>
                      <div class="fw-semibold text-truncate" style="max-width: 220px;" :title="inc.rule_name">{{ inc.rule_name }}</div>
                      <div class="text-muted small">{{ incidentMetricLabel(inc.metric) }}</div>
                    </td>
                    <td>
                      <router-link :to="`/hosts/${inc.host_id}`" class="text-decoration-none">{{ inc.host_name }}</router-link>
                    </td>
                    <td><code>{{ incidentFormatValue(inc.value, inc.metric) }}</code></td>
                    <td class="text-muted small">{{ formatDate(inc.triggered_at) }}</td>
                    <td class="text-muted small">
                      <span v-if="inc.resolved_at">{{ formatDate(inc.resolved_at) }}</span>
                      <span v-else class="text-secondary">—</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal for Add/Edit Alert -->
    <div v-if="showModal" class="modal modal-blur fade show" style="display: block" tabindex="-1">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingRule ? 'Modifier l\'alerte' : 'Nouvelle alerte' }}</h5>
            <button @click="closeModal" type="button" class="btn-close"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label required">Nom</label>
              <input 
                v-model="form.name" 
                type="text" 
                class="form-control" 
                placeholder="Ex: CPU élevé sur serveur web"
              />
            </div>

            <div class="mb-3">
              <label class="form-label">Hôte cible</label>
              <select v-model="form.host_id" class="form-select">
                <option :value="null">Tous les hôtes</option>
                <option v-for="host in hosts" :key="host.id" :value="host.id">
                  {{ host.name }}
                </option>
              </select>
            </div>

            <div class="row">
              <div class="col-md-4 mb-3">
                <label class="form-label required">Métrique</label>
                <select v-model="form.metric" class="form-select">
                  <option value="cpu">CPU (%)</option>
                  <option value="memory">Mémoire (%)</option>
                  <option value="disk">Disque (%)</option>
                  <option value="load">Load Average</option>
                </select>
              </div>
              <div class="col-md-4 mb-3">
                <label class="form-label required">Opérateur</label>
                <select v-model="form.operator" class="form-select">
                  <option value=">">Supérieur à (>)</option>
                  <option value=">=">Supérieur ou égal (≥)</option>
                  <option value="<">Inférieur à (<)</option>
                  <option value="<=">Inférieur ou égal (≤)</option>
                </select>
              </div>
              <div class="col-md-4 mb-3">
                <label class="form-label required">Seuil</label>
                <input 
                  v-model.number="form.threshold" 
                  type="number" 
                  step="0.1" 
                  class="form-control"
                  placeholder="80"
                />
              </div>
            </div>

            <div class="mb-3">
              <label class="form-label">Durée (secondes)</label>
              <input 
                v-model.number="form.duration" 
                type="number" 
                class="form-control"
                placeholder="300"
              />
              <small class="form-hint">Le seuil doit être dépassé pendant cette durée avant de déclencher l'alerte. Mettre 0 pour déclencher immédiatement.</small>
              <small v-if="form.duration > 0" class="form-hint text-warning d-block mt-1">
                ⚠ Définit aussi l'âge maximum accepté pour les métriques. Si l'agent reporte toutes les 60s, une durée &lt; 60 empêchera l'alerte de se déclencher.
              </small>
            </div>

            <div class="mb-3">
              <label class="form-label">Période de silence (secondes)</label>
              <input
                v-model.number="form.actions.cooldown"
                type="number"
                class="form-control"
                placeholder="3600"
              />
              <small class="form-hint">Temps minimum entre deux alertes successives pour cette règle</small>
            </div>

            <div class="mb-3">
              <label class="form-label">Canaux de notification</label>
              <div>
                <label class="form-check form-check-inline">
                  <input 
                    v-model="channelSmtp" 
                    class="form-check-input" 
                    type="checkbox"
                  />
                  <span class="form-check-label">SMTP (Email)</span>
                </label>
                <label class="form-check form-check-inline">
                  <input
                    v-model="channelNtfy"
                    class="form-check-input"
                    type="checkbox"
                  />
                  <span class="form-check-label">Ntfy (Push)</span>
                </label>
                <label class="form-check form-check-inline">
                  <input
                    v-model="channelBrowser"
                    class="form-check-input"
                    type="checkbox"
                  />
                  <span class="form-check-label">Navigateur</span>
                </label>
              </div>
              <div v-if="channelBrowser" class="mt-2">
                <div v-if="browserPermission === 'denied'" class="alert alert-warning py-2 small mb-0">
                  Notifications bloquées par le navigateur. Autorisez-les dans les paramètres du site.
                </div>
                <div v-else-if="browserPermission === 'granted'" class="alert alert-success py-2 small mb-0">
                  Notifications navigateur autorisées.
                </div>
                <div v-else-if="browserPermission === 'unsupported'" class="alert alert-warning py-2 small mb-0">
                  Ce navigateur ne supporte pas les notifications.
                </div>
                <div v-else class="text-secondary small mt-1">
                  La permission sera demandée à l'enregistrement.
                </div>
              </div>
            </div>

            <div v-if="channelSmtp" class="mb-3">
              <label class="form-label">Destinataire(s) email</label>
              <input
                v-model="form.actions.smtp_to"
                type="text"
                class="form-control"
                placeholder="admin@example.com, ops@example.com"
              />
              <small class="form-hint">Séparez plusieurs emails par des virgules</small>
            </div>

            <div v-if="channelNtfy" class="mb-3">
              <label class="form-label">Topic ntfy</label>
              <input
                v-model="form.actions.ntfy_topic"
                type="text"
                class="form-control"
                placeholder="mon-serveur-alerts"
              />
            </div>

            <!-- Command trigger -->
            <div class="mb-3">
              <label class="form-check mb-2">
                <input v-model="commandTriggerEnabled" class="form-check-input" type="checkbox" />
                <span class="form-check-label fw-medium">Déclencher une commande à l'alerte</span>
              </label>
              <div v-if="commandTriggerEnabled" class="border rounded p-3 bg-dark-subtle">
                <div class="row g-2">
                  <div class="col-md-4">
                    <label class="form-label form-label-sm">Module</label>
                    <select v-model="form.actions.command_trigger.module" class="form-select form-select-sm" @change="onCommandModuleChange">
                      <option value="processes">Processus (top)</option>
                      <option value="journal">Journal systemd</option>
                      <option value="systemd">Service systemd</option>
                      <option value="docker">Conteneur Docker</option>
                    </select>
                  </div>
                  <div class="col-md-4">
                    <label class="form-label form-label-sm">Action</label>
                    <select v-model="form.actions.command_trigger.action" class="form-select form-select-sm">
                      <option v-for="a in commandActions" :key="a" :value="a">{{ a }}</option>
                    </select>
                  </div>
                  <div class="col-md-4" v-if="commandNeedsTarget">
                    <label class="form-label form-label-sm">Cible</label>
                    <input
                      v-model="form.actions.command_trigger.target"
                      class="form-control form-control-sm"
                      :placeholder="commandTargetPlaceholder"
                    />
                  </div>
                </div>
                <small class="form-hint mt-1">La commande sera créée automatiquement sur l'hôte concerné dès le déclenchement de l'alerte.</small>
              </div>
            </div>

            <div class="mb-3">
              <label class="form-check">
                <input
                  v-model="form.enabled"
                  class="form-check-input"
                  type="checkbox"
                />
                <span class="form-check-label">Activer immédiatement</span>
              </label>
            </div>

            <!-- Test results panel (inside modal-body) -->
            <div v-if="testResults" class="mt-3">
              <div v-if="hasNoDataResults" class="alert alert-warning py-2 small mb-2">
                <strong>⚠ Aucune donnée disponible</strong> pour un ou plusieurs hôtes — l'agent n'est peut-être pas actif, ou la <strong>Durée</strong> configurée est inférieure à l'intervalle de collecte de l'agent (généralement 60s).
              </div>
              <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="fw-bold">
                  Résultat du test
                  <span v-if="testResults.any_fires" class="badge bg-danger-lt text-danger ms-2">Déclencherait une alerte</span>
                  <span v-else class="badge bg-success-lt text-success ms-2">Aucune alerte déclenchée</span>
                </div>
                <span class="text-secondary small">{{ formatDate(testResults.evaluated_at) }}</span>
              </div>
              <div class="table-responsive">
                <table class="table table-sm table-vcenter card-table">
                  <thead>
                    <tr>
                      <th>Hôte</th>
                      <th>Valeur actuelle</th>
                      <th>Résultat</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-if="!testResults.results?.length">
                      <td colspan="3" class="text-center text-secondary">Aucun hôte concerné</td>
                    </tr>
                    <tr v-for="r in testResults.results" :key="r.host_id">
                      <td class="fw-medium">{{ r.host_name }}</td>
                      <td>
                        <span v-if="r.has_data">
                          {{ r.current_value.toFixed(1) }}{{ getMetricUnit(form.metric) }}
                        </span>
                        <span v-else class="text-secondary">Pas de données</span>
                      </td>
                      <td>
                        <span v-if="r.would_fire" class="badge bg-danger-lt text-danger">Alerte</span>
                        <span v-else class="badge bg-success-lt text-success">OK</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
          <div v-if="saveError" class="alert alert-danger mx-3 mb-0 mt-0 py-2 small" role="alert">
            {{ saveError }}
          </div>
          <div class="modal-footer">
            <button @click="closeModal" type="button" class="btn btn-link">Annuler</button>
            <button @click="testAlert" type="button" class="btn btn-outline-secondary" :disabled="testing || saving">
              <span v-if="testing" class="spinner-border spinner-border-sm me-2"></span>
              <svg v-else class="icon me-1" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
              </svg>
              {{ testing ? 'Test en cours...' : 'Tester' }}
            </button>
            <button @click="saveAlert" type="button" class="btn btn-primary" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-2"></span>
              {{ editingRule ? 'Mettre à jour' : 'Créer' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="showModal" class="modal-backdrop fade show"></div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { formatDurationSecs } from '../utils/formatters'
import apiClient from '../api'

const { confirm } = useConfirmDialog()
const route = useRoute()

// Tabs
const alertsTab = ref('rules')

// Incidents tab state
const incidents = ref([])
const incidentsLoading = ref(false)
const incidentsError = ref('')
const incidentsLoaded = ref(false)

const activeIncidentCount = computed(() => incidents.value.filter(i => !i.resolved_at).length)

async function loadIncidents() {
  incidentsLoading.value = true
  incidentsError.value = ''
  try {
    const res = await apiClient.getNotifications()
    incidents.value = res.data?.notifications || []
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

function incidentMetricLabel(metric) {
  const labels = { cpu: 'CPU', cpu_percent: 'CPU', memory: 'RAM', ram_percent: 'RAM', disk: 'Disque', disk_percent: 'Disque', load: 'Load avg', status_offline: 'Statut hôte' }
  return labels[metric] || metric || ''
}

function incidentFormatValue(value, metric) {
  if (metric === 'status_offline') return value === 1 ? 'offline' : 'online'
  const unit = ['cpu', 'cpu_percent', 'memory', 'ram_percent', 'disk', 'disk_percent'].includes(metric) ? '%' : ''
  return `${Number(value).toFixed(2)}${unit}`
}

const rules = ref([])
const hosts = ref([])
const loading = ref(true)
const showModal = ref(false)
const saving = ref(false)
const saveError = ref('')
const testing = ref(false)
const testResults = ref(null)
const editingRule = ref(null)

const hasNoDataResults = computed(() =>
  testResults.value?.results?.some(r => !r.has_data) || false
)

const defaultCommandTrigger = () => ({ module: 'processes', action: 'list', target: '' })

const form = ref({
  name: '',
  enabled: true,
  host_id: null,
  metric: 'cpu',
  operator: '>',
  threshold: 80,
  duration: 300,
  actions: {
    channels: [],
    smtp_to: '',
    ntfy_topic: '',
    cooldown: 3600,
    command_trigger: defaultCommandTrigger(),
  }
})

const channelSmtp = ref(false)
const channelNtfy = ref(false)
const channelBrowser = ref(false)
const browserPermission = ref(typeof Notification !== 'undefined' ? Notification.permission : 'unsupported')
const commandTriggerEnabled = ref(false)

let autoTestTimer = null
watch(
  () => [form.value.host_id, form.value.metric, form.value.operator, form.value.threshold, form.value.duration],
  () => {
    if (!showModal.value) return
    clearTimeout(autoTestTimer)
    autoTestTimer = setTimeout(testAlert, 600)
  }
)

const commandModuleActions = {
  processes: ['list'],
  journal: ['read'],
  systemd: ['status', 'start', 'stop', 'restart'],
  docker: ['logs', 'restart', 'start', 'stop'],
}

const commandActions = computed(() => {
  const mod = form.value.actions.command_trigger?.module || 'processes'
  return commandModuleActions[mod] || ['list']
})

const commandNeedsTarget = computed(() => {
  const mod = form.value.actions.command_trigger?.module
  return mod === 'journal' || mod === 'systemd' || mod === 'docker'
})

const commandTargetPlaceholder = computed(() => {
  const mod = form.value.actions.command_trigger?.module
  if (mod === 'journal' || mod === 'systemd') return 'nom du service (ex: nginx)'
  if (mod === 'docker') return 'nom du conteneur'
  return ''
})

function onCommandModuleChange() {
  const mod = form.value.actions.command_trigger.module
  const actions = commandModuleActions[mod] || ['list']
  form.value.actions.command_trigger.action = actions[0]
  form.value.actions.command_trigger.target = ''
}


function onKeyDown(e) {
  if (e.key === 'Escape' && showModal.value) closeModal()
}

onMounted(async () => {
  if (route.query.tab === 'incidents') switchToIncidents()
  await loadRules()
  await loadHosts()
  document.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeyDown)
  clearTimeout(autoTestTimer)
})

async function loadRules() {
  try {
    loading.value = true
    const res = await apiClient.getAlertRules()
    rules.value = res.data || []
  } catch (err) {
    console.error('Failed to load alert rules:', err)
  } finally {
    loading.value = false
  }
}

async function loadHosts() {
  try {
    const res = await apiClient.getHosts()
    hosts.value = res.data || []
  } catch (err) {
    console.error('Failed to load hosts:', err)
  }
}

function startAddAlert() {
  editingRule.value = null
  form.value = {
    name: '',
    enabled: true,
    host_id: null,
    metric: 'cpu',
    operator: '>',
    threshold: 80,
    duration: 300,
    actions: { channels: [], smtp_to: '', ntfy_topic: '', cooldown: 3600, command_trigger: defaultCommandTrigger() }
  }
  channelSmtp.value = false
  channelNtfy.value = false
  channelBrowser.value = false
  commandTriggerEnabled.value = false
  showModal.value = true
}

function startEditAlert(rule) {
  testResults.value = null
  editingRule.value = rule
  const act = rule.actions || {}
  const ct = act.command_trigger
  form.value = {
    name: rule.name || '',
    enabled: rule.enabled,
    host_id: rule.host_id,
    metric: rule.metric,
    operator: rule.operator,
    threshold: rule.threshold,
    duration: rule.duration_seconds,
    actions: {
      channels: act.channels || [],
      smtp_to: act.smtp_to || '',
      ntfy_topic: act.ntfy_topic || '',
      cooldown: act.cooldown || 3600,
      command_trigger: ct ? { module: ct.module, action: ct.action, target: ct.target || '' } : defaultCommandTrigger(),
    }
  }
  channelSmtp.value = act.channels?.includes('smtp') || false
  channelNtfy.value = act.channels?.includes('ntfy') || false
  channelBrowser.value = act.channels?.includes('browser') || false
  commandTriggerEnabled.value = !!ct
  showModal.value = true
}

async function saveAlert() {
  saveError.value = ''
  try {
    saving.value = true

    const channels = []
    if (channelSmtp.value) channels.push('smtp')
    if (channelNtfy.value) channels.push('ntfy')
    if (channelBrowser.value) channels.push('browser')

    // Demande de permission navigateur si nécessaire
    if (channelBrowser.value && typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
      const perm = await Notification.requestPermission()
      browserPermission.value = perm
    }

    const actions = { ...form.value.actions, channels }
    if (!commandTriggerEnabled.value) {
      delete actions.command_trigger
    }
    const payload = { ...form.value, actions }

    if (editingRule.value) {
      await apiClient.updateAlertRule(editingRule.value.id, payload)
    } else {
      await apiClient.createAlertRule(payload)
    }

    await loadRules()
    closeModal()
  } catch (err) {
    console.error('Failed to save alert:', err)
    saveError.value = 'Erreur : ' + (err.response?.data?.error || err.message)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(rule) {
  try {
    await apiClient.updateAlertRule(rule.id, { enabled: !rule.enabled })
    await loadRules()
  } catch (err) {
    console.error('Failed to toggle alert:', err)
  }
}

async function deleteAlert(rule) {
  const confirmed = await confirm({
    title: 'Supprimer l\'alerte ?',
    message: `Voulez-vous vraiment supprimer la règle "${rule.name || 'Sans nom'}" ?\n\nCette action est irréversible.`,
    variant: 'danger'
  })

  if (!confirmed) return

  try {
    await apiClient.deleteAlertRule(rule.id)
    await loadRules()
  } catch (err) {
    console.error('Failed to delete alert:', err)
    saveError.value = 'Erreur lors de la suppression : ' + (err.response?.data?.error || err.message)
  }
}

async function testAlert() {
  testing.value = true
  testResults.value = null
  try {
    const channels = []
    if (channelSmtp.value) channels.push('smtp')
    if (channelNtfy.value) channels.push('ntfy')
    if (channelBrowser.value) channels.push('browser')
    const testActions = { ...form.value.actions, channels }
    if (!commandTriggerEnabled.value) delete testActions.command_trigger
    const res = await apiClient.testAlertRule({
      ...form.value,
      actions: testActions,
    })
    testResults.value = res.data
  } catch (err) {
    console.error('Test alert failed:', err)
  } finally {
    testing.value = false
  }
}

function closeModal() {
  showModal.value = false
  editingRule.value = null
  testResults.value = null
  saveError.value = ''
  commandTriggerEnabled.value = false
}

function getHostName(hostId) {
  return hosts.value.find(h => h.id === hostId)?.name || hostId
}

function getMetricLabel(metric) {
  const labels = {
    cpu: 'CPU',
    memory: 'Mémoire',
    disk: 'Disque',
    load: 'Load'
  }
  return labels[metric] || metric
}

function getMetricBadgeClass(metric) {
  const classes = {
    cpu: 'bg-red-lt',
    memory: 'bg-blue-lt',
    disk: 'bg-yellow-lt',
    load: 'bg-purple-lt'
  }
  return classes[metric] || 'bg-secondary-lt'
}

function getMetricUnit(metric) {
  const units = {
    cpu: '%',
    memory: '%',
    disk: '%',
    load: ''
  }
  return units[metric] || ''
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleString('fr-FR')
}
</script>

<style scoped>
.modal {
  background: rgba(0, 0, 0, 0.5);
}
</style>

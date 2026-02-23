<template>
  <div class="page-wrapper">
    <div class="page-header d-print-none">
      <div class="container-xl">
        <div class="row g-2 align-items-center">
          <div class="col">
            <h2 class="page-title">Règles d'Alertes</h2>
            <div class="text-muted mt-1">Configurez des alertes pour surveiller vos hôtes automatiquement</div>
          </div>
          <div class="col-auto ms-auto">
            <button @click="startAddAlert" class="btn btn-primary">
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
        <!-- Rules Table -->
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
                    <span v-if="rule.host_id" class="badge bg-secondary">{{ getHostName(rule.host_id) }}</span>
                    <span v-else class="badge bg-info">Tous les hôtes</span>
                  </td>
                  <td>
                    <span class="badge" :class="getMetricBadgeClass(rule.metric)">
                      {{ getMetricLabel(rule.metric) }}
                    </span>
                  </td>
                  <td>
                    <code>{{ rule.operator }} {{ rule.threshold }}{{ getMetricUnit(rule.metric) }}</code>
                  </td>
                  <td>{{ rule.duration_seconds }}s</td>
                  <td>
                    <span v-for="channel in rule.channels" :key="channel" class="badge bg-azure me-1">
                      {{ channel }}
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
              <small class="form-hint">Le seuil doit être dépassé pendant cette durée avant de déclencher l'alerte</small>
            </div>

            <div class="mb-3">
              <label class="form-label">Période de silence (secondes)</label>
              <input 
                v-model.number="form.cooldown" 
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
              </div>
            </div>

            <div v-if="channelSmtp" class="mb-3">
              <label class="form-label">Destinataire(s) email</label>
              <input 
                v-model="form.smtp_to" 
                type="text" 
                class="form-control"
                placeholder="admin@example.com, ops@example.com"
              />
              <small class="form-hint">Séparez plusieurs emails par des virgules</small>
            </div>

            <div v-if="channelNtfy" class="mb-3">
              <label class="form-label">Topic ntfy</label>
              <input 
                v-model="form.ntfy_topic" 
                type="text" 
                class="form-control"
                placeholder="mon-serveur-alerts"
              />
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
          </div>
          <div class="modal-footer">
            <button @click="closeModal" type="button" class="btn btn-link">Annuler</button>
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
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import api from '../api'

const auth = useAuthStore()
const { confirm } = useConfirmDialog()

const rules = ref([])
const hosts = ref([])
const loading = ref(true)
const showModal = ref(false)
const saving = ref(false)
const editingRule = ref(null)

const form = ref({
  name: '',
  enabled: true,
  host_id: null,
  metric: 'cpu',
  operator: '>',
  threshold: 80,
  duration: 300,
  cooldown: 3600,
  smtp_to: '',
  ntfy_topic: ''
})

const channelSmtp = ref(false)
const channelNtfy = ref(false)

onMounted(async () => {
  await loadRules()
  await loadHosts()
})

async function loadRules() {
  try {
    loading.value = true
    const res = await api.get('/api/v1/alert-rules')
    rules.value = res.data || []
  } catch (err) {
    console.error('Failed to load alert rules:', err)
  } finally {
    loading.value = false
  }
}

async function loadHosts() {
  try {
    const res = await api.get('/api/v1/hosts')
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
    cooldown: 3600,
    smtp_to: '',
    ntfy_topic: ''
  }
  channelSmtp.value = false
  channelNtfy.value = false
  showModal.value = true
}

function startEditAlert(rule) {
  editingRule.value = rule
  form.value = {
    name: rule.name || '',
    enabled: rule.enabled,
    host_id: rule.host_id,
    metric: rule.metric,
    operator: rule.operator,
    threshold: rule.threshold,
    duration: rule.duration_seconds,
    cooldown: rule.cooldown || 3600,
    smtp_to: rule.smtp_to || '',
    ntfy_topic: rule.ntfy_topic || ''
  }
  channelSmtp.value = rule.channels?.includes('smtp') || false
  channelNtfy.value = rule.channels?.includes('ntfy') || false
  showModal.value = true
}

async function saveAlert() {
  try {
    saving.value = true
    
    const channels = []
    if (channelSmtp.value) channels.push('smtp')
    if (channelNtfy.value) channels.push('ntfy')
    
    const payload = {
      ...form.value,
      channels
    }

    if (editingRule.value) {
      await api.patch(`/api/v1/alert-rules/${editingRule.value.id}`, payload)
    } else {
      await api.post('/api/v1/alert-rules', payload)
    }

    await loadRules()
    closeModal()
  } catch (err) {
    console.error('Failed to save alert:', err)
    alert('Erreur lors de la sauvegarde de l\'alerte: ' + err.response?.data?.error || err.message)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(rule) {
  try {
    await api.patch(`/api/v1/alert-rules/${rule.id}`, {
      enabled: !rule.enabled
    })
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
    await api.delete(`/api/v1/alert-rules/${rule.id}`)
    await loadRules()
  } catch (err) {
    console.error('Failed to delete alert:', err)
    alert('Erreur lors de la suppression: ' + err.response?.data?.error || err.message)
  }
}

function closeModal() {
  showModal.value = false
  editingRule.value = null
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

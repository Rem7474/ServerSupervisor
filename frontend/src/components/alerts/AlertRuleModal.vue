<template>
  <div v-if="visible">
    <div class="modal modal-blur fade show" style="display: block" tabindex="-1">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ rule ? 'Modifier l\'alerte' : 'Nouvelle alerte' }}</h5>
            <button @click="close" type="button" class="btn-close"></button>
          </div>
          <div class="modal-body">
            <div class="alert-steps mb-4">
              <div class="step-chip" :class="{ active: step === 1, done: step > 1 }">
                <span class="step-chip-index">1</span>
                <span>Quoi surveiller</span>
              </div>
              <div class="step-chip" :class="{ active: step === 2, done: step > 2 }">
                <span class="step-chip-index">2</span>
                <span>Conditions</span>
              </div>
              <div class="step-chip" :class="{ active: step === 3 }">
                <span class="step-chip-index">3</span>
                <span>Notification</span>
              </div>
            </div>

            <div v-if="step === 1">
              <div class="mb-3">
                <label class="form-label required">Nom</label>
                <input v-model="form.name" type="text" class="form-control" placeholder="Ex: CPU élevé sur serveur web" />
              </div>

              <div class="mb-3">
                <label class="form-label">Hôte cible</label>
                <select v-model="form.host_id" class="form-select">
                  <option :value="null">Tous les hôtes</option>
                  <option v-for="host in hosts" :key="host.id" :value="host.id">{{ host.name }}</option>
                </select>
              </div>

              <div class="mb-2 fw-semibold">Choisissez une métrique à surveiller</div>
              <div class="metric-grid">
                <button
                  v-for="metric in metricCards"
                  :key="metric.value"
                  type="button"
                  class="metric-card"
                  :class="{ selected: form.metric === metric.value }"
                  @click="selectMetric(metric.value)"
                >
                  <span class="metric-icon">{{ metric.icon }}</span>
                  <span class="metric-label">{{ metric.label }}</span>
                </button>
              </div>
              <div class="text-secondary small mt-2">
                Note: la température correspond à la température disque (SMART), pas à la température CPU.
              </div>
            </div>

            <div v-if="step === 2">
              <div class="row">
                <div v-if="form.metric !== 'heartbeat_timeout'" class="col-md-6 mb-3">
                  <label class="form-label required">Opérateur</label>
                  <select v-model="form.operator" class="form-select">
                    <option value=">">Supérieur à (>)</option>
                    <option value=">=">Supérieur ou égal (>=)</option>
                    <option value="<">Inférieur à (&lt;)</option>
                    <option value="<=">Inférieur ou égal (&lt;=)</option>
                  </select>
                </div>
                <div :class="form.metric !== 'heartbeat_timeout' ? 'col-md-6 mb-3' : 'col-md-12 mb-3'">
                  <label class="form-label required">{{ form.metric === 'heartbeat_timeout' ? 'Silence maximum (secondes)' : 'Seuil' }}</label>
                  <input
                    v-model.number="form.threshold"
                    type="number"
                    :step="form.metric === 'heartbeat_timeout' ? 60 : 0.1"
                    class="form-control"
                    :placeholder="form.metric === 'heartbeat_timeout' ? '300' : '80'"
                  />
                  <small v-if="form.metric === 'heartbeat_timeout'" class="form-hint">
                    Durée en secondes sans rapport avant alerte.
                  </small>
                </div>
              </div>

              <div v-if="form.metric !== 'heartbeat_timeout'" class="mb-3">
                <label class="form-label">Durée (secondes)</label>
                <input v-model.number="form.duration" type="number" class="form-control" placeholder="300" />
                <small class="form-hint">Le seuil doit être dépassé pendant cette durée avant de déclencher l'alerte.</small>
                <small v-if="form.duration > 0" class="form-hint text-warning d-block mt-1">
                  Si l'agent reporte toutes les 60s, une durée inférieure peut empêcher le déclenchement.
                </small>
              </div>

              <div v-if="testResults" class="mt-3">
                <div v-if="hasNoDataResults" class="alert alert-warning py-2 small mb-2">
                  <strong>Aucune donnée disponible</strong> pour un ou plusieurs hôtes.
                </div>
                <div class="d-flex align-items-center justify-content-between mb-2">
                  <div class="fw-bold">
                    Résultat du test
                    <span v-if="testResults.any_fires" class="badge bg-danger-lt text-danger ms-2">Declencherait une alerte</span>
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
                      <tr v-for="result in testResults.results" :key="result.host_id">
                        <td class="fw-medium">{{ result.host_name }}</td>
                        <td>
                          <span v-if="result.has_data">{{ result.current_value.toFixed(1) }}{{ getMetricUnit(form.metric) }}</span>
                          <span v-else class="text-secondary">Pas de données</span>
                        </td>
                        <td>
                          <span v-if="result.would_fire" class="badge bg-danger-lt text-danger">Alerte</span>
                          <span v-else class="badge bg-success-lt text-success">OK</span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>

            <div v-if="step === 3">
              <div class="mb-3">
                <label class="form-label">Periode de silence (secondes)</label>
                <input v-model.number="form.actions.cooldown" type="number" class="form-control" placeholder="3600" />
                <small class="form-hint">Temps minimum entre deux alertes successives pour cette regle</small>
              </div>

              <div class="mb-3">
                <label class="form-label">Canaux de notification</label>
                <div>
                  <label class="form-check form-check-inline">
                    <input v-model="channelSmtp" class="form-check-input" type="checkbox" />
                    <span class="form-check-label">SMTP (Email)</span>
                  </label>
                  <label class="form-check form-check-inline">
                    <input v-model="channelNtfy" class="form-check-input" type="checkbox" />
                    <span class="form-check-label">Ntfy (Push)</span>
                  </label>
                  <label class="form-check form-check-inline">
                    <input v-model="channelBrowser" class="form-check-input" type="checkbox" />
                    <span class="form-check-label">Navigateur</span>
                  </label>
                </div>
                <div v-if="channelBrowser" class="mt-2">
                  <div v-if="browserPermission === 'denied'" class="alert alert-warning py-2 small mb-0">
                    Notifications bloquées par le navigateur.
                  </div>
                  <div v-else-if="browserPermission === 'granted'" class="alert alert-success py-2 small mb-0">
                    Notifications navigateur autorisées.
                  </div>
                  <div v-else-if="browserPermission === 'unsupported'" class="alert alert-warning py-2 small mb-0">
                    Ce navigateur ne supporte pas les notifications.
                  </div>
                  <div v-else class="text-secondary small mt-1">La permission sera demandée à l'enregistrement.</div>
                </div>
              </div>

              <div v-if="channelSmtp" class="mb-3">
                <label class="form-label">Destinataire(s) email</label>
                <input v-model="form.actions.smtp_to" type="text" class="form-control" placeholder="admin@example.com, ops@example.com" />
                <small class="form-hint">Séparez plusieurs emails par des virgules</small>
              </div>

              <div v-if="channelNtfy" class="mb-3">
                <label class="form-label">Topic ntfy</label>
                <input v-model="form.actions.ntfy_topic" type="text" class="form-control" placeholder="mon-serveur-alerts" />
              </div>

              <AlertRuleCommandTrigger
                v-model:enabled="commandTriggerEnabled"
                :model-value="form.actions.command_trigger"
                @update:model-value="form.actions.command_trigger = $event"
              />

              <div class="mb-3">
                <label class="form-check">
                  <input v-model="form.enabled" class="form-check-input" type="checkbox" />
                  <span class="form-check-label">Activer immédiatement</span>
                </label>
              </div>
            </div>
          </div>
          <div v-if="error" class="alert alert-danger mx-3 mb-0 mt-0 py-2 small" role="alert">{{ error }}</div>
          <div class="modal-footer">
            <button @click="close" type="button" class="btn btn-link">Annuler</button>
            <button v-if="step > 1" @click="step -= 1" type="button" class="btn btn-outline-secondary" :disabled="saving">← Précédent</button>
            <button v-if="step < 3" @click="goNextStep" type="button" class="btn btn-primary" :disabled="saving || !canProceedStep">
              Suivant →
            </button>
            <button v-if="step === 3" @click="testAlert" type="button" class="btn btn-outline-secondary" :disabled="testing || saving">
              <span v-if="testing" class="spinner-border spinner-border-sm me-2"></span>
              <svg v-else class="icon me-1" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
              </svg>
              {{ testing ? 'Test en cours...' : 'Tester' }}
            </button>
            <button v-if="step === 3" @click="submit" type="button" class="btn btn-primary" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-2"></span>
              {{ rule ? 'Mettre à jour' : 'Créer' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="visible" class="modal-backdrop fade show"></div>
  </div>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import apiClient from '../../api'
import AlertRuleCommandTrigger from './AlertRuleCommandTrigger.vue'
import { useAlertRuleForm } from '../../composables/useAlertRuleForm'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false,
  },
  rule: {
    type: Object,
    default: null,
  },
  hosts: {
    type: Array,
    default: () => [],
  },
  saving: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'submit'])

const browserPermission = ref(typeof Notification !== 'undefined' ? Notification.permission : 'unsupported')
const step = ref(1)

const metricCards = [
  { value: 'cpu', label: 'CPU', icon: '⚡' },
  { value: 'memory', label: 'RAM', icon: '🧠' },
  { value: 'disk', label: 'Disque', icon: '💾' },
  { value: 'heartbeat_timeout', label: 'Heartbeat', icon: '🫀' },
  { value: 'disk_temperature', label: 'Temp. disque', icon: '🌡' },
  { value: 'proxmox_storage_percent', label: 'Proxmox', icon: '🖥' },
  { value: 'npm_requests', label: 'NPM requetes', icon: '🌐' },
  { value: 'npm_traffic_bytes', label: 'NPM trafic', icon: '📦' },
  { value: 'npm_5xx_errors', label: 'NPM 5xx', icon: '🚨' },
]

const {
  form,
  channelSmtp,
  channelNtfy,
  channelBrowser,
  commandTriggerEnabled,
  hydrateFormFromRule,
  onMetricChange,
  buildPayload,
} = useAlertRuleForm()

const testing = ref(false)
const testResults = ref(null)

const hasNoDataResults = computed(() => testResults.value?.results?.some((result) => !result.has_data) || false)

const canProceedStep = computed(() => {
  if (step.value === 1) return !!form.value.metric && !!form.value.name?.trim()
  if (step.value === 2) return Number.isFinite(Number(form.value.threshold))
  return true
})

let autoTestTimer = null

watch(
  () => [props.visible, props.rule],
  () => {
    if (!props.visible) {
      clearTimeout(autoTestTimer)
      testResults.value = null
      step.value = 1
      return
    }
    testResults.value = null
    hydrateFormFromRule(props.rule)
    step.value = 1
  },
  { immediate: true, deep: true }
)

watch(
  () => [form.value.host_id, form.value.metric, form.value.operator, form.value.threshold, form.value.duration],
  () => {
    if (!props.visible) return
    clearTimeout(autoTestTimer)
    autoTestTimer = setTimeout(testAlert, 600)
  }
)

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      document.addEventListener('keydown', onKeyDown)
      return
    }
    document.removeEventListener('keydown', onKeyDown)
  },
  { immediate: true }
)

onUnmounted(() => {
  clearTimeout(autoTestTimer)
  document.removeEventListener('keydown', onKeyDown)
})

async function submit() {
  if (channelBrowser.value && typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
    browserPermission.value = await Notification.requestPermission()
  }
  emit('submit', buildPayload())
}

async function testAlert() {
  if (!props.visible) return
  testing.value = true
  testResults.value = null
  try {
    const response = await apiClient.testAlertRule(buildPayload())
    testResults.value = response.data
  } catch {
    testResults.value = null
  } finally {
    testing.value = false
  }
}

function close() {
  emit('close')
}

function onKeyDown(event) {
  if (event.key === 'Escape' && props.visible) close()
}

function selectMetric(metric) {
  form.value.metric = metric
  onMetricChange()
}

function goNextStep() {
  if (!canProceedStep.value || step.value >= 3) return
  step.value += 1
}

function getMetricUnit(metric) {
  const units = {
    cpu: '%',
    memory: '%',
    disk: '%',
    load: '',
    heartbeat_timeout: 's',
    disk_smart_status: '',
    disk_temperature: '°C',
    proxmox_storage_percent: '%',
    npm_requests: 'req',
    npm_traffic_bytes: 'B',
    npm_5xx_errors: 'err',
  }
  return units[metric] || ''
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>

<style scoped>
.alert-steps {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.step-chip {
  align-items: center;
  background: var(--tblr-bg-surface, #f3f6fa);
  border: 1px solid var(--tblr-border-color, #dbe3ec);
  border-radius: 0.6rem;
  color: var(--tblr-body-color, #4c5b6b);
  display: flex;
  font-weight: 600;
  gap: 0.5rem;
  justify-content: center;
  min-height: 44px;
  padding: 0.4rem 0.6rem;
}

.step-chip.active {
  background: rgba(45, 140, 255, 0.14);
  border-color: #4b9bff;
  color: #8ec2ff;
}

.step-chip.done {
  background: rgba(61, 196, 126, 0.12);
  border-color: #57c48b;
  color: #63d39a;
}

.step-chip-index {
  background: rgba(255, 255, 255, 0.75);
  color: #1f2d3d;
  border-radius: 999px;
  display: inline-flex;
  font-size: 0.85rem;
  font-weight: 700;
  height: 24px;
  justify-content: center;
  width: 24px;
}

.metric-grid {
  display: grid;
  gap: 0.8rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-card {
  align-items: center;
  background: var(--tblr-bg-surface, #ffffff);
  border: 1px solid var(--tblr-border-color, #d9e2ee);
  border-radius: 0.8rem;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  justify-content: center;
  min-height: 90px;
  padding: 0.8rem;
  transition: all 0.15s ease;
}

.metric-card:hover {
  border-color: #89b8ff;
  box-shadow: 0 2px 10px rgba(66, 132, 245, 0.18);
}

.metric-card.selected {
  background: linear-gradient(160deg, rgba(45, 140, 255, 0.14) 0%, rgba(45, 140, 255, 0.06) 100%);
  border-color: #4b9bff;
  box-shadow: inset 0 0 0 1px #4b9bff;
}

.metric-icon {
  font-size: 1.2rem;
  line-height: 1;
}

.metric-label {
  color: var(--tblr-body-color, #1f2d3d);
  font-size: 0.92rem;
  font-weight: 600;
}

[data-bs-theme='dark'] .step-chip {
  background: #1f2a3a;
  border-color: #2f3f57;
  color: #c9d6ea;
}

[data-bs-theme='dark'] .step-chip.active {
  background: rgba(33, 118, 210, 0.28);
  border-color: #4b9bff;
  color: #d2e6ff;
}

[data-bs-theme='dark'] .step-chip.done {
  background: rgba(56, 142, 99, 0.24);
  border-color: #4fb37f;
  color: #c7f2da;
}

[data-bs-theme='dark'] .step-chip-index {
  background: rgba(255, 255, 255, 0.16);
  color: #d6e4fb;
}

[data-bs-theme='dark'] .metric-card {
  background: #1f2a3a;
  border-color: #2f3f57;
}

[data-bs-theme='dark'] .metric-card.selected {
  background: linear-gradient(160deg, rgba(33, 118, 210, 0.34) 0%, rgba(18, 79, 150, 0.2) 100%);
}

@media (max-width: 768px) {
  .alert-steps {
    grid-template-columns: 1fr;
  }

  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
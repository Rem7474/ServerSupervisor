<template>
  <div>
    <!-- ── docker_container_state ───────────────────────────────────── -->
    <template v-if="form.metric === 'docker_container_state'">
      <div class="row">
        <div class="col-md-6 mb-3">
          <label class="form-label">États déclenchant une alerte <span class="badge bg-warning-lt text-warning ms-1">warn</span></label>
          <div class="border rounded p-2 d-flex flex-wrap gap-2">
            <label
              v-for="s in DOCKER_STATES"
              :key="'w-' + s"
              class="form-check form-check-inline mb-0"
            >
              <input
                type="checkbox"
                class="form-check-input"
                :checked="form.docker_scope.warn_states.includes(s)"
                @change="toggleState('warn', s, ($event.target as HTMLInputElement).checked)"
              >
              <span class="form-check-label"><code>{{ s }}</code></span>
            </label>
          </div>
        </div>
        <div class="col-md-6 mb-3">
          <label class="form-label">États déclenchant une alerte <span class="badge bg-danger-lt text-danger ms-1">crit</span></label>
          <div class="border rounded p-2 d-flex flex-wrap gap-2">
            <label
              v-for="s in DOCKER_STATES"
              :key="'c-' + s"
              class="form-check form-check-inline mb-0"
            >
              <input
                type="checkbox"
                class="form-check-input"
                :checked="form.docker_scope.crit_states.includes(s)"
                @change="toggleState('crit', s, ($event.target as HTMLInputElement).checked)"
              >
              <span class="form-check-label"><code>{{ s }}</code></span>
            </label>
          </div>
        </div>
      </div>
      <div
        v-if="dockerStateNoSelection"
        class="alert alert-warning py-2 small mb-3"
      >
        Sélectionnez au moins un état pour activer cette alerte.
      </div>
      <div class="mb-3">
        <label class="form-label">Durée (secondes)</label>
        <input
          v-model.number="form.duration"
          type="number"
          min="0"
          class="form-control"
          placeholder="0"
          :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`duration-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >
          Durée pendant laquelle l'état doit persister avant de déclencher l'alerte (0 = immédiat).
        </small>
      </div>
    </template>

    <!-- ── docker_compose_degraded_services ────────────────────────── -->
    <template v-else-if="form.metric === 'docker_compose_degraded_services'">
      <div class="alert alert-light py-2 small mb-3 border">
        La valeur est le <strong>nombre de services déclarés dans le compose.yml qui n'ont aucun container running</strong>.
        Un service est dégradé si tous ses containers sont arrêtés.
        Valeur 0 = projet en bonne santé.
      </div>
      <div class="row">
        <div class="col-md-6 mb-3">
          <label class="form-label required">Warn si ≥ N service(s) dégradé(s)</label>
          <input
            v-model.number="form.threshold_warn"
            type="number"
            step="1"
            min="1"
            class="form-control"
            placeholder="1"
          >
          <small class="form-hint">
            Alerte warn dès {{ form.threshold_warn ?? 1 }} service{{ (form.threshold_warn ?? 1) > 1 ? 's' : '' }} dégradé{{ (form.threshold_warn ?? 1) > 1 ? 's' : '' }}.
          </small>
        </div>
        <div class="col-md-6 mb-3">
          <label class="form-label required">Crit si ≥ N service(s) dégradé(s)</label>
          <input
            v-model.number="form.threshold_crit"
            type="number"
            step="1"
            min="1"
            class="form-control"
            placeholder="1"
          >
          <small class="form-hint">
            Alerte critique dès {{ form.threshold_crit ?? 1 }} service{{ (form.threshold_crit ?? 1) > 1 ? 's' : '' }} dégradé{{ (form.threshold_crit ?? 1) > 1 ? 's' : '' }}.
          </small>
        </div>
      </div>
      <div class="mb-3">
        <label class="form-label">Durée (secondes)</label>
        <input
          v-model.number="form.duration"
          type="number"
          min="0"
          class="form-control"
          placeholder="0"
          :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`duration-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >
          Le seuil doit être atteint pendant cette durée avant de déclencher l'alerte.
        </small>
      </div>
    </template>

    <!-- ── heartbeat_timeout ────────────────────────────────────────── -->
    <template v-else-if="form.metric === 'heartbeat_timeout'">
      <div class="row">
        <div class="col-md-12 mb-3">
          <label class="form-label required">Silence maximum (secondes)</label>
          <input
            v-model.number="form.threshold_crit"
            type="number"
            step="60"
            class="form-control"
            placeholder="300"
            :aria-describedby="`heartbeat-hint-${rule?.id || 'new'}`"
          >
          <small
            :id="`heartbeat-hint-${rule?.id || 'new'}`"
            class="form-hint"
          >
            Durée en secondes sans rapport avant alerte.
          </small>
        </div>
      </div>
    </template>

    <!-- ── Generic form (agent / proxmox / synthetic) ───────────────── -->
    <template v-else>
      <div class="row">
        <div class="col-md-6 mb-3">
          <label class="form-label required">Opérateur</label>
          <select
            v-model="form.operator"
            class="form-select"
          >
            <option value=">">
              Supérieur à (>)
            </option>
            <option value=">=">
              Supérieur ou égal (>=)
            </option>
            <option value="<">
              Inférieur à (&lt;)
            </option>
            <option value="<=">
              Inférieur ou égal (&lt;=)
            </option>
          </select>
        </div>
      </div>

      <div class="row">
        <div class="col-md-6 mb-3">
          <label class="form-label required">Seuil d'avertissement (warn)</label>
          <input
            v-model.number="form.threshold_warn"
            type="number"
            step="0.1"
            class="form-control"
            placeholder="70"
          >
          <small class="form-hint">Déclenche une alerte de niveau avertissement.</small>
        </div>
        <div class="col-md-6 mb-3">
          <label class="form-label required">Seuil critique (crit)</label>
          <input
            v-model.number="form.threshold_crit"
            type="number"
            step="0.1"
            class="form-control"
            placeholder="85"
          >
          <small class="form-hint">Déclenche une alerte de niveau critique.</small>
        </div>
      </div>

      <div class="row">
        <div class="col-md-6 mb-3">
          <label class="form-label">Seuil de résolution warn (hystérésis)</label>
          <input
            v-model.number="form.threshold_clear_warn"
            type="number"
            step="0.1"
            class="form-control"
            :class="{ 'is-invalid': clearWarnIncoherent }"
            :placeholder="clearPlaceholder('warn')"
            :aria-describedby="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
          >
          <small
            v-if="clearWarnIncoherent"
            class="invalid-feedback d-block"
          >
            {{ incoherenceMessage('warn') }}
          </small>
          <small
            :id="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
            class="form-hint"
          >
            Valeur <strong>exacte</strong> à laquelle l'alerte warn se résout (ex. {{ clearExample('warn') }}). Laisser vide = se résout dès que le seuil n'est plus dépassé.
          </small>
        </div>
        <div class="col-md-6 mb-3">
          <label class="form-label">Seuil de résolution crit (hystérésis)</label>
          <input
            v-model.number="form.threshold_clear_crit"
            type="number"
            step="0.1"
            class="form-control"
            :class="{ 'is-invalid': clearCritIncoherent }"
            :placeholder="clearPlaceholder('crit')"
            :aria-describedby="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
          >
          <small
            v-if="clearCritIncoherent"
            class="invalid-feedback d-block"
          >
            {{ incoherenceMessage('crit') }}
          </small>
          <small
            :id="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
            class="form-hint"
          >
            Valeur <strong>exacte</strong> à laquelle l'alerte crit se résout (ex. {{ clearExample('crit') }}). Laisser vide = se résout dès que le seuil n'est plus dépassé.
          </small>
        </div>
      </div>

      <div class="mb-3">
        <label class="form-label">Durée (secondes)</label>
        <input
          v-model.number="form.duration"
          type="number"
          class="form-control"
          placeholder="300"
          :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`duration-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >Le seuil doit être dépassé pendant cette durée avant de déclencher l'alerte.</small>
        <small
          v-if="Number.isFinite(Number(form.duration)) && form.duration > 0 && form.duration < 60"
          :id="`duration-warn-${rule?.id || 'new'}`"
          class="form-hint text-warning d-block mt-1"
        >
          Si l'agent reporte toutes les 60s, une durée inférieure peut empêcher le déclenchement.
        </small>
      </div>
    </template>

    <!-- ── Test results (all metrics) ───────────────────────────────── -->
    <div
      v-if="testResults"
      class="mt-3"
    >
      <div
        v-if="hasNoDataResults"
        class="alert alert-warning py-2 small mb-2"
      >
        <strong>Aucune donnée disponible</strong> pour un ou plusieurs hôtes.
      </div>
      <div class="d-flex align-items-center justify-content-between mb-2">
        <div class="fw-bold">
          Résultat du test
          <span
            v-if="testResults.any_fires"
            class="badge bg-danger-lt text-danger ms-2"
          >Déclencherait une alerte</span>
          <span
            v-else
            class="badge bg-success-lt text-success ms-2"
          >Aucune alerte déclenchée</span>
        </div>
        <div class="d-flex align-items-center gap-2">
          <button
            v-if="canDownloadTestLogs"
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="downloadingLogs"
            @click="emit('download-logs')"
          >
            <span
              v-if="downloadingLogs"
              class="spinner-border spinner-border-sm me-1"
            />
            Télécharger logs
          </button>
          <span class="text-secondary small">{{ formatDate(testResults.evaluated_at) }}</span>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-sm table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ testResultColLabel }}</th>
              <th>Valeur actuelle</th>
              <th>Résultat</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!testResults.results?.length">
              <td
                colspan="3"
                class="text-center text-secondary"
              >
                Aucun hôte concerné
              </td>
            </tr>
            <tr
              v-for="result in testResults.results"
              :key="result.host_id"
            >
              <td class="fw-medium">
                {{ result.host_name }}
              </td>
              <td>
                <span v-if="result.has_data">{{ formatTestValue(result.current_value) }}</span>
                <span
                  v-else
                  class="text-secondary"
                >Pas de données</span>
              </td>
              <td>
                <span
                  v-if="result.would_fire"
                  class="badge bg-danger-lt text-danger"
                >Alerte</span>
                <span
                  v-else
                  class="badge bg-success-lt text-success"
                >OK</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

interface TestResultRow {
  host_id: string
  host_name: string
  has_data: boolean
  current_value: number
  would_fire: boolean
}
interface TestResults {
  any_fires?: boolean
  evaluated_at?: string
  results?: TestResultRow[]
}

const props = defineProps<{
  form: AlertRuleFormData
  rule?: { id?: number | string } | null
  testResults?: TestResults | null
  hasNoDataResults?: boolean
  canDownloadTestLogs?: boolean
  downloadingLogs?: boolean
  metricUnit?: string
}>()

const emit = defineEmits<{ (e: 'download-logs'): void }>()

type Severity = 'warn' | 'crit'

// ── Docker state helpers ─────────────────────────────────────────────

const DOCKER_STATES = ['created', 'paused', 'restarting', 'exited', 'dead'] as const

const dockerStateNoSelection = computed(
  () => props.form.metric === 'docker_container_state' &&
    props.form.docker_scope.warn_states.length === 0 &&
    props.form.docker_scope.crit_states.length === 0
)

function toggleState(level: 'warn' | 'crit', state: string, checked: boolean): void {
  const arr = level === 'warn' ? props.form.docker_scope.warn_states : props.form.docker_scope.crit_states
  if (checked) {
    if (!arr.includes(state)) arr.push(state)
  } else {
    const idx = arr.indexOf(state)
    if (idx >= 0) arr.splice(idx, 1)
  }
}

// ── Generic hysteresis helpers (generic form only) ──────────────────

const isDescending = computed(() => props.form.operator === '<' || props.form.operator === '<=')

function triggerThreshold(sev: Severity): number | null {
  const v = sev === 'warn' ? props.form.threshold_warn : props.form.threshold_crit
  return Number.isFinite(Number(v)) ? Number(v) : null
}

function clearValue(sev: Severity): number | null {
  const v = sev === 'warn' ? props.form.threshold_clear_warn : props.form.threshold_clear_crit
  return Number.isFinite(Number(v)) ? Number(v) : null
}

function isIncoherent(sev: Severity): boolean {
  const trigger = triggerThreshold(sev)
  const clear = clearValue(sev)
  if (trigger === null || clear === null) return false
  return isDescending.value ? clear < trigger : clear > trigger
}

const clearWarnIncoherent = computed(() => isIncoherent('warn'))
const clearCritIncoherent = computed(() => isIncoherent('crit'))

const unitLabel = computed(() => props.metricUnit || '')

function clearExample(sev: Severity): string {
  const trigger = triggerThreshold(sev)
  if (trigger === null) return isDescending.value ? '72' : '68'
  const suggestion = isDescending.value ? trigger + 2 : trigger - 2
  return `${suggestion}${unitLabel.value}`
}

function clearPlaceholder(sev: Severity): string {
  return `ex. ${clearExample(sev)} — vide = auto`
}

function incoherenceMessage(sev: Severity): string {
  const trigger = triggerThreshold(sev)
  const side = isDescending.value ? `≥ ${trigger}${unitLabel.value}` : `≤ ${trigger}${unitLabel.value}`
  return `Incohérent : le seuil de résolution doit être ${side} (sinon l'alerte ne se résout jamais).`
}

// ── Test results display ─────────────────────────────────────────────

const testResultColLabel = computed(() => {
  switch (props.form.metric) {
    case 'docker_container_state': return 'Container'
    case 'docker_compose_degraded_services': return 'Projet Compose'
    case 'proxmox_storage_percent': return 'Stockage'
    default: return props.form.source_type === 'proxmox' ? 'Portée' : 'Hôte'
  }
})

function formatTestValue(value: number): string {
  switch (props.form.metric) {
    case 'docker_container_state':
      if (value >= 2) return 'Crit (état critique)'
      if (value >= 1) return 'Warn (état dégradé)'
      return 'OK (running)'
    case 'docker_compose_degraded_services':
      return `${Math.round(value)} service${value !== 1 ? 's' : ''} dégradé${value !== 1 ? 's' : ''}`
    default:
      return `${value.toFixed(1)}${unitLabel.value}`
  }
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>

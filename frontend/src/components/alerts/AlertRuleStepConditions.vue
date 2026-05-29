<template>
  <div>
    <div class="row">
      <div
        v-if="form.metric !== 'heartbeat_timeout'"
        class="col-md-6 mb-3"
      >
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

    <div
      v-if="form.metric !== 'heartbeat_timeout'"
      class="row"
    >
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

    <div
      v-if="form.metric === 'heartbeat_timeout'"
      class="row"
    >
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

    <div
      v-if="form.metric !== 'heartbeat_timeout'"
      class="row"
    >
      <div class="col-md-6 mb-3">
        <label class="form-label">Désactivation warn (hystérésis)</label>
        <input
          v-model.number="form.threshold_clear_warn"
          type="number"
          step="0.1"
          class="form-control"
          placeholder="(Laisser vide pour auto)"
          :aria-describedby="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >
          Seuil pour résoudre l'alerte warn. Évite le fluttering.
        </small>
      </div>
      <div class="col-md-6 mb-3">
        <label class="form-label">Désactivation crit (hystérésis)</label>
        <input
          v-model.number="form.threshold_clear_crit"
          type="number"
          step="0.1"
          class="form-control"
          placeholder="(Laisser vide pour auto)"
          :aria-describedby="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >
          Seuil pour résoudre l'alerte crit. Évite le fluttering.
        </small>
      </div>
    </div>

    <div
      v-if="form.metric !== 'heartbeat_timeout'"
      class="mb-3"
    >
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
              <th>{{ form.source_type === 'proxmox' ? 'Portée' : 'Hôte' }}</th>
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
                <span v-if="result.has_data">{{ result.current_value.toFixed(1) }}{{ metricUnit }}</span>
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

defineProps<{
  form: AlertRuleFormData
  rule?: { id?: number | string } | null
  testResults?: TestResults | null
  hasNoDataResults?: boolean
  canDownloadTestLogs?: boolean
  downloadingLogs?: boolean
  metricUnit?: string
}>()

const emit = defineEmits<{ (e: 'download-logs'): void }>()

function formatDate(dateStr?: string): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>

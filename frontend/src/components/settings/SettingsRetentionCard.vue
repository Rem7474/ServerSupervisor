<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        Rétention des données
      </h3>
    </div>
    <div class="card-body">
      <div class="mb-3">
        <label class="form-label">Métriques (jours)</label>
        <input
          v-model.number="form.metricsRetentionDays"
          type="number"
          class="form-control"
          min="1"
          max="365"
          aria-describedby="metrics-retention-hint"
        >
        <div
          id="metrics-retention-hint"
          class="form-hint"
        >
          Politique de rétention TimescaleDB pour system_metrics et disk_metrics
        </div>
      </div>
      <div class="mb-0">
        <label class="form-label">Logs audit (jours)</label>
        <input
          v-model.number="form.auditRetentionDays"
          type="number"
          class="form-control"
          min="1"
          max="3650"
          aria-describedby="audit-retention-hint"
        >
        <div
          id="audit-retention-hint"
          class="form-hint"
        >
          Entrées d'audit plus anciennes que ce seuil sont supprimées
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        type="button"
        class="btn btn-primary"
        :disabled="savingRetention"
        @click="$emit('save')"
      >
        {{ savingRetention ? 'Enregistrement...' : 'Enregistrer' }}
      </button>
      <span
        v-if="retentionSaveMsg"
        :class="['ms-auto small', retentionSaveOk ? 'text-success' : 'text-danger']"
      >
        {{ retentionSaveMsg }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
interface RetentionForm {
  metricsRetentionDays: number
  auditRetentionDays: number
}

withDefaults(defineProps<{
  form: RetentionForm
  authIsAdmin?: boolean
  savingRetention?: boolean
  retentionSaveMsg?: string
  retentionSaveOk?: boolean
}>(), {
  authIsAdmin: false,
  savingRetention: false,
  retentionSaveMsg: '',
  retentionSaveOk: false,
})

defineEmits<{
  (e: 'save'): void
}>()
</script>


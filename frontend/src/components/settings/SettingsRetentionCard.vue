<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        Rétention des données
      </h3>
    </div>
    <div class="card-body">
      <div class="mb-3">
        <label class="form-label">Metriques (jours)</label>
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
          Metriques brutes et agregats plus anciens que ce seuil sont supprimes
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
          Entrees d'audit plus anciennes que ce seuil sont supprimees
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
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

<script setup>
defineProps({
  form: {
    type: Object,
    required: true,
  },
  authIsAdmin: {
    type: Boolean,
    default: false,
  },
  savingRetention: {
    type: Boolean,
    default: false,
  },
  retentionSaveMsg: {
    type: String,
    default: '',
  },
  retentionSaveOk: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['save'])
</script>


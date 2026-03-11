<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">Retention des donnees</h3>
    </div>
    <div class="card-body">
      <div class="mb-3">
        <label class="form-label">Metriques (jours)</label>
        <input type="number" class="form-control" v-model.number="form.metricsRetentionDays" min="1" max="365">
        <div class="form-hint">Metriques brutes et agregats plus anciens que ce seuil sont supprimes</div>
      </div>
      <div class="mb-0">
        <label class="form-label">Logs audit (jours)</label>
        <input type="number" class="form-control" v-model.number="form.auditRetentionDays" min="1" max="3650">
        <div class="form-hint">Entrees d'audit plus anciennes que ce seuil sont supprimees</div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        class="btn btn-primary"
        @click="$emit('save')"
        :disabled="savingRetention"
      >
        {{ savingRetention ? 'Enregistrement...' : 'Enregistrer' }}
      </button>
      <span v-if="retentionSaveMsg" :class="['ms-auto small', retentionSaveOk ? 'text-success' : 'text-danger']">
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

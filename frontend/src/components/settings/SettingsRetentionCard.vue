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
      <div class="mb-3">
        <label class="form-label">Logs audit — par défaut (jours)</label>
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
          Entrées d'audit plus anciennes que ce seuil sont supprimées — s'applique à toute
          catégorie sans valeur spécifique ci-dessous
        </div>
      </div>
      <div
        v-if="auditCategories.length > 0"
        class="mb-0"
      >
        <label class="form-label">Logs audit — par catégorie (jours, optionnel)</label>
        <div class="row g-2">
          <div
            v-for="cat in auditCategories"
            :key="cat.key"
            class="col-6 col-md-3"
          >
            <div class="input-group input-group-flat">
              <span class="input-group-text">{{ cat.label }}</span>
              <input
                type="number"
                class="form-control"
                min="1"
                max="3650"
                :placeholder="String(form.auditRetentionDays)"
                :value="form.auditRetentionDaysByCategory[cat.key] ?? ''"
                @input="onCategoryDaysInput(cat.key, ($event.target as HTMLInputElement).value)"
              >
            </div>
          </div>
        </div>
        <div class="form-hint">
          Laisser vide pour utiliser la valeur par défaut ci-dessus
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
  auditRetentionDaysByCategory: Record<string, number>
}

const props = withDefaults(defineProps<{
  form: RetentionForm
  auditCategories?: { key: string; label: string }[]
  authIsAdmin?: boolean
  savingRetention?: boolean
  retentionSaveMsg?: string
  retentionSaveOk?: boolean
}>(), {
  auditCategories: () => [],
  authIsAdmin: false,
  savingRetention: false,
  retentionSaveMsg: '',
  retentionSaveOk: false,
})

defineEmits<{
  (e: 'save'): void
}>()

// A blank input removes the category's override (falls back to the default
// above) rather than persisting 0/NaN — v-model.number can't express "no
// value" cleanly here since 0 is falsy but a real (invalid) retention value.
function onCategoryDaysInput(key: string, raw: string): void {
  if (raw === '') {
    delete props.form.auditRetentionDaysByCategory[key]
    return
  }
  const n = Number(raw)
  if (Number.isFinite(n) && n > 0) {
    props.form.auditRetentionDaysByCategory[key] = n
  }
}
</script>


<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">Maintenance</h3>
    </div>
    <div class="card-body">
      <div class="row g-3">
        <div class="col-md-6">
          <h4 class="text-sm mb-2">Nettoyage des metriques</h4>
          <p class="text-secondary small mb-3">
            Supprime les metriques brutes + agregats plus anciens que {{ settings.metricsRetentionDays }} jours
          </p>
          <button class="btn btn-warning btn-sm" @click="showCleanMetricsModal = true" :disabled="cleaningMetrics">
            {{ cleaningMetrics ? 'Nettoyage en cours...' : 'Lancer le nettoyage' }}
          </button>
          <div v-if="cleanMessage" :class="['alert alert-sm mt-2', cleanSuccess ? 'alert-success' : 'alert-danger']">
            {{ cleanMessage }}
          </div>
        </div>

        <div class="col-md-6">
          <h4 class="text-sm mb-2">Nettoyage des logs audit</h4>
          <p class="text-secondary small mb-3">
            Supprime les entrees audit plus anciennes que {{ settings.auditRetentionDays }} jours
          </p>
          <button class="btn btn-warning btn-sm" @click="showCleanAuditModal = true" :disabled="cleaningAuditLogs">
            {{ cleaningAuditLogs ? 'Nettoyage en cours...' : 'Lancer le nettoyage' }}
          </button>
          <div v-if="auditCleanMessage" :class="['alert alert-sm mt-2', auditCleanSuccess ? 'alert-success' : 'alert-danger']">
            {{ auditCleanMessage }}
          </div>
        </div>
      </div>
    </div>

    <div v-if="showCleanMetricsModal" class="modal modal-blur fade show" style="display: block; background: rgba(0,0,0,0.5);">
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <button @click="showCleanMetricsModal = false" type="button" class="btn-close" aria-label="Close"></button>
          <div class="modal-status bg-warning"></div>
          <div class="modal-body text-center py-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 9m-6 0a6 6 0 1 0 12 0a6 6 0 1 0 -12 0" /><path d="M12 7v5" /><path d="M12 13v.01" /></svg>
            <h3>Confirmer le nettoyage</h3>
            <div class="text-secondary mb-3">Les metriques plus anciennes que {{ settings.metricsRetentionDays }} jours seront supprimees definitivement.</div>
          </div>
          <div class="modal-footer">
            <div class="w-100 d-flex gap-2">
              <button @click="showCleanMetricsModal = false" type="button" class="btn btn-link link-secondary w-100">Annuler</button>
              <button @click="confirmCleanMetrics" type="button" class="btn btn-warning w-100">Continuer</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showCleanAuditModal" class="modal modal-blur fade show" style="display: block; background: rgba(0,0,0,0.5);">
      <div class="modal-dialog modal-sm modal-dialog-centered">
        <div class="modal-content">
          <button @click="showCleanAuditModal = false" type="button" class="btn-close" aria-label="Close"></button>
          <div class="modal-status bg-warning"></div>
          <div class="modal-body text-center py-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M12 9m-6 0a6 6 0 1 0 12 0a6 6 0 1 0 -12 0" /><path d="M12 7v5" /><path d="M12 13v.01" /></svg>
            <h3>Confirmer le nettoyage</h3>
            <div class="text-secondary mb-3">Les entrees audit plus anciennes que {{ settings.auditRetentionDays }} jours seront supprimees. Cette action est irreversible.</div>
          </div>
          <div class="modal-footer">
            <div class="w-100 d-flex gap-2">
              <button @click="showCleanAuditModal = false" type="button" class="btn btn-link link-secondary w-100">Annuler</button>
              <button @click="confirmCleanAudit" type="button" class="btn btn-warning w-100">Continuer</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

defineProps({
  settings: {
    type: Object,
    required: true,
  },
  cleaningMetrics: {
    type: Boolean,
    default: false,
  },
  cleanMessage: {
    type: String,
    default: '',
  },
  cleanSuccess: {
    type: Boolean,
    default: false,
  },
  cleaningAuditLogs: {
    type: Boolean,
    default: false,
  },
  auditCleanMessage: {
    type: String,
    default: '',
  },
  auditCleanSuccess: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['clean-metrics', 'clean-audit'])

const showCleanMetricsModal = ref(false)
const showCleanAuditModal = ref(false)

function confirmCleanMetrics() {
  showCleanMetricsModal.value = false
  emit('clean-metrics')
}

function confirmCleanAudit() {
  showCleanAuditModal.value = false
  emit('clean-audit')
}
</script>

<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">
        Maintenance
      </h3>
    </div>
    <div class="card-body">
      <div class="row g-3">
        <div class="col-md-6">
          <h4 class="text-sm mb-2">
            Nettoyage des métriques
          </h4>
          <p class="text-secondary small mb-3">
            Met à jour la politique de rétention TimescaleDB à {{ settings.metricsRetentionDays }} jours (system_metrics, disk_metrics)
          </p>
          <button
            type="button"
            class="btn btn-warning btn-sm"
            :disabled="cleaningMetrics"
            @click="confirmCleanMetrics"
          >
            {{ cleaningMetrics ? 'Nettoyage en cours...' : 'Lancer le nettoyage' }}
          </button>
          <div
            v-if="cleanMessage"
            :class="['alert alert-sm mt-2', cleanSuccess ? 'alert-success' : 'alert-danger']"
          >
            {{ cleanMessage }}
          </div>
        </div>

        <div class="col-md-6">
          <h4 class="text-sm mb-2">
            Nettoyage des logs audit
          </h4>
          <p class="text-secondary small mb-3">
            Supprime les entrées audit plus anciennes que {{ settings.auditRetentionDays }} jours
          </p>
          <button
            type="button"
            class="btn btn-warning btn-sm"
            :disabled="cleaningAuditLogs"
            @click="confirmCleanAudit"
          >
            {{ cleaningAuditLogs ? 'Nettoyage en cours...' : 'Lancer le nettoyage' }}
          </button>
          <div
            v-if="auditCleanMessage"
            :class="['alert alert-sm mt-2', auditCleanSuccess ? 'alert-success' : 'alert-danger']"
          >
            {{ auditCleanMessage }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useConfirmDialog } from '../../composables/useConfirmDialog'

interface Settings {
  metricsRetentionDays: number
  auditRetentionDays: number
}

const props = withDefaults(defineProps<{
  settings: Settings
  cleaningMetrics?: boolean
  cleanMessage?: string
  cleanSuccess?: boolean
  cleaningAuditLogs?: boolean
  auditCleanMessage?: string
  auditCleanSuccess?: boolean
}>(), {
  cleaningMetrics: false,
  cleanMessage: '',
  cleanSuccess: false,
  cleaningAuditLogs: false,
  auditCleanMessage: '',
  auditCleanSuccess: false,
})

const emit = defineEmits<{
  (e: 'clean-metrics'): void
  (e: 'clean-audit'): void
}>()

const dialog = useConfirmDialog()

async function confirmCleanMetrics(): Promise<void> {
  const confirmed = await dialog.confirm({
    title: 'Confirmer le nettoyage',
    message: `La politique de rétention TimescaleDB sera mise à jour à ${props.settings.metricsRetentionDays} jours. Le nettoyage sera appliqué automatiquement par TimescaleDB.`,
    variant: 'warning',
    okLabel: 'Continuer',
  })
  if (!confirmed) return
  emit('clean-metrics')
}

async function confirmCleanAudit(): Promise<void> {
  const confirmed = await dialog.confirm({
    title: 'Confirmer le nettoyage',
    message: `Les entrées audit plus anciennes que ${props.settings.auditRetentionDays} jours seront supprimées. Cette action est irréversible.`,
    variant: 'warning',
    okLabel: 'Continuer',
  })
  if (!confirmed) return
  emit('clean-audit')
}
</script>

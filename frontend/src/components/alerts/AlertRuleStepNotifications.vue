<template>
  <div>
    <div
      v-if="testResults"
      class="alert py-2 small mb-3"
      :class="testResults.any_fires ? 'alert-warning' : 'alert-success'"
    >
      <strong>Dernier test :</strong>
      {{ testResults.any_fires ? ' la règle déclencherait une alerte.' : ' la règle ne déclencherait pas d\'alerte.' }}
      <span class="text-secondary ms-1">({{ formatDate(testResults.evaluated_at) }})</span>
    </div>

    <div
      v-if="testError"
      class="alert alert-danger py-2 small mb-3"
      role="alert"
    >
      {{ testError }}
    </div>

    <div
      v-if="commandTriggerEnabled"
      class="mb-3"
    >
      <label class="form-label">Période de silence (secondes)</label>
      <input
        v-model.number="form.actions.cooldown"
        type="number"
        class="form-control"
        placeholder="3600"
        :aria-describedby="`cooldown-hint-${rule?.id || 'new'}`"
      >
      <small
        :id="`cooldown-hint-${rule?.id || 'new'}`"
        class="form-hint"
      >Temps minimum entre deux alertes successives pour cette règle</small>
    </div>

    <div class="mb-3">
      <label class="form-label">Canaux de notification</label>
      <div>
        <label class="form-check form-check-inline">
          <input
            v-model="channelSmtp"
            class="form-check-input"
            type="checkbox"
          >
          <span class="form-check-label">SMTP (Email)</span>
        </label>
        <label class="form-check form-check-inline">
          <input
            v-model="channelNtfy"
            class="form-check-input"
            type="checkbox"
          >
          <span class="form-check-label">Ntfy (Push)</span>
        </label>
        <label class="form-check form-check-inline">
          <input
            v-model="channelBrowser"
            class="form-check-input"
            type="checkbox"
          >
          <span class="form-check-label">Navigateur</span>
        </label>
      </div>
      <div
        v-if="channelBrowser"
        class="mt-2"
      >
        <div
          v-if="browserPermission === 'denied'"
          class="alert alert-warning py-2 small mb-0"
        >
          Notifications bloquées par le navigateur.
        </div>
        <div
          v-else-if="browserPermission === 'granted'"
          class="alert alert-success py-2 small mb-0"
        >
          Notifications navigateur autorisées.
        </div>
        <div
          v-else-if="browserPermission === 'unsupported'"
          class="alert alert-warning py-2 small mb-0"
        >
          Ce navigateur ne supporte pas les notifications.
        </div>
        <div
          v-else
          class="text-secondary small mt-1"
        >
          La permission sera demandée à l'enregistrement.
        </div>
      </div>
    </div>

    <div
      v-if="channelSmtp"
      class="mb-3"
    >
      <label class="form-label">Destinataire(s) email</label>
      <input
        v-model="form.actions.smtp_to"
        type="text"
        class="form-control"
        placeholder="admin@example.com, ops@example.com"
        aria-describedby="smtp-hint"
      >
      <small
        id="smtp-hint"
        class="form-hint"
      >Séparez plusieurs emails par des virgules</small>
    </div>

    <div
      v-if="channelNtfy"
      class="mb-3"
    >
      <label class="form-label">Topic ntfy</label>
      <input
        v-model="form.actions.ntfy_topic"
        type="text"
        class="form-control"
        placeholder="mon-serveur-alerts"
      >
    </div>

    <AlertRuleCommandTrigger
      v-model:enabled="commandTriggerEnabled"
      :model-value="form.actions.command_trigger || { module: 'processes', action: 'list', target: '' }"
      :docker-scope="form.source_type === 'docker' ? form.docker_scope : null"
      @update:model-value="form.actions.command_trigger = $event"
    />

    <div class="mb-3">
      <label class="form-check">
        <input
          v-model="form.enabled"
          class="form-check-input"
          type="checkbox"
        >
        <span class="form-check-label">Activer immédiatement</span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import AlertRuleCommandTrigger from './AlertRuleCommandTrigger.vue'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

interface TestResults {
  any_fires?: boolean
  evaluated_at?: string
}

defineProps<{
  form: AlertRuleFormData
  rule?: { id?: number | string } | null
  testResults?: TestResults | null
  testError?: string
  browserPermission?: string
}>()

const channelSmtp = defineModel<boolean>('channelSmtp', { default: false })
const channelNtfy = defineModel<boolean>('channelNtfy', { default: false })
const channelBrowser = defineModel<boolean>('channelBrowser', { default: false })
const commandTriggerEnabled = defineModel<boolean>('commandTriggerEnabled', { default: false })

function formatDate(dateStr?: string): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>

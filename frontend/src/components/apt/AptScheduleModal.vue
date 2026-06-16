<template>
  <div
    v-if="host"
    class="modal modal-blur show d-block modal-overlay"
    tabindex="-1"
    @click.self="$emit('close')"
  >
    <div class="modal-dialog modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <div>
            <h5 class="modal-title">
              Planifier une commande APT
            </h5>
            <div class="text-muted small mt-1">
              {{ hostLabel }}
            </div>
          </div>
          <button
            type="button"
            class="btn-close"
            @click="$emit('close')"
          />
        </div>
        <div class="modal-body">
          <div class="mb-3">
            <label class="form-label">Nom de la tâche</label>
            <input
              v-model="form.name"
              type="text"
              class="form-control"
              placeholder="ex: apt upgrade hebdo"
            >
          </div>
          <div class="mb-3">
            <label class="form-label">Commande</label>
            <select
              v-model="form.action"
              class="form-select"
            >
              <option value="update">
                apt update
              </option>
              <option value="upgrade">
                apt upgrade
              </option>
              <option value="dist-upgrade">
                apt dist-upgrade
              </option>
            </select>
          </div>
          <div class="mb-3">
            <label class="form-check form-switch">
              <input
                v-model="form.manualOnly"
                type="checkbox"
                class="form-check-input"
              >
              <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
            </label>
          </div>
          <div
            v-if="!form.manualOnly"
            class="mb-3"
          >
            <CronBuilder v-model="form.cron_expression" />
          </div>
          <div
            v-if="!form.manualOnly"
            class="form-check form-switch mb-2"
          >
            <input
              id="schedEnabled"
              v-model="form.enabled"
              type="checkbox"
              class="form-check-input"
            >
            <label
              class="form-check-label"
              for="schedEnabled"
            >Activée</label>
          </div>
          <div
            v-if="form.error"
            class="alert alert-danger py-2"
          >
            {{ form.error }}
          </div>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary"
            @click="$emit('close')"
          >
            Annuler
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="form.saving"
            @click="saveSchedule"
          >
            <span
              v-if="form.saving"
              class="spinner-border spinner-border-sm me-1"
            />
            Créer la tâche
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, watch } from 'vue'
import apiClient from '../../api'
import CronBuilder from '../CronBuilder.vue'
import { MANUAL_SENTINEL } from '../../utils/cron'

const props = defineProps<{
  host: any | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created'): void
}>()

const form = reactive({
  name: '',
  action: 'update',
  cron_expression: '0 3 * * 0',
  manualOnly: false,
  enabled: true,
  saving: false,
  error: '',
})

const hostLabel = computed(() => {
  const host = props.host
  if (!host) return ''
  return host.name && host.hostname && host.name !== host.hostname
    ? `${host.name} (${host.hostname})`
    : (host.name || host.hostname)
})

// Reset the form each time the modal is opened for a host.
watch(() => props.host, (host) => {
  if (!host) return
  form.name = ''
  form.action = 'update'
  form.cron_expression = '0 3 * * 0'
  form.manualOnly = false
  form.enabled = true
  form.saving = false
  form.error = ''
})

async function saveSchedule(): Promise<void> {
  if (!props.host) return
  form.error = ''
  form.saving = true
  const cronExpr = form.manualOnly ? MANUAL_SENTINEL : form.cron_expression
  try {
    await apiClient.createScheduledTask(props.host.id, {
      name: form.name || `apt ${form.action}`,
      module: 'apt',
      action: form.action,
      target: '',
      payload: '{}',
      cron_expression: cronExpr,
      enabled: form.manualOnly ? false : form.enabled,
    })
    emit('created')
    emit('close')
  } catch (e: any) {
    form.error = e?.response?.data?.error || 'Erreur lors de la création'
  } finally {
    form.saving = false
  }
}
</script>

<style scoped>
.modal-overlay { background: rgba(0, 0, 0, .5); z-index: 1050; }
</style>

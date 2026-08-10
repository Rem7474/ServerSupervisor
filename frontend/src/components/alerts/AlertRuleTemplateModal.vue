<template>
  <div v-if="visible">
    <div
      ref="modalRef"
      class="modal modal-blur fade show"
      style="display: block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ template ? 'Modifier le modèle' : 'Nouveau modèle de règle' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              aria-label="Fermer"
              @click="$emit('close')"
            />
          </div>
          <form @submit.prevent="submit">
            <div class="modal-body">
              <div class="mb-3">
                <label class="form-label required">Nom</label>
                <input
                  v-model="form.name"
                  type="text"
                  class="form-control"
                  placeholder="CPU élevé"
                  required
                >
              </div>
              <div class="row g-3">
                <div class="col-12 col-md-6">
                  <label class="form-label required">Métrique</label>
                  <select
                    v-model="form.metric"
                    class="form-select"
                    required
                  >
                    <option
                      v-for="m in agentMetrics"
                      :key="m.metric"
                      :value="m.metric"
                    >
                      {{ m.label }}
                    </option>
                  </select>
                  <div class="form-hint">
                    Uniquement les métriques agent — Docker/Proxmox ne s'appliquent pas hôte par hôte.
                  </div>
                </div>
                <div class="col-12 col-md-6">
                  <label class="form-label required">Opérateur</label>
                  <select
                    v-model="form.operator"
                    class="form-select"
                    required
                  >
                    <option value=">">
                      &gt;
                    </option>
                    <option value=">=">
                      &gt;=
                    </option>
                    <option value="<">
                      &lt;
                    </option>
                    <option value="<=">
                      &lt;=
                    </option>
                  </select>
                </div>
                <div class="col-6 col-md-3">
                  <label class="form-label required">Seuil avertissement</label>
                  <input
                    v-model.number="form.threshold_warn"
                    type="number"
                    step="any"
                    class="form-control"
                    required
                  >
                </div>
                <div class="col-6 col-md-3">
                  <label class="form-label required">Seuil critique</label>
                  <input
                    v-model.number="form.threshold_crit"
                    type="number"
                    step="any"
                    class="form-control"
                    required
                  >
                </div>
                <div class="col-6 col-md-3">
                  <label class="form-label">Durée (secondes)</label>
                  <input
                    v-model.number="form.duration"
                    type="number"
                    min="0"
                    class="form-control"
                  >
                </div>
                <div class="col-6 col-md-3">
                  <label class="form-label">Silence (secondes)</label>
                  <input
                    v-model.number="form.actions.cooldown"
                    type="number"
                    min="0"
                    class="form-control"
                  >
                </div>
                <div class="col-12">
                  <label class="form-label">Escalade si non acquittée (minutes)</label>
                  <input
                    v-model.number="form.actions.escalate_after_minutes"
                    type="number"
                    min="0"
                    class="form-control"
                    placeholder="0"
                  >
                  <div class="form-hint">
                    0 = désactivée.
                  </div>
                </div>
              </div>
              <div class="mb-0 mt-3">
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
              </div>
              <div
                v-if="error"
                class="alert alert-danger mt-3 mb-0"
              >
                {{ error }}
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn btn-outline-secondary"
                @click="$emit('close')"
              >
                Annuler
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="saving"
              >
                <span
                  v-if="saving"
                  class="spinner-border spinner-border-sm me-2"
                />
                {{ template ? 'Enregistrer' : 'Créer' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
    <div
      v-if="visible"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useModalChrome } from '../../composables/useModalChrome'
import type { AlertRuleTemplate, AlertRuleTemplateRequest, AlertMetricCapability } from '../../types/generated'

const props = withDefaults(defineProps<{
  visible?: boolean
  template?: AlertRuleTemplate | null
  agentMetrics?: AlertMetricCapability[]
  saving?: boolean
  error?: string
}>(), {
  visible: false,
  template: null,
  agentMetrics: () => [],
  saving: false,
  error: '',
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: AlertRuleTemplateRequest): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => props.visible, { onClose: () => emit('close') })

const channelSmtp = ref(false)
const channelNtfy = ref(false)
const channelBrowser = ref(false)

function defaultForm(): AlertRuleTemplateRequest {
  return {
    name: '', metric: props.agentMetrics[0]?.metric || 'cpu', operator: '>',
    threshold_warn: 70, threshold_crit: 90, duration: 0,
    actions: { channels: [], cooldown: 0, escalate_after_minutes: 0 },
  }
}

const form = reactive<AlertRuleTemplateRequest>(defaultForm())

watch(() => props.template, (t) => {
  if (t) {
    Object.assign(form, {
      name: t.name, metric: t.metric, operator: t.operator,
      threshold_warn: t.threshold_warn, threshold_crit: t.threshold_crit,
      threshold_clear_warn: t.threshold_clear_warn, threshold_clear_crit: t.threshold_clear_crit,
      duration: t.duration_seconds,
      actions: { ...t.actions, channels: [...(t.actions.channels || [])] },
    })
  } else {
    Object.assign(form, defaultForm())
  }
  channelSmtp.value = form.actions.channels?.includes('smtp') || false
  channelNtfy.value = form.actions.channels?.includes('ntfy') || false
  channelBrowser.value = form.actions.channels?.includes('browser') || false
}, { immediate: true })

function submit(): void {
  const channels: string[] = []
  if (channelSmtp.value) channels.push('smtp')
  if (channelNtfy.value) channels.push('ntfy')
  if (channelBrowser.value) channels.push('browser')
  emit('submit', { ...form, actions: { ...form.actions, channels } })
}
</script>

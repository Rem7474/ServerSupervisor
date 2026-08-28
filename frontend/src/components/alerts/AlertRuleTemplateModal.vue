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
                <label
                  for="alert-template-name"
                  class="form-label required"
                >Nom</label>
                <input
                  id="alert-template-name"
                  v-model="form.name"
                  type="text"
                  class="form-control"
                  placeholder="CPU élevé"
                  required
                >
              </div>
              <div class="row g-3">
                <div class="col-12 col-md-6">
                  <label
                    for="alert-template-metric"
                    class="form-label required"
                  >Métrique</label>
                  <select
                    id="alert-template-metric"
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
                  <label
                    for="alert-template-operator"
                    class="form-label required"
                  >Opérateur</label>
                  <select
                    id="alert-template-operator"
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
                <div
                  v-if="form.metric === 'bandwidth_vs_rolling_avg'"
                  class="col-12"
                >
                  <label
                    for="alert-template-baseline-window"
                    class="form-label required"
                  >Fenêtre de moyenne glissante</label>
                  <select
                    id="alert-template-baseline-window"
                    v-model.number="form.baseline_window_seconds"
                    class="form-select"
                  >
                    <option :value="3600">
                      1 heure
                    </option>
                    <option :value="21600">
                      6 heures
                    </option>
                    <option :value="86400">
                      24 heures
                    </option>
                  </select>
                </div>
                <div class="col-6 col-md-3">
                  <label
                    for="alert-template-threshold-warn"
                    class="form-label required"
                  >
                    {{ form.metric === 'bandwidth_vs_rolling_avg' ? 'Seuil avertissement (% moyenne)' : 'Seuil avertissement' }}
                  </label>
                  <input
                    id="alert-template-threshold-warn"
                    v-model.number="form.threshold_warn"
                    type="number"
                    step="any"
                    class="form-control"
                    required
                  >
                </div>
                <div class="col-6 col-md-3">
                  <label
                    for="alert-template-threshold-crit"
                    class="form-label required"
                  >
                    {{ form.metric === 'bandwidth_vs_rolling_avg' ? 'Seuil critique (% moyenne)' : 'Seuil critique' }}
                  </label>
                  <input
                    id="alert-template-threshold-crit"
                    v-model.number="form.threshold_crit"
                    type="number"
                    step="any"
                    class="form-control"
                    required
                  >
                </div>
                <div
                  v-if="form.metric !== 'bandwidth_vs_rolling_avg'"
                  class="col-6 col-md-3"
                >
                  <label
                    for="alert-template-duration"
                    class="form-label"
                  >Durée (secondes)</label>
                  <input
                    id="alert-template-duration"
                    v-model.number="form.duration"
                    type="number"
                    min="0"
                    class="form-control"
                  >
                </div>
                <div class="col-6 col-md-3">
                  <label
                    for="alert-template-cooldown"
                    class="form-label"
                  >Silence (secondes)</label>
                  <input
                    id="alert-template-cooldown"
                    v-model.number="form.actions.cooldown"
                    type="number"
                    min="0"
                    class="form-control"
                  >
                </div>
                <div class="col-12">
                  <label
                    for="alert-template-escalate-after-minutes"
                    class="form-label"
                  >Escalade si non acquittée (minutes)</label>
                  <input
                    id="alert-template-escalate-after-minutes"
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
                <div class="form-label">
                  Canaux de notification
                </div>
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
      baseline_window_seconds: t.baseline_window_seconds ?? (t.metric === 'bandwidth_vs_rolling_avg' ? 3600 : undefined),
      actions: { ...t.actions, channels: [...(t.actions.channels || [])] },
    })
  } else {
    Object.assign(form, defaultForm())
  }
  channelSmtp.value = form.actions.channels?.includes('smtp') || false
  channelNtfy.value = form.actions.channels?.includes('ntfy') || false
  channelBrowser.value = form.actions.channels?.includes('browser') || false
}, { immediate: true })

// bandwidth_vs_rolling_avg is the only templatable metric that understands
// baseline_window_seconds — default it in when selected, clear it out for
// every other metric so a stale value never gets submitted (server-side
// validateBaselineWindow rejects a non-nil value on any other metric).
watch(() => form.metric, (metric) => {
  if (metric === 'bandwidth_vs_rolling_avg') {
    if (!form.baseline_window_seconds) form.baseline_window_seconds = 3600
  } else {
    form.baseline_window_seconds = undefined
  }
})

function submit(): void {
  const channels: string[] = []
  if (channelSmtp.value) channels.push('smtp')
  if (channelNtfy.value) channels.push('ntfy')
  if (channelBrowser.value) channels.push('browser')
  emit('submit', { ...form, actions: { ...form.actions, channels } })
}
</script>

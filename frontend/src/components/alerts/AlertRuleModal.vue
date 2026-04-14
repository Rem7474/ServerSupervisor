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
              {{ rule ? 'Modifier l\'alerte' : 'Nouvelle alerte' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              @click="close"
            />
          </div>
          <div class="modal-body">
            <div class="alert-steps mb-4">
              <div
                class="step-chip"
                :class="{ active: step === 1, done: step > 1 }"
              >
                <span class="step-chip-index">1</span>
                <span>Quoi surveiller</span>
              </div>
              <div
                class="step-chip"
                :class="{ active: step === 2, done: step > 2 }"
              >
                <span class="step-chip-index">2</span>
                <span>Conditions</span>
              </div>
              <div
                class="step-chip"
                :class="{ active: step === 3 }"
              >
                <span class="step-chip-index">3</span>
                <span>Notification</span>
              </div>
            </div>

            <div v-if="step === 1">
              <div class="mb-3">
                <label class="form-label required">Nom</label>
                <input
                  v-model="form.name"
                  type="text"
                  class="form-control"
                  placeholder="Ex: CPU élevé sur serveur web"
                >
              </div>

              <div class="mb-3">
                <label class="form-label required">Source des donnees</label>
                <div
                  class="btn-group w-100"
                  role="group"
                  aria-label="Source type"
                >
                  <button
                    type="button"
                    class="btn"
                    :class="form.source_type === 'agent' ? 'btn-primary' : 'btn-outline-primary'"
                    @click="setSourceType('agent')"
                  >
                    Agent
                  </button>
                  <button
                    type="button"
                    class="btn"
                    :class="form.source_type === 'proxmox' ? 'btn-primary' : 'btn-outline-primary'"
                    @click="setSourceType('proxmox')"
                  >
                    Proxmox
                  </button>
                </div>
              </div>

              <div
                v-if="form.source_type === 'agent'"
                class="mb-3"
              >
                <label class="form-label">Hôte cible</label>
                <select
                  v-model="form.host_id"
                  class="form-select"
                  :disabled="!metricSupportsHostFilter"
                >
                  <option :value="null">
                    Tous les hôtes
                  </option>
                  <option
                    v-for="host in hosts"
                    :key="host.id"
                    :value="host.id"
                  >
                    {{ host.name }}
                  </option>
                </select>
                <small
                  v-if="!metricSupportsHostFilter"
                  :id="`host-filter-hint-${rule?.id || 'new'}`"
                  class="form-hint"
                >Cette metrique est globale et n'est pas liee a un hote.</small>
              </div>


              <div class="mb-2 fw-semibold">
                Choisissez une métrique à surveiller
              </div>
              <div
                v-if="capabilitiesLoading"
                class="alert alert-info py-2 small mb-2"
              >
                Chargement des metriques...
              </div>
              <div
                v-else-if="capabilitiesError"
                class="alert alert-warning py-2 small mb-2"
              >
                {{ capabilitiesError }}
              </div>
              <div
                v-if="form.host_id && hostMetricsLoading"
                class="alert alert-info py-2 small mb-2"
              >
                Chargement des metriques pour cet hote...
              </div>
              <div
                v-else-if="form.host_id && hostMetricsError"
                class="alert alert-warning py-2 small mb-2"
              >
                {{ hostMetricsError }}
              </div>
              <div
                v-else-if="form.host_id && hostMetrics?.metrics && hostMetrics.metrics.length < (capabilities?.metrics?.length || 0)"
                class="alert alert-info py-2 small mb-2"
              >
                ℹ️ Cet hote dispose de {{ hostMetrics.metrics.length }} metrique(s): certains collecteurs peuvent ne pas etre actifs.
              </div>
              <div class="metric-grid">
                <button
                  v-for="metric in metricCards"
                  :key="metric.value"
                  type="button"
                  class="metric-card"
                  :class="{ selected: form.metric === metric.value }"
                  @click="selectMetric(metric.value)"
                >
                  <span class="metric-icon">{{ metric.icon }}</span>
                  <span class="metric-label">{{ metric.label }}</span>
                </button>
              </div>
              <div
                v-if="isProxmoxMetric(form.metric)"
                class="row g-2 mt-2"
              >
                <div class="col-md-4">
                  <label class="form-label">Scope Proxmox</label>
                  <select
                    v-model="form.proxmox_scope.scope_mode"
                    class="form-select"
                  >
                    <option value="global">
                      Global
                    </option>
                    <option
                      v-if="!metricAllowsGuestScope"
                      value="connection"
                    >
                      Connexion
                    </option>
                    <option
                      v-if="!metricAllowsGuestScope"
                      value="node"
                    >
                      Noeud
                    </option>
                    <option
                      v-if="metricAllowsGuestScope"
                      value="guest"
                    >
                      VM/LXC
                    </option>
                    <option
                      v-if="metricAllowsStorageScope"
                      value="storage"
                    >
                      Stockage
                    </option>
                    <option
                      v-if="metricAllowsDiskScope"
                      value="disk"
                    >
                      Disque physique
                    </option>
                  </select>
                </div>
                <div
                  v-if="!metricAllowsGuestScope && form.proxmox_scope.scope_mode === 'connection'"
                  class="col-md-8"
                >
                  <label class="form-label">Connexion</label>
                  <select
                    v-model="form.proxmox_scope.connection_id"
                    class="form-select"
                  >
                    <option value="">
                      Selectionner...
                    </option>
                    <option
                      v-for="opt in proxmoxConnections"
                      :key="opt.id"
                      :value="opt.id"
                    >
                      {{ opt.label }}
                    </option>
                  </select>
                </div>
                <div
                  v-if="!metricAllowsGuestScope && form.proxmox_scope.scope_mode === 'node'"
                  class="col-md-8"
                >
                  <label class="form-label">Noeud</label>
                  <select
                    v-model="form.proxmox_scope.node_id"
                    class="form-select"
                  >
                    <option value="">
                      Selectionner...
                    </option>
                    <option
                      v-for="opt in proxmoxNodes"
                      :key="opt.id"
                      :value="opt.id"
                    >
                      {{ opt.label }}
                    </option>
                  </select>
                </div>
                <div
                  v-if="metricAllowsGuestScope && form.proxmox_scope.scope_mode === 'guest'"
                  class="col-md-8"
                >
                  <label class="form-label">VM/LXC</label>
                  <select
                    v-model="form.proxmox_scope.guest_id"
                    class="form-select"
                  >
                    <option value="">
                      Selectionner...
                    </option>
                    <option
                      v-for="opt in proxmoxGuests"
                      :key="opt.id"
                      :value="opt.id"
                    >
                      {{ opt.label }}
                    </option>
                  </select>
                </div>
                <div
                  v-if="metricAllowsStorageScope && form.proxmox_scope.scope_mode === 'storage'"
                  class="col-md-8"
                >
                  <label class="form-label">Stockage</label>
                  <select
                    v-model="form.proxmox_scope.storage_id"
                    class="form-select"
                  >
                    <option value="">
                      Selectionner...
                    </option>
                    <option
                      v-for="opt in proxmoxStorages"
                      :key="opt.id"
                      :value="opt.id"
                    >
                      {{ opt.label }}
                    </option>
                  </select>
                </div>
                <div
                  v-if="metricAllowsDiskScope && form.proxmox_scope.scope_mode === 'disk'"
                  class="col-md-8"
                >
                  <label class="form-label">Disque physique</label>
                  <select
                    v-model="form.proxmox_scope.disk_id"
                    class="form-select"
                  >
                    <option value="">
                      Selectionner...
                    </option>
                    <option
                      v-for="opt in proxmoxDisks"
                      :key="opt.id"
                      :value="opt.id"
                    >
                      {{ opt.label }}
                    </option>
                  </select>
                </div>
                <div class="col-12">
                  <small
                    :id="`proxmox-scope-hint-${rule?.id || 'new'}`"
                    class="form-hint d-block"
                  >
                    Connexion = toute l'instance Proxmox liée. Nœud = un hôte Proxmox précis à l'intérieur de cette connexion.
                  </small>
                </div>
              </div>
              <div
                v-if="form.metric === 'proxmox_storage_percent'"
                class="text-secondary small mt-2"
              >
                Cette métrique est globale Proxmox: elle surveille le stockage le plus rempli parmi les stockages actifs.
              </div>
              <div
                v-else-if="form.metric === 'disk_smart_status'"
                class="text-secondary small mt-2"
              >
                Utilisez typiquement un seuil > 0.5 pour déclencher quand au moins un disque est en etat SMART FAILED.
              </div>
            </div>

            <div v-if="step === 2">
              <div class="row">
                <div
                  v-if="form.metric !== 'heartbeat_timeout'"
                  class="col-md-6 mb-3"
                >
                  <label class="form-label required">Opérateur</label>
                  <select
                    v-model="form.operator"
                    class="form-select"
                  >
                    <option value=">">
                      Supérieur à (>)
                    </option>
                    <option value=">=">
                      Supérieur ou égal (>=)
                    </option>
                    <option value="<">
                      Inférieur à (&lt;)
                    </option>
                    <option value="<=">
                      Inférieur ou égal (&lt;=)
                    </option>
                  </select>
                </div>
              </div>

              <div v-if="form.metric !== 'heartbeat_timeout'" class="row">
                <div class="col-md-6 mb-3">
                  <label class="form-label required">Seuil d'avertissement (warn)</label>
                  <input
                    v-model.number="form.threshold_warn"
                    type="number"
                    step="0.1"
                    class="form-control"
                    placeholder="70"
                  >
                  <small class="form-hint">Déclenche une alerte de niveau avertissement.</small>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label required">Seuil critique (crit)</label>
                  <input
                    v-model.number="form.threshold_crit"
                    type="number"
                    step="0.1"
                    class="form-control"
                    placeholder="85"
                  >
                  <small class="form-hint">Déclenche une alerte de niveau critique.</small>
                </div>
              </div>

              <div v-if="form.metric === 'heartbeat_timeout'" class="row">
                <div class="col-md-12 mb-3">
                  <label class="form-label required">Silence maximum (secondes)</label>
                  <input
                    v-model.number="form.threshold_crit"
                    type="number"
                    step="60"
                    class="form-control"
                    placeholder="300"
                    :aria-describedby="`heartbeat-hint-${rule?.id || 'new'}`"
                  >
                  <small
                    :id="`heartbeat-hint-${rule?.id || 'new'}`"
                    class="form-hint"
                  >
                    Durée en secondes sans rapport avant alerte.
                  </small>
                </div>
              </div>

              <div
                v-if="form.metric !== 'heartbeat_timeout'"
                class="row">
                <div class="col-md-6 mb-3">
                  <label class="form-label">Désactivation warn (hystérésis)</label>
                  <input
                    v-model.number="form.threshold_clear_warn"
                    type="number"
                    step="0.1"
                    class="form-control"
                    placeholder="(Laisser vide pour auto)"
                    :aria-describedby="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
                  >
                  <small
                    :id="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
                    class="form-hint"
                  >
                    Seuil pour résoudre l'alerte warn. Évite le fluttering.
                  </small>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">Désactivation crit (hystérésis)</label>
                  <input
                    v-model.number="form.threshold_clear_crit"
                    type="number"
                    step="0.1"
                    class="form-control"
                    placeholder="(Laisser vide pour auto)"
                    :aria-describedby="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
                  >
                  <small
                    :id="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
                    class="form-hint"
                  >
                    Seuil pour résoudre l'alerte crit. Évite le fluttering.
                  </small>
                </div>
              </div>

              <div
                v-if="form.metric !== 'heartbeat_timeout'"
                class="mb-3"
              >
                <label class="form-label">Durée (secondes)</label>
                <input
                  v-model.number="form.duration"
                  type="number"
                  class="form-control"
                  placeholder="300"
                  :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
                >
                <small
                  :id="`duration-hint-${rule?.id || 'new'}`"
                  class="form-hint"
                >Le seuil doit être dépassé pendant cette durée avant de déclencher l'alerte.</small>
                <small
                  v-if="Number.isFinite(Number(form.duration)) && form.duration > 0 && form.duration < 60"
                  :id="`duration-warn-${rule?.id || 'new'}`"
                  class="form-hint text-warning d-block mt-1"
                >
                  Si l'agent reporte toutes les 60s, une durée inférieure peut empêcher le déclenchement.
                </small>
              </div>

              <div
                v-if="testResults"
                class="mt-3"
              >
                <div
                  v-if="hasNoDataResults"
                  class="alert alert-warning py-2 small mb-2"
                >
                  <strong>Aucune donnée disponible</strong> pour un ou plusieurs hôtes.
                </div>
                <div class="d-flex align-items-center justify-content-between mb-2">
                  <div class="fw-bold">
                    Résultat du test
                    <span
                      v-if="testResults.any_fires"
                      class="badge bg-danger-lt text-danger ms-2"
                    >Declencherait une alerte</span>
                    <span
                      v-else
                      class="badge bg-success-lt text-success ms-2"
                    >Aucune alerte déclenchée</span>
                  </div>
                  <span class="text-secondary small">{{ formatDate(testResults.evaluated_at) }}</span>
                </div>
                <div class="table-responsive">
                  <table class="table table-sm table-vcenter card-table">
                    <thead>
                      <tr>
                        <th>{{ form.source_type === 'proxmox' ? 'Portée' : 'Hôte' }}</th>
                        <th>Valeur actuelle</th>
                        <th>Résultat</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-if="!testResults.results?.length">
                        <td
                          colspan="3"
                          class="text-center text-secondary"
                        >
                          Aucun hôte concerné
                        </td>
                      </tr>
                      <tr
                        v-for="result in testResults.results"
                        :key="result.host_id"
                      >
                        <td class="fw-medium">
                          {{ result.host_name }}
                        </td>
                        <td>
                          <span v-if="result.has_data">{{ result.current_value.toFixed(1) }}{{ getMetricUnit(form.metric) }}</span>
                          <span
                            v-else
                            class="text-secondary"
                          >Pas de données</span>
                        </td>
                        <td>
                          <span
                            v-if="result.would_fire"
                            class="badge bg-danger-lt text-danger"
                          >Alerte</span>
                          <span
                            v-else
                            class="badge bg-success-lt text-success"
                          >OK</span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>

            <div v-if="step === 3">
              <div
                v-if="testResults"
                class="alert py-2 small mb-3"
                :class="testResults.any_fires ? 'alert-warning' : 'alert-success'"
              >
                <strong>Dernier test:</strong>
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
                >Temps minimum entre deux alertes successives pour cette regle</small>
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
                :model-value="form.actions.command_trigger"
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
          </div>
          <div
            v-if="error"
            class="alert alert-danger mx-3 mb-0 mt-0 py-2 small"
            role="alert"
          >
            {{ error }}
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-link"
              @click="close"
            >
              Annuler
            </button>
            <button
              v-if="step > 1"
              type="button"
              class="btn btn-outline-secondary"
              :disabled="saving"
              @click="step -= 1"
            >
              ← Précédent
            </button>
            <button
              v-if="step < 3"
              type="button"
              class="btn btn-primary"
              :disabled="saving || !canProceedStep"
              @click="goNextStep"
            >
              Suivant →
            </button>
            <button
              v-if="step === 3"
              type="button"
              class="btn btn-outline-secondary"
              :disabled="testing || saving"
              @click="testAlert"
            >
              <span
                v-if="testing"
                class="spinner-border spinner-border-sm me-2"
              />
              <svg
                v-else
                class="icon me-1"
                width="16"
                height="16"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              {{ testing ? 'Test en cours...' : 'Tester' }}
            </button>
            <button
              v-if="step === 3"
              type="button"
              class="btn btn-primary"
              :disabled="saving"
              @click="submit"
            >
              <span
                v-if="saving"
                class="spinner-border spinner-border-sm me-2"
              />
              {{ rule ? 'Mettre à jour' : 'Créer' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div
      v-if="visible"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import apiClient from '../../api'
import AlertRuleCommandTrigger from './AlertRuleCommandTrigger.vue'
import { useAlertRuleForm } from '../../composables/useAlertRuleForm'
import { useModalFocusTrap } from '../../composables/useModalFocusTrap'
import { ALERT_METRIC_ORDER, getAlertMetricMeta } from '../../utils/alertMetrics'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false,
  },
  rule: {
    type: Object,
    default: null,
  },
  hosts: {
    type: Array,
    default: () => [],
  },
  capabilities: {
    type: Object,
    default: null,
  },
  capabilitiesLoading: {
    type: Boolean,
    default: false,
  },
  capabilitiesError: {
    type: String,
    default: '',
  },
  saving: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'submit'])

const modalRef = ref<HTMLElement | null>(null)
useModalFocusTrap(modalRef)

const browserPermission = ref(typeof Notification !== 'undefined' ? Notification.permission : 'unsupported')
const step = ref(1)
const hostMetrics = ref(null)
const hostMetricsLoading = ref(false)
const hostMetricsError = ref('')

const metricCards = computed(() => {
  const metricSource = form.value.source_type || 'agent'

  // If a specific host is selected, use that host's filtered metrics
  if (metricSource === 'agent' && form.value.host_id && hostMetrics.value?.metrics) {
    return hostMetrics.value.metrics.map((metric) => ({
      value: metric.metric,
      label: metric.label,
      icon: metric.icon || getAlertMetricMeta(metric.metric).icon,
    }))
  }

  // Otherwise, use global capabilities (all hosts)
  const fromCapabilities = props.capabilities?.metrics
  if (Array.isArray(fromCapabilities) && fromCapabilities.length > 0) {
    return fromCapabilities
      .filter((metric) => metricSource === 'proxmox' ? isProxmoxMetric(metric.metric) : !isProxmoxMetric(metric.metric))
      .map((metric) => ({
      value: metric.metric,
      label: metric.label,
      icon: metric.icon || getAlertMetricMeta(metric.metric).icon,
      }))
  }

  return ALERT_METRIC_ORDER
    .filter((metric) => metricSource === 'proxmox' ? isProxmoxMetric(metric) : !isProxmoxMetric(metric))
    .map((metric) => ({ value: metric, label: getAlertMetricMeta(metric).label, icon: getAlertMetricMeta(metric).icon }))
})

const proxmoxConnections = computed(() => props.capabilities?.proxmox_scope?.connections || [])
const proxmoxNodes = computed(() => props.capabilities?.proxmox_scope?.nodes || [])
const proxmoxStorages = computed(() => props.capabilities?.proxmox_scope?.storages || [])
const proxmoxGuests = computed(() => props.capabilities?.proxmox_scope?.guests || [])
const proxmoxDisks = computed(() => props.capabilities?.proxmox_scope?.disks || [])

const metricMetaByKey = computed(() => {
  const items = props.capabilities?.metrics || []
  return Object.fromEntries(items.map((item) => [item.metric, item]))
})

const isProxmoxMetric = (metric) => getAlertMetricMeta(metric).category === 'proxmox'
const metricAllowsStorageScope = computed(() => form.value.metric === 'proxmox_storage_percent')
const metricAllowsGuestScope = computed(() => form.value.metric === 'proxmox_guest_cpu_percent' || form.value.metric === 'proxmox_guest_memory_percent')
const metricAllowsDiskScope = computed(() => form.value.metric === 'proxmox_disk_failed_count' || form.value.metric === 'proxmox_disk_min_wearout_percent')

const metricSupportsHostFilter = computed(() => {
  const supports = metricMetaByKey.value?.[form.value.metric]?.supports_host_filter
  return supports !== false
})
const {
  form,
  channelSmtp,
  channelNtfy,
  channelBrowser,
  commandTriggerEnabled,
  hydrateFormFromRule,
  onMetricChange,
  buildPayload,
} = useAlertRuleForm()

const testing = ref(false)
const testResults = ref(null)
const testError = ref('')

const hasNoDataResults = computed(() => testResults.value?.results?.some((result) => !result.has_data) || false)

const canProceedStep = computed(() => {
  if (step.value === 1) {
    const hasBase = !!form.value.metric && !!form.value.name?.trim()
    if (!hasBase) return false
    // "Tous les hôtes" is a valid selection for agent-based rules.
    if (form.value.source_type === 'agent') return true

    const scope = form.value.proxmox_scope || { scope_mode: 'global' }
    if (scope.scope_mode === 'connection') return !!scope.connection_id
    if (scope.scope_mode === 'node') return !!scope.node_id
    if (scope.scope_mode === 'guest') return !!scope.guest_id
    if (scope.scope_mode === 'storage') return !!scope.storage_id
    if (scope.scope_mode === 'disk') return !!scope.disk_id
    return true
  }
  if (step.value === 2) {
    if (form.value.metric === 'heartbeat_timeout') {
      return Number.isFinite(Number(form.value.threshold_crit))
    }

    return Number.isFinite(Number(form.value.threshold_warn)) && Number.isFinite(Number(form.value.threshold_crit))
  }
  return true
})

let autoTestTimer = null

watch(
  () => [props.visible, props.rule],
  () => {
    if (!props.visible) {
      clearTimeout(autoTestTimer)
      testResults.value = null
      testError.value = ''
      step.value = 1
      return
    }
    testResults.value = null
    testError.value = ''
    hydrateFormFromRule(props.rule)
    step.value = 1
  },
  { immediate: true, deep: true }
)

watch(
  () => form.value.host_id,
  async (hostId) => {
    if (!hostId) {
      // "Tous les hôtes" selected — clear host-specific metrics
      hostMetrics.value = null
      hostMetricsLoading.value = false
      hostMetricsError.value = ''
      return
    }

    // Load metrics filtered for this specific host
    hostMetricsLoading.value = true
    hostMetricsError.value = ''
    try {
      const response = await apiClient.getHostCapabilities(hostId)
      hostMetrics.value = response.data
    } catch (error) {
      hostMetricsError.value = 'Échec du chargement des métriques pour cet hôte'
      hostMetrics.value = null
    } finally {
      hostMetricsLoading.value = false
    }
  }
)

watch(
  () => [
    form.value.source_type,
    form.value.host_id,
    form.value.metric,
    form.value.operator,
    form.value.threshold_warn,
    form.value.threshold_crit,
    form.value.threshold_clear_warn,
    form.value.threshold_clear_crit,
    form.value.duration,
    form.value.proxmox_scope?.scope_mode,
    form.value.proxmox_scope?.connection_id,
    form.value.proxmox_scope?.node_id,
    form.value.proxmox_scope?.guest_id,
    form.value.proxmox_scope?.storage_id,
    form.value.proxmox_scope?.disk_id,
  ],
  () => {
    if (!props.visible) return
    if (step.value !== 2) return
    clearTimeout(autoTestTimer)
    autoTestTimer = setTimeout(testAlert, 600)
  }
)

watch(
  () => step.value,
  (currentStep) => {
    if (!props.visible) return
    if (currentStep !== 2) return
    clearTimeout(autoTestTimer)
    autoTestTimer = setTimeout(testAlert, 100)
  }
)

watch(
  () => metricSupportsHostFilter.value,
  (supportsHost) => {
    if (!supportsHost || form.value.source_type === 'proxmox') {
      form.value.host_id = null
    }
  }
)

watch(
  () => form.value.proxmox_scope?.scope_mode,
  (mode) => {
    const scope = form.value.proxmox_scope
    if (!scope) return
    if (mode !== 'connection') scope.connection_id = ''
    if (mode !== 'node') scope.node_id = ''
    if (mode !== 'guest' || !metricAllowsGuestScope.value) scope.guest_id = ''
    if (mode !== 'storage' || !metricAllowsStorageScope.value) scope.storage_id = ''
    if (mode !== 'disk' || !metricAllowsDiskScope.value) scope.disk_id = ''
  }
)

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      document.addEventListener('keydown', onKeyDown)
      return
    }
    document.removeEventListener('keydown', onKeyDown)
  },
  { immediate: true }
)

watch(
  () => metricCards.value,
  (cards) => {
    if (!Array.isArray(cards) || cards.length === 0) return
    const current = form.value.metric
    const exists = cards.some((item) => item.value === current)
    if (!exists && cards.length > 0) {
      // Selected metric is no longer available for this host
      form.value.metric = cards[0].value
      onMetricChange()
    }
  },
  { deep: true }
)

onUnmounted(() => {
  clearTimeout(autoTestTimer)
  document.removeEventListener('keydown', onKeyDown)
})

async function submit() {
  if (channelBrowser.value && typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
    browserPermission.value = await Notification.requestPermission()
  }
  emit('submit', buildPayload())
}

async function testAlert() {
  if (!props.visible) return
  testing.value = true
  testResults.value = null
  testError.value = ''
  try {
    const response = await apiClient.testAlertRule(buildPayload())
    testResults.value = response.data
  } catch (err) {
    testResults.value = null
    testError.value = err?.response?.data?.error || 'Échec du test de la règle.'
  } finally {
    testing.value = false
  }
}

function close() {
  emit('close')
}

function onKeyDown(event) {
  if (event.key === 'Escape' && props.visible) close()
}

function selectMetric(metric) {
  form.value.metric = metric
  onMetricChange()
}

function setSourceType(sourceType) {
  form.value.source_type = sourceType
  if (sourceType === 'proxmox') {
    form.value.host_id = null
    if (!isProxmoxMetric(form.value.metric)) {
      const first = metricCards.value[0]
      if (first?.value) {
        form.value.metric = first.value
      }
    }
    return
  }

  if (isProxmoxMetric(form.value.metric)) {
    const first = metricCards.value[0]
    if (first?.value) {
      form.value.metric = first.value
    }
  }
}

function goNextStep() {
  if (!canProceedStep.value || step.value >= 3) return
  step.value += 1
}

function getMetricUnit(metric) {
  return metricMetaByKey.value?.[metric]?.unit || getAlertMetricMeta(metric).unit
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>

<style scoped>
.alert-steps {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.step-chip {
  align-items: center;
  background: var(--tblr-bg-surface, #f3f6fa);
  border: 1px solid var(--tblr-border-color, #dbe3ec);
  border-radius: 0.6rem;
  color: var(--tblr-body-color, #4c5b6b);
  display: flex;
  font-weight: 600;
  gap: 0.5rem;
  justify-content: center;
  min-height: 44px;
  padding: 0.4rem 0.6rem;
}

.step-chip.active {
  background: rgba(45, 140, 255, 0.14);
  border-color: #4b9bff;
  color: #8ec2ff;
}

.step-chip.done {
  background: rgba(61, 196, 126, 0.12);
  border-color: #57c48b;
  color: #63d39a;
}

.step-chip-index {
  background: rgba(255, 255, 255, 0.75);
  color: #1f2d3d;
  border-radius: 999px;
  display: inline-flex;
  font-size: 0.85rem;
  font-weight: 700;
  height: 24px;
  justify-content: center;
  width: 24px;
}

.metric-grid {
  display: grid;
  gap: 0.8rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-card {
  align-items: center;
  background: var(--tblr-bg-surface, #ffffff);
  border: 1px solid var(--tblr-border-color, #d9e2ee);
  border-radius: 0.8rem;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  justify-content: center;
  min-height: 90px;
  padding: 0.8rem;
  transition: all 0.15s ease;
}

.metric-card:hover {
  border-color: #89b8ff;
  box-shadow: 0 2px 10px rgba(66, 132, 245, 0.18);
}

.metric-card.selected {
  background: linear-gradient(160deg, rgba(45, 140, 255, 0.14) 0%, rgba(45, 140, 255, 0.06) 100%);
  border-color: #4b9bff;
  box-shadow: inset 0 0 0 1px #4b9bff;
}

.metric-icon {
  font-size: 1.2rem;
  line-height: 1;
}

.metric-label {
  color: var(--tblr-body-color, #1f2d3d);
  font-size: 0.92rem;
  font-weight: 600;
}

[data-bs-theme='dark'] .step-chip {
  background: #1f2a3a;
  border-color: #2f3f57;
  color: #c9d6ea;
}

[data-bs-theme='dark'] .step-chip.active {
  background: rgba(33, 118, 210, 0.28);
  border-color: #4b9bff;
  color: #d2e6ff;
}

[data-bs-theme='dark'] .step-chip.done {
  background: rgba(56, 142, 99, 0.24);
  border-color: #4fb37f;
  color: #c7f2da;
}

[data-bs-theme='dark'] .step-chip-index {
  background: rgba(255, 255, 255, 0.16);
  color: #d6e4fb;
}

[data-bs-theme='dark'] .metric-card {
  background: #1f2a3a;
  border-color: #2f3f57;
}

[data-bs-theme='dark'] .metric-card.selected {
  background: linear-gradient(160deg, rgba(33, 118, 210, 0.34) 0%, rgba(18, 79, 150, 0.2) 100%);
}

@media (max-width: 768px) {
  .alert-steps {
    grid-template-columns: 1fr;
  }

  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>







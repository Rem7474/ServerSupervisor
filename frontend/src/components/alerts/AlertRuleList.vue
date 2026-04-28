<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">
        Règles actives
      </h3>
    </div>

    <div
      v-if="error"
      class="alert alert-danger m-3 mb-0"
    >
      {{ error }}
    </div>

    <!-- Spinner : en cours de chargement OU pas encore chargé une première fois -->
    <div
      v-if="loading || !fetched"
      class="card-body text-center py-5"
    >
      <div
        class="spinner-border text-primary"
        role="status"
      />
      <div class="mt-2">
        Chargement...
      </div>
    </div>

    <div
      v-else-if="rules.length === 0"
      class="card-body"
    >
      <EmptyState
        title="Aucune règle d'alerte configurée"
        subtitle="Créez votre première règle pour commencer à surveiller votre infrastructure."
        cta-label="Créer ma première alerte"
        @cta="$emit('add')"
      />
    </div>

    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>État</th>
            <th>Nom</th>
            <th>Source / Hôte</th>
            <th>Métrique</th>
            <th>Condition</th>
            <th>Durée</th>
            <th>Canaux</th>
            <th class="w-1">
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="rule in rules"
            :key="rule.id"
          >
            <td>
              <label class="form-check form-switch m-0">
                <input
                  class="form-check-input"
                  type="checkbox"
                  :checked="rule.enabled"
                  @change="$emit('toggle', rule)"
                >
              </label>
            </td>
            <td>
              <div class="d-flex align-items-center gap-2">
                <span class="fw-bold">{{ rule.name || 'Sans nom' }}</span>
                <span
                  v-if="rule.active_incident_count > 0"
                  class="badge bg-red-lt text-red"
                  :title="`${rule.active_incident_count} incident${rule.active_incident_count > 1 ? 's' : ''} actif${rule.active_incident_count > 1 ? 's' : ''}`"
                >{{ rule.active_incident_count }} actif{{ rule.active_incident_count > 1 ? 's' : '' }}</span>
              </div>
              <div
                v-if="rule.last_fired"
                class="text-muted small"
              >
                Dernière alerte: {{ formatDate(rule.last_fired) }}
              </div>
            </td>
            <td>
              <span
                v-if="ruleSourceType(rule) === 'agent'"
                class="badge bg-secondary-lt text-secondary"
              >Agent › {{ getHostName(rule.host_id) || 'Tous les hôtes' }}</span>
              <span
                v-else
                class="badge bg-cyan-lt text-cyan"
              >{{ proxmoxScopeLabel(rule) }}</span>
            </td>
            <td>
              <span
                class="badge"
                :class="getMetricBadgeClass(rule.metric)"
              >{{ getMetricLabel(rule.metric) }}</span>
            </td>
            <td>
              <div
                v-if="rule.metric === 'heartbeat_timeout'"
                class="condition-cell"
              >
                <code>{{ rule.operator }} {{ rule.threshold_crit }}s</code>
              </div>
              <div
                v-else
                class="condition-cell"
              >
                <div><code>{{ rule.operator }} {{ rule.threshold_warn }}{{ getMetricUnit(rule.metric) }} (warn)</code></div>
                <div><code>{{ rule.operator }} {{ rule.threshold_crit }}{{ getMetricUnit(rule.metric) }} (crit)</code></div>
                <div class="text-muted small mt-1">
                  clear warn:
                  <code v-if="rule.threshold_clear_warn != null">{{ formatClearThreshold(rule, rule.threshold_clear_warn) }}</code>
                  <span v-else>{{ autoHysteresisHint(rule, 'warn') }}</span>
                </div>
                <div class="text-muted small">
                  clear crit:
                  <code v-if="rule.threshold_clear_crit != null">{{ formatClearThreshold(rule, rule.threshold_clear_crit) }}</code>
                  <span v-else>{{ autoHysteresisHint(rule, 'crit') }}</span>
                </div>
              </div>
            </td>
            <td>{{ formatDurationSecs(rule.duration_seconds) }}</td>
            <td>
              <span
                v-for="channel in rule.actions?.channels"
                :key="channel"
                class="badge me-1"
                :class="channelBadgeClass(channel)"
              >
                {{ channelLabel(channel) }}
              </span>
              <span
                v-if="rule.actions?.command_trigger"
                class="badge bg-orange-lt text-orange me-1"
                :title="`${rule.actions.command_trigger.module}/${rule.actions.command_trigger.action}${rule.actions.command_trigger.target ? ' -> ' + rule.actions.command_trigger.target : ''}`"
              >
                cmd
              </span>
            </td>
            <td>
              <div class="btn-group">
                <button
                  class="btn btn-sm btn-ghost-secondary"
                  title="Modifier"
                  @click="$emit('edit', rule)"
                >
                  <svg
                    class="icon"
                    width="20"
                    height="20"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                    />
                  </svg>
                </button>
                <button
                  class="btn btn-sm btn-ghost-danger"
                  title="Supprimer"
                  @click="$emit('delete', rule)"
                >
                  <svg
                    class="icon"
                    width="20"
                    height="20"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import EmptyState from '../EmptyState.vue'
import { formatDurationSecs } from '../../utils/formatters'
import { getAlertMetricMeta } from '../../utils/alertMetrics'

const CHANNEL_LABELS = { browser: 'Navigateur', smtp: 'Email', ntfy: 'Ntfy', notify: 'Système' }
const CHANNEL_BADGE_CLASSES = {
  browser: 'bg-green-lt text-green',
  smtp: 'bg-azure-lt text-azure',
  ntfy: 'bg-azure-lt text-azure',
  notify: 'bg-purple-lt text-purple',
}

function channelLabel(channel) {
  return CHANNEL_LABELS[channel] || channel
}

function channelBadgeClass(channel) {
  return CHANNEL_BADGE_CLASSES[channel] || 'bg-azure-lt text-azure'
}

const props = defineProps({
  rules: {
    type: Array,
    default: () => [],
  },
  hosts: {
    type: Array,
    default: () => [],
  },
  loading: {
    type: Boolean,
    default: false,
  },
  // fetched = true une fois que le premier fetch a abouti (succès ou erreur)
  fetched: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
  formatDate: {
    type: Function,
    required: true,
  },
})

defineEmits(['add', 'edit', 'toggle', 'delete'])

function getHostName(hostId) {
  return hostId
    ? (Array.isArray(props.hosts) ? props.hosts.find((host) => host.id === hostId)?.name || hostId : hostId)
    : hostId
}

function getMetricLabel(metric) {
  return getAlertMetricMeta(metric).label
}

function getMetricBadgeClass(metric) {
  return getAlertMetricMeta(metric).badgeClass
}

function getMetricUnit(metric) {
  return getAlertMetricMeta(metric).unit
}

function formatClearThreshold(rule, value) {
  return `${rule.operator} ${value}${getMetricUnit(rule.metric)}`
}

function autoHysteresisHint(rule, level) {
  if (level === 'crit') {
    return 'auto : résolution quand la condition crit n\'est plus vraie'
  }
  return 'auto : résolution quand aucune condition n\'est vraie'
}

function ruleSourceType(rule) {
  if (rule?.source_type) return rule.source_type
  return String(rule?.metric || '').startsWith('proxmox_') ? 'proxmox' : 'agent'
}

function proxmoxScopeLabel(rule) {
  const scope = rule?.proxmox_scope
  if (!scope || !scope.scope_mode || scope.scope_mode === 'global') return 'Proxmox › Cluster'
  if (scope.scope_mode === 'connection') return `Proxmox › Connexion ${scope.connection_id || ''}`.trim()
  if (scope.scope_mode === 'node') return `Proxmox › Nœud ${scope.node_id || ''}`.trim()
  if (scope.scope_mode === 'guest') return `Proxmox › VM/LXC ${scope.guest_id || ''}`.trim()
  if (scope.scope_mode === 'storage') return `Proxmox › Stockage ${scope.storage_id || ''}`.trim()
  if (scope.scope_mode === 'disk') return `Proxmox › Disque ${scope.disk_id || ''}`.trim()
  return 'Proxmox › Scope inconnu'
}
</script>

<style scoped>
.condition-cell {
  line-height: 1.4;
}
</style>

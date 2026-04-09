<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        Regles actives
      </h3>
      <button
        class="btn btn-primary btn-sm"
        @click="$emit('add')"
      >
        <svg
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
            d="M12 4v16m8-8H4"
          />
        </svg>
        Nouvelle alerte
      </button>
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
      class="card-body text-center py-5 text-muted"
    >
      <svg
        class="icon icon-lg mb-3 text-muted"
        width="48"
        height="48"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
        />
      </svg>
      <div>Aucune regle d'alerte configuree</div>
      <button
        class="btn btn-primary mt-3"
        @click="$emit('add')"
      >
        Creer ma premiere alerte
      </button>
    </div>

    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Etat</th>
            <th>Nom</th>
            <th>Source / Hote</th>
            <th>Metrique</th>
            <th>Condition</th>
            <th>Duree</th>
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
              <div class="fw-bold">
                {{ rule.name || 'Sans nom' }}
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
              >Agent › {{ getHostName(rule.host_id) || 'Tous les hotes' }}</span>
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
            <td><code>{{ rule.operator }} {{ rule.threshold }}{{ getMetricUnit(rule.metric) }}</code></td>
            <td>{{ formatDurationSecs(rule.duration_seconds) }}</td>
            <td>
              <span
                v-for="channel in rule.actions?.channels"
                :key="channel"
                class="badge me-1"
                :class="channel === 'browser' ? 'bg-green-lt text-green' : 'bg-azure-lt text-azure'"
              >
                {{ channel === 'browser' ? 'Navigateur' : channel }}
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
import { formatDurationSecs } from '../../utils/formatters'
import { getAlertMetricMeta } from '../../utils/alertMetrics'

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

function ruleSourceType(rule) {
  if (rule?.source_type) return rule.source_type
  return String(rule?.metric || '').startsWith('proxmox_') ? 'proxmox' : 'agent'
}

function proxmoxScopeLabel(rule) {
  const scope = rule?.proxmox_scope
  if (!scope || !scope.scope_mode || scope.scope_mode === 'global') return 'Proxmox › Cluster'
  if (scope.scope_mode === 'connection') return `Proxmox › Connexion ${scope.connection_id || ''}`.trim()
  if (scope.scope_mode === 'node') return `Proxmox › Noeud ${scope.node_id || ''}`.trim()
  if (scope.scope_mode === 'guest') return `Proxmox › VM/LXC ${scope.guest_id || ''}`.trim()
  if (scope.scope_mode === 'storage') return `Proxmox › Stockage ${scope.storage_id || ''}`.trim()
  if (scope.scope_mode === 'disk') return `Proxmox › Disque ${scope.disk_id || ''}`.trim()
  return 'Proxmox › Scope inconnu'
}
</script>


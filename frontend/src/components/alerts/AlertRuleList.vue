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
                  v-if="(rule.active_incident_count ?? 0) > 0"
                  class="badge bg-red-lt text-red"
                  :title="`${rule.active_incident_count} incident${(rule.active_incident_count ?? 0) > 1 ? 's' : ''} actif${(rule.active_incident_count ?? 0) > 1 ? 's' : ''}`"
                >{{ rule.active_incident_count }} actif{{ (rule.active_incident_count ?? 0) > 1 ? 's' : '' }}</span>
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
                v-else-if="ruleSourceType(rule) === 'docker'"
                class="badge bg-teal-lt text-teal"
              >{{ dockerScopeLabel(rule) }}</span>
              <span
                v-else-if="ruleSourceType(rule) === 'synthetic'"
                class="badge bg-purple-lt text-purple"
              >Synthétique</span>
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
                  type="button"
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
                  type="button"
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

<script setup lang="ts">
import EmptyState from '../EmptyState.vue'
import { formatDurationSecs } from '../../utils/formatters'
import { getAlertMetricMeta } from '../../utils/alertMetrics'

interface Host {
  id: string
  name?: string
}

interface ProxmoxScope {
  scope_mode?: string
  connection_id?: string | number
  node_id?: string | number
  guest_id?: string | number
  storage_id?: string | number
  disk_id?: string | number
}

interface CommandTrigger {
  module: string
  action: string
  target?: string
}

interface AlertActions {
  channels?: string[]
  command_trigger?: CommandTrigger | null
}

interface DockerScope {
  scope_mode?: string
  host_id?: string
  container_id?: string
  project_name?: string
}

interface AlertRule {
  id: string | number
  name?: string
  enabled?: boolean
  host_id?: string
  source_type?: string
  metric: string
  operator: string
  threshold_warn?: number
  threshold_crit?: number
  threshold_clear_warn?: number | null
  threshold_clear_crit?: number | null
  duration_seconds?: number
  active_incident_count?: number
  last_fired?: string
  actions?: AlertActions
  proxmox_scope?: ProxmoxScope
  docker_scope?: DockerScope
}

const CHANNEL_LABELS: Record<string, string> = { browser: 'Navigateur', smtp: 'Email', ntfy: 'Ntfy', notify: 'Système' }
const CHANNEL_BADGE_CLASSES: Record<string, string> = {
  browser: 'bg-green-lt text-green',
  smtp: 'bg-azure-lt text-azure',
  ntfy: 'bg-azure-lt text-azure',
  notify: 'bg-purple-lt text-purple',
}

function channelLabel(channel: string): string {
  return CHANNEL_LABELS[channel] || channel
}

function channelBadgeClass(channel: string): string {
  return CHANNEL_BADGE_CLASSES[channel] || 'bg-azure-lt text-azure'
}

const props = withDefaults(defineProps<{
  rules?: AlertRule[]
  hosts?: Host[]
  loading?: boolean
  fetched?: boolean
  error?: string
  formatDate: (d: string | undefined) => string
}>(), {
  rules: () => [],
  hosts: () => [],
  loading: false,
  fetched: false,
  error: '',
})

defineEmits<{
  (e: 'add'): void
  (e: 'edit', rule: AlertRule): void
  (e: 'toggle', rule: AlertRule): void
  (e: 'delete', rule: AlertRule): void
}>()

function getHostName(hostId: string | undefined): string | undefined {
  return hostId
    ? (Array.isArray(props.hosts) ? props.hosts.find((host) => host.id === hostId)?.name || hostId : hostId)
    : hostId
}

function getMetricLabel(metric: string): string {
  return getAlertMetricMeta(metric).label
}

function getMetricBadgeClass(metric: string): string {
  return getAlertMetricMeta(metric).badgeClass
}

function getMetricUnit(metric: string): string {
  return getAlertMetricMeta(metric).unit
}

function formatClearThreshold(rule: AlertRule, value: number): string {
  return `${rule.operator} ${value}${getMetricUnit(rule.metric)}`
}

function autoHysteresisHint(_rule: AlertRule, level: 'warn' | 'crit'): string {
  if (level === 'crit') {
    return 'auto : résolution quand la condition crit n\'est plus vraie'
  }
  return 'auto : résolution quand aucune condition n\'est vraie'
}

function ruleSourceType(rule: AlertRule): string {
  if (rule?.source_type) return rule.source_type
  return String(rule?.metric || '').startsWith('proxmox_') ? 'proxmox' : 'agent'
}

function proxmoxScopeLabel(rule: AlertRule): string {
  const scope = rule?.proxmox_scope
  if (!scope || !scope.scope_mode || scope.scope_mode === 'global') return 'Proxmox › Cluster'
  if (scope.scope_mode === 'connection') return `Proxmox › Connexion ${scope.connection_id || ''}`.trim()
  if (scope.scope_mode === 'node') return `Proxmox › Nœud ${scope.node_id || ''}`.trim()
  if (scope.scope_mode === 'guest') return `Proxmox › VM/LXC ${scope.guest_id || ''}`.trim()
  if (scope.scope_mode === 'storage') return `Proxmox › Stockage ${scope.storage_id || ''}`.trim()
  if (scope.scope_mode === 'disk') return `Proxmox › Disque ${scope.disk_id || ''}`.trim()
  return 'Proxmox › Scope inconnu'
}

function dockerScopeLabel(rule: AlertRule): string {
  const scope = rule?.docker_scope
  if (!scope) return 'Docker'
  if (scope.scope_mode === 'compose_project') return `Compose › ${scope.project_name || 'Projet inconnu'}`
  if (scope.scope_mode === 'container') return `Docker › Conteneur`
  return `Docker › Tous les conteneurs`
}
</script>

<style scoped>
.condition-cell {
  line-height: 1.4;
}
</style>

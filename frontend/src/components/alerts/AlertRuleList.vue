<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('alerts.activeRulesTitle') }}
      </h3>
    </div>

    <div
      v-if="error"
      class="alert alert-danger m-3 mb-0"
    >
      {{ error }}
    </div>

    <div
      v-if="loading || !fetched"
      class="card-body"
    >
      <LoadingSkeleton variant="table" />
    </div>

    <div
      v-else-if="rules.length === 0"
      class="card-body"
    >
      <EmptyState
        :title="t('alerts.noRulesTitle')"
        :subtitle="isAdmin ? t('alerts.noRulesSubtitleAdmin') : t('alerts.noRulesSubtitleViewer')"
        :cta-label="isAdmin ? t('alerts.createFirstAlertCta') : ''"
        @cta="$emit('add')"
      />
    </div>

    <div
      v-else
      class="table-responsive scroll-table"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('alerts.stateColumn') }}</th>
            <th>
              <SortableHeader
                :label="t('alerts.nameColumn')"
                :active="sortKey === 'name'"
                :direction="sortDir"
                @toggle="toggleSort('name')"
              />
            </th>
            <th>{{ t('alerts.sourceHostColumn') }}</th>
            <th>
              <SortableHeader
                :label="t('alerts.metricColumn')"
                :active="sortKey === 'metric'"
                :direction="sortDir"
                @toggle="toggleSort('metric')"
              />
            </th>
            <th>{{ t('alerts.conditionColumn') }}</th>
            <th>
              <SortableHeader
                :label="t('alerts.durationColumn')"
                :active="sortKey === 'duration_seconds'"
                :direction="sortDir"
                @toggle="toggleSort('duration_seconds')"
              />
            </th>
            <th>{{ t('alerts.channelsColumn') }}</th>
            <th class="w-1">
              {{ t('alerts.actionsColumn') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="rule in sortedRules"
            :key="rule.id"
            :class="{ 'opacity-60': !rule.enabled }"
          >
            <td>
              <label
                class="form-check form-switch m-0"
                :title="isAdmin ? '' : t('alerts.adminOnlyTitle')"
              >
                <input
                  class="form-check-input"
                  type="checkbox"
                  :checked="rule.enabled"
                  :disabled="!isAdmin"
                  @change="$emit('toggle', rule)"
                >
              </label>
            </td>
            <td>
              <div class="d-flex align-items-center gap-2">
                <span class="fw-bold">{{ rule.name || t('alerts.unnamedRule') }}</span>
                <span
                  v-if="(rule.active_incident_count ?? 0) > 0"
                  class="badge bg-danger-lt text-danger"
                  :title="t('alerts.activeIncidentsBadge', { count: rule.active_incident_count ?? 0 }, rule.active_incident_count ?? 0)"
                >{{ t('alerts.activeIncidentsBadge', { count: rule.active_incident_count ?? 0 }, rule.active_incident_count ?? 0) }}</span>
              </div>
              <div
                v-if="rule.last_fired"
                class="text-muted small"
              >
                {{ t('alerts.lastFiredLabel', { date: formatDate(rule.last_fired) }) }}
              </div>
            </td>
            <td>
              <span
                v-if="ruleSourceType(rule) === 'agent'"
                class="badge bg-secondary-lt text-secondary"
              >{{ t('alerts.agentSourceLabel', { host: getHostName(rule.host_id) || t('alerts.allHostsBadge') }) }}</span>
              <span
                v-else-if="ruleSourceType(rule) === 'docker'"
                class="badge bg-teal-lt text-teal"
              >{{ dockerScopeLabel(rule) }}</span>
              <span
                v-else-if="ruleSourceType(rule) === 'synthetic'"
                class="badge bg-purple-lt text-purple"
              >{{ t('alerts.syntheticLabel') }}</span>
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
                  {{ t('alerts.clearWarnLabel') }}
                  <code v-if="rule.threshold_clear_warn != null">{{ formatClearThreshold(rule, rule.threshold_clear_warn) }}</code>
                  <span v-else>{{ autoHysteresisHint(rule, 'warn') }}</span>
                </div>
                <div class="text-muted small">
                  {{ t('alerts.clearCritLabel') }}
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
              <div
                v-if="isAdmin"
                class="btn-group"
              >
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :title="t('alerts.editTooltip')"
                  :aria-label="t('alerts.editRuleAriaLabel')"
                  @click="$emit('edit', rule)"
                >
                  <IconPencil
                    :size="14"
                    class="icon"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :title="t('alerts.deleteTooltip')"
                  :aria-label="t('alerts.deleteRuleAriaLabel')"
                  @click="$emit('delete', rule)"
                >
                  <IconTrash
                    :size="14"
                    class="icon"
                  />
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
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import SortableHeader from '../common/SortableHeader.vue'
import { IconPencil, IconTrash } from '@tabler/icons-vue'
import { formatDurationSecs } from '../../utils/formatters'
import { getAlertMetricMeta } from '../../utils/alertMetrics'

const { t } = useI18n()

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

const CHANNEL_BADGE_CLASSES: Record<string, string> = {
  browser: 'bg-green-lt text-green',
  smtp: 'bg-azure-lt text-azure',
  ntfy: 'bg-azure-lt text-azure',
  notify: 'bg-purple-lt text-purple',
}

function channelLabel(channel: string): string {
  if (channel === 'browser') return t('alerts.channelBrowserLabel')
  if (channel === 'smtp') return t('alerts.channelEmail')
  if (channel === 'ntfy') return t('alerts.channelNtfyShort')
  if (channel === 'notify') return t('alerts.channelNotify')
  return channel
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
  isAdmin?: boolean
  formatDate: (d: string | undefined) => string
}>(), {
  rules: () => [],
  hosts: () => [],
  loading: false,
  fetched: false,
  error: '',
  isAdmin: false,
})

defineEmits<{
  (e: 'add'): void
  (e: 'edit', rule: AlertRule): void
  (e: 'toggle', rule: AlertRule): void
  (e: 'delete', rule: AlertRule): void
}>()

type SortKey = 'name' | 'metric' | 'duration_seconds'
const sortKey = ref<SortKey>('name')
const sortDir = ref<'asc' | 'desc'>('asc')

function toggleSort(key: SortKey): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

const sortedRules = computed(() => {
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...props.rules].sort((a, b) => {
    if (sortKey.value === 'duration_seconds') {
      return ((a.duration_seconds ?? 0) - (b.duration_seconds ?? 0)) * dir
    }
    const av = sortKey.value === 'metric' ? getMetricLabel(a.metric) : (a.name || '')
    const bv = sortKey.value === 'metric' ? getMetricLabel(b.metric) : (b.name || '')
    return av.toLowerCase().localeCompare(bv.toLowerCase()) * dir
  })
})

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
    return t('alerts.autoResolveCrit')
  }
  return t('alerts.autoResolveOther')
}

function ruleSourceType(rule: AlertRule): string {
  if (rule?.source_type) return rule.source_type
  return String(rule?.metric || '').startsWith('proxmox_') ? 'proxmox' : 'agent'
}

function proxmoxScopeLabel(rule: AlertRule): string {
  const scope = rule?.proxmox_scope
  if (!scope || !scope.scope_mode || scope.scope_mode === 'global') return t('alerts.proxmoxCluster')
  if (scope.scope_mode === 'connection') return t('alerts.proxmoxConnection', { id: scope.connection_id || '' }).trim()
  if (scope.scope_mode === 'node') return t('alerts.proxmoxNode', { id: scope.node_id || '' }).trim()
  if (scope.scope_mode === 'guest') return t('alerts.proxmoxGuest', { id: scope.guest_id || '' }).trim()
  if (scope.scope_mode === 'storage') return t('alerts.proxmoxStorage', { id: scope.storage_id || '' }).trim()
  if (scope.scope_mode === 'disk') return t('alerts.proxmoxDisk', { id: scope.disk_id || '' }).trim()
  return t('alerts.proxmoxUnknownScope')
}

function dockerScopeLabel(rule: AlertRule): string {
  const scope = rule?.docker_scope
  if (!scope) return t('alerts.dockerLabel')
  if (scope.scope_mode === 'compose_project') return t('alerts.dockerComposeLabel', { project: scope.project_name || t('alerts.dockerUnknownProject') })
  if (scope.scope_mode === 'container') return t('alerts.dockerContainerLabel')
  return t('alerts.dockerAllContainersLabel')
}
</script>

<style scoped>
.condition-cell {
  line-height: 1.4;
}
</style>

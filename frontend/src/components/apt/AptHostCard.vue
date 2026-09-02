<template>
  <div class="card">
    <!-- Header: identity + status + per-host actions -->
    <div class="card-header">
      <div class="d-flex align-items-center gap-3 flex-wrap w-100">
        <label class="form-check m-0">
          <input
            type="checkbox"
            class="form-check-input"
            :checked="selected"
            @change="$emit('update:selected', ($event.target as HTMLInputElement).checked)"
          >
          <span class="form-check-label" />
        </label>
        <div class="flex-fill min-w-0">
          <div class="d-flex align-items-center gap-2 flex-wrap">
            <router-link
              :to="`/hosts/${host.id}`"
              class="fw-semibold text-reset text-decoration-none"
            >
              {{ host.name || host.hostname }}
            </router-link>
            <span
              v-if="host.name && host.hostname && host.name !== host.hostname"
              class="text-secondary small"
            >
              {{ host.hostname }}
            </span>
            <span class="text-muted small">{{ host.ip_address }}</span>
          </div>
        </div>
        <span :class="host.status === 'online' ? 'status status-success' : 'status status-danger'">
          <span :class="['status-dot', host.status === 'online' ? 'status-dot-animated' : '']" />
          <span>{{ host.status === 'online' ? t('apt.online') : t('apt.offline') }}</span>
        </span>
        <span
          v-if="activeCommand"
          class="badge bg-primary-lt text-primary d-inline-flex align-items-center gap-1 flex-shrink-0"
        >
          <span
            class="spinner-border spinner-border-sm"
            role="status"
          />
          apt {{ activeCommand.action }} — {{ statusLabel(activeCommand.status) }}
        </span>
        <span
          v-else-if="enriching"
          class="badge bg-secondary-lt text-secondary d-inline-flex align-items-center gap-1 flex-shrink-0"
          :title="t('apt.refreshingDataTooltip')"
        >
          <span
            class="spinner-border spinner-border-sm"
            role="status"
          />
          {{ t('apt.refreshingData') }}
        </span>
        <span
          v-if="uuStatus?.installed"
          class="badge flex-shrink-0"
          :class="uuStatus.enabled ? 'bg-success-lt text-success' : 'bg-secondary-lt text-secondary'"
          :title="t('apt.autoUpdatesTooltip')"
        >
          {{ uuStatus.enabled ? t('apt.autoUpdatesEnabledBadge') : t('apt.autoUpdatesDisabledBadge') }}
        </span>
        <span
          v-if="uuStatus?.reboot_required"
          class="badge bg-warning-lt text-warning flex-shrink-0"
          :title="t('apt.rebootRequiredTooltip')"
        >
          {{ t('apt.rebootRequired') }}
        </span>
        <span
          v-if="agentOutdated"
          class="badge bg-primary-lt text-primary flex-shrink-0"
          :title="t('apt.agentOutdatedTooltip', { version: host.agent_version || '?', latest: latestAgentVersion })"
        >
          {{ t('apt.filterOutdatedAgent') }}
        </span>
        <button
          v-if="activeCommand"
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary flex-shrink-0"
          :title="t('apt.viewLiveLogs')"
          @click="$emit('watch-command', activeCommand)"
        >
          <IconList
            :size="16"
            class="icon icon-sm"
          />
        </button>
        <div
          v-if="canRunApt"
          class="d-flex gap-1 flex-shrink-0"
        >
          <div class="btn-group btn-group-sm">
            <button
              type="button"
              class="btn btn-outline-secondary"
              :disabled="isCmdLoading"
              @click="$emit('run-cmd', 'update')"
            >
              <span
                v-if="cmdLoading === 'update'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              update
            </button>
            <button
              type="button"
              class="btn btn-outline-primary"
              :disabled="isCmdLoading"
              @click="$emit('run-cmd', 'upgrade')"
            >
              <span
                v-if="cmdLoading === 'upgrade'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              upgrade
            </button>
            <button
              type="button"
              class="btn btn-outline-danger"
              :disabled="isCmdLoading"
              @click="$emit('run-cmd', 'dist-upgrade')"
            >
              <span
                v-if="cmdLoading === 'dist-upgrade'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              dist-upgrade
            </button>
          </div>
          <button
            type="button"
            class="btn btn-icon btn-sm btn-outline-secondary"
            :title="t('apt.scheduleModalTitle')"
            @click="$emit('schedule')"
          >
            <IconCalendar
              :size="16"
              class="icon icon-sm"
            />
          </button>
        </div>
        <span
          v-else
          class="text-secondary small flex-shrink-0"
        >{{ t('apt.readOnlyMode') }}</span>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary flex-shrink-0"
          :title="expanded ? t('apt.collapse') : t('apt.expandTooltip')"
          @click="$emit('update:expanded', !expanded)"
        >
          <IconChevronDown :size="16" />
        </button>
      </div>
    </div>

    <!-- Body: KPI + CVE + packages + history -->
    <div class="card-body">
      <!-- No data -->
      <div
        v-if="!aptStatus"
        class="text-secondary small"
      >
        {{ t('apt.noAptDataIntro') }} <strong>apt update</strong> {{ t('apt.noAptDataOutro') }}
      </div>

      <template v-else>
        <!-- KPI stats (always visible) -->
        <div class="row g-2 mb-1">
          <div class="col-3">
            <div
              class="text-center p-2 rounded"
              :class="(aptStatus.pending_packages ?? 0) > 0 ? 'bg-warning-lt' : 'bg-success-lt'"
            >
              <div
                class="fs-3 fw-bold lh-1 mb-1"
                :class="(aptStatus.pending_packages ?? 0) > 0 ? 'text-warning' : 'text-success'"
              >
                {{ aptStatus.pending_packages ?? 0 }}
              </div>
              <div class="text-secondary small">
                {{ t('apt.pendingLabel') }}
              </div>
            </div>
          </div>
          <div class="col-3">
            <div
              class="text-center p-2 rounded"
              :class="(aptStatus.security_updates ?? 0) > 0 ? 'bg-danger-lt' : 'bg-secondary-lt'"
            >
              <div
                class="fs-3 fw-bold lh-1 mb-1"
                :class="(aptStatus.security_updates ?? 0) > 0 ? 'text-danger' : 'text-secondary'"
              >
                {{ aptStatus.security_updates ?? 0 }}
              </div>
              <div class="text-secondary small">
                {{ t('apt.securityLabel') }}
              </div>
            </div>
          </div>
          <div class="col-3">
            <div
              class="text-center p-2 rounded"
              :class="cveList.length
                ? (cveList.some(c => c.severity === 'CRITICAL') ? 'bg-danger-lt' : 'bg-warning-lt')
                : 'bg-secondary-lt'"
            >
              <div
                class="fs-3 fw-bold lh-1 mb-1"
                :class="cveList.length
                  ? (cveList.some(c => c.severity === 'CRITICAL') ? 'text-danger' : 'text-warning')
                  : 'text-secondary'"
              >
                {{ cveList.length }}
              </div>
              <div class="text-secondary small">
                {{ t('apt.cveLabel') }}
              </div>
            </div>
          </div>
          <div class="col-3">
            <div class="text-center p-2 rounded bg-secondary-lt">
              <div class="text-secondary small">
                {{ t('apt.lastAptUpdate') }}
              </div>
              <div class="fw-semibold small lh-1 mb-2 text-truncate">
                {{ aptStatus.last_update ? formatDate(aptStatus.last_update) : t('common.never') }}
              </div>
              <div class="text-secondary small">
                {{ t('apt.lastUpgrade') }}
              </div>
              <div class="fw-semibold small lh-1 text-truncate">
                {{ aptStatus.last_upgrade ? formatDate(aptStatus.last_upgrade) : t('common.never') }}
              </div>
            </div>
          </div>
        </div>

        <!-- Expandable detail -->
        <template v-if="expanded">
          <!-- Automatic updates (summary) -->
          <div
            v-if="uuStatus"
            class="mt-3 d-flex align-items-center gap-2 flex-wrap small"
          >
            <span class="fw-semibold text-secondary">{{ t('apt.autoUpdatesSummaryLabel') }}</span>
            <span
              v-if="!uuStatus.installed"
              class="badge bg-secondary-lt text-secondary"
            >{{ t('apt.notInstalled') }}</span>
            <template v-else>
              <span
                class="badge"
                :class="uuStatus.enabled ? 'bg-success-lt text-success' : 'bg-secondary-lt text-secondary'"
              >{{ uuStatus.enabled ? t('apt.enabledBadge') : t('apt.disabledBadge') }}</span>
              <span
                v-if="uuStatus.last_run_at"
                class="text-secondary"
              >{{ t('apt.lastRunAt', { date: formatDate(uuStatus.last_run_at) }) }}</span>
            </template>
          </div>

          <!-- CVE -->
          <div
            v-if="cveList.length"
            class="mt-3 mb-3"
          >
            <CVEList
              :cve-list="cveList"
              :show-max-severity="true"
              :always-expanded="false"
              :initially-collapsed="false"
              :limit="3"
            />
          </div>

          <!-- Paquets en attente -->
          <AptPendingPackagesList :packages="packages" />

          <!-- History (last 2 commands) -->
          <div
            v-if="history?.length"
            class="border-top pt-2"
          >
            <div class="d-flex align-items-center justify-content-between mb-1">
              <span class="small fw-semibold text-secondary">{{ t('apt.lastCommands') }}</span>
              <router-link
                to="/audit?module=apt"
                class="small text-secondary text-decoration-none"
              >
                {{ t('apt.fullHistory') }}
              </router-link>
            </div>
            <div
              v-for="cmd in history.slice(0, 2)"
              :key="cmd.id"
              class="d-flex align-items-center gap-2 py-1 flex-wrap"
            >
              <code class="small">apt {{ cmd.action }}</code>
              <span :class="statusClass(cmd.status)">{{ statusLabel(cmd.status) }}</span>
              <span class="text-secondary small flex-shrink-0">{{ formatDate(cmd.created_at ?? "") }}</span>
              <span
                v-if="cmd.triggered_by"
                class="text-muted small flex-shrink-0"
              >· {{ cmd.triggered_by }}</span>
              <button
                type="button"
                class="btn btn-icon btn-sm btn-ghost-secondary ms-auto flex-shrink-0"
                :title="t('apt.viewLogs')"
                @click="$emit('watch-command', cmd)"
              >
                <IconList
                  :size="16"
                  class="icon icon-sm"
                />
              </button>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconChevronDown, IconList, IconCalendar } from '@tabler/icons-vue'
import CVEList from './CVEList.vue'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { useStatusBadge } from '../../composables/useStatusBadge'
import AptPendingPackagesList from './AptPendingPackagesList.vue'
import type { Host } from '../../types/host'
import type { UnattendedUpgradesDB } from '../../types/ws'

interface CveInfo { severity?: string; [key: string]: unknown }
interface AptStatusView {
  pending_packages?: number
  security_updates?: number
  last_update?: string
  last_upgrade?: string
  cve_list?: unknown
  package_list?: unknown
  [key: string]: unknown
}
interface AptCommandRow {
  id: string
  action?: string
  status?: string
  created_at?: string
  triggered_by?: string
  [key: string]: unknown
}

const props = defineProps<{
  host: Host
  aptStatus: AptStatusView | undefined
  history: AptCommandRow[] | undefined
  expanded: boolean
  selected: boolean
  canRunApt: boolean
  cmdLoading: string | null | undefined
  enriching?: boolean
  uuStatus?: UnattendedUpgradesDB | undefined
  agentOutdated?: boolean
  latestAgentVersion?: string
}>()

defineEmits<{
  (e: 'update:selected', value: boolean): void
  (e: 'update:expanded', value: boolean): void
  (e: 'run-cmd', command: string): void
  (e: 'schedule'): void
  (e: 'watch-command', cmd: AptCommandRow): void
}>()

const { t } = useI18n()
const { formatRelativeDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

const activeCommand = computed(() =>
  props.history?.find((cmd) => cmd.status === 'pending' || cmd.status === 'running')
)
const isCmdLoading = computed(() => !!props.cmdLoading || !!activeCommand.value)

function parseJsonArray<T = unknown>(value: unknown): T[] {
  if (!value) return []
  try {
    const parsed = typeof value === 'string' ? JSON.parse(value) : value
    return Array.isArray(parsed) ? (parsed as T[]) : []
  } catch {
    return []
  }
}

const cveList = computed(() => parseJsonArray<CveInfo>(props.aptStatus?.cve_list))
const packages = computed(() => parseJsonArray<string>(props.aptStatus?.package_list))

function formatDate(date: string | undefined): string {
  return formatRelativeDate(date)
}

const STATUS_LABEL_KEYS: Record<string, string> = {
  pending: 'apt.statusPending',
  running: 'apt.statusRunning',
  completed: 'apt.statusCompleted',
  failed: 'apt.statusFailed',
}

function statusLabel(status?: string): string {
  const key = status && STATUS_LABEL_KEYS[status]
  return key ? t(key) : status ?? ''
}

function statusClass(status: string | undefined): string {
  return getStatusBadgeClass(status, 'badge bg-warning-lt text-warning')
}
</script>

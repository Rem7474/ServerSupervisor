<template>
  <div
    v-if="aptStatus"
    class="card"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        {{ t('host.aptTitle') }}
      </h3>
      <div
        v-if="canRunApt"
        class="btn-group btn-group-sm"
      >
        <button
          type="button"
          class="btn btn-outline-secondary"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'update')"
        >
          <span
            v-if="aptCmdLoading === 'update'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt update
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'upgrade')"
        >
          <span
            v-if="aptCmdLoading === 'upgrade'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt upgrade
        </button>
        <button
          type="button"
          class="btn btn-outline-danger"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'dist-upgrade')"
        >
          <span
            v-if="aptCmdLoading === 'dist-upgrade'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt dist-upgrade
        </button>
      </div>
      <span
        v-else
        class="text-secondary small"
      >{{ t('host.aptReadOnlyMode') }}</span>
    </div>
    <div class="card-body">
      <div class="row row-cards">
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div
                class="h2 mb-0"
                :class="(aptStatus.pending_packages ?? 0) > 0 ? 'text-warning' : 'text-success'"
              >
                {{ aptStatus.pending_packages }}
              </div>
              <div class="text-secondary small">
                {{ t('host.aptPendingPackagesLabel') }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="h2 mb-0 text-danger">
                {{ aptStatus.security_updates }}
              </div>
              <div class="text-secondary small">
                {{ t('host.aptSecurityUpdatesLabel') }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="text-secondary small">
                {{ t('host.aptLastUpdateLabel') }}
              </div>
              <div class="fw-semibold">
                {{ formatDate(aptStatus.last_update) }}
              </div>
              <div class="text-secondary small mt-2">
                {{ t('host.aptLastUpgradeLabel') }}
              </div>
              <div class="fw-semibold">
                {{ formatDate(lastUpgradeDate) }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="aptStatus.cve_list"
        class="mt-3"
      >
        <CVEList
          :cve-list="(aptStatus.cve_list as any)"
          :show-max-severity="true"
          :always-expanded="true"
        />
      </div>

      <AptPendingPackagesList
        class="mt-3"
        :packages="pendingPackages"
      />
    </div>
  </div>
  <div
    v-else
    class="card"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        {{ t('host.aptTitle') }}
      </h3>
      <div
        v-if="canRunApt"
        class="btn-group btn-group-sm"
      >
        <button
          type="button"
          class="btn btn-outline-secondary"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'update')"
        >
          <span
            v-if="aptCmdLoading === 'update'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt update
        </button>
      </div>
      <span
        v-else
        class="text-secondary small"
      >{{ t('host.aptReadOnlyMode') }}</span>
    </div>
    <div class="card-body text-secondary small">
      {{ t('host.aptNoDataPrefix') }} <strong>apt update</strong> {{ t('host.aptNoDataSuffix') }}
    </div>
  </div>

  <!-- Unattended-upgrades card -->
  <div class="card mt-3">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        {{ t('host.uuTitle') }}
      </h3>
      <div
        v-if="uuStatus"
        class="d-flex align-items-center gap-2"
      >
        <span
          v-if="!uuStatus.installed"
          class="badge bg-secondary-lt text-secondary"
        >{{ t('host.uuNotInstalledBadge') }}</span>
        <template v-else>
          <span
            class="badge"
            :class="uuStatus.enabled ? 'bg-success-lt text-success' : 'bg-secondary-lt text-secondary'"
          >{{ uuStatus.enabled ? t('host.uuEnabledBadge') : t('host.uuDisabledBadge') }}</span>
          <span
            v-if="uuStatus.reboot_required"
            class="badge bg-warning-lt text-warning"
          >{{ t('host.uuRebootRequiredBadge') }}</span>
        </template>
      </div>
    </div>
    <div class="card-body">
      <!-- Not installed -->
      <div
        v-if="uuStatus && !uuStatus.installed"
        class="d-flex align-items-center gap-3"
      >
        <span class="text-secondary">{{ t('host.uuNotInstalledMessage') }}</span>
        <button
          v-if="canRunApt"
          type="button"
          class="btn btn-sm btn-primary"
          :disabled="uuLoading === 'install'"
          @click="$emit('uu-install')"
        >
          <span
            v-if="uuLoading === 'install'"
            class="spinner-border spinner-border-sm me-1"
          />
          {{ t('host.uuInstallButton') }}
        </button>
      </div>

      <!-- Installed -->
      <div v-else-if="uuStatus && uuStatus.installed">
        <!-- Last run info -->
        <div
          v-if="uuStatus.last_run_at"
          class="mb-3 text-secondary small"
        >
          {{ t('host.uuLastRunPrefix') }} <strong>{{ formatDate(uuStatus.last_run_at) }}</strong>
          — {{ t('host.uuLastRunPackagesSuffix', { count: uuStatus.last_run_packages }) }}
        </div>
        <div
          v-else
          class="mb-3 text-secondary small"
        >
          {{ t('host.uuNoRunRecorded') }}
        </div>

        <!-- Config form -->
        <div
          v-if="canRunApt && uuForm"
          class="row g-3 mb-3"
        >
          <!-- Enable toggle -->
          <div class="col-12">
            <label class="form-check form-switch">
              <input
                v-model="uuForm.enabled"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label fw-semibold">{{ t('host.uuEnabledBadge') }}</span>
            </label>
          </div>
          <!-- Config options (only meaningful when enabled) -->
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="uuForm.config.security_only"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">{{ t('host.uuSecurityOnlyLabel') }}</span>
            </label>
          </div>
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="uuForm.config.remove_unused"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">{{ t('host.uuRemoveUnusedLabel') }}</span>
            </label>
          </div>
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="uuForm.config.auto_reboot"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">{{ t('host.uuAutoRebootLabel') }}</span>
            </label>
          </div>
          <div
            v-if="uuForm.config.auto_reboot"
            class="col-md-6"
          >
            <label class="form-label small mb-1">{{ t('host.uuRebootTimeLabel') }}</label>
            <input
              v-model="uuForm.config.auto_reboot_time"
              type="time"
              class="form-control form-control-sm"
              style="max-width:120px"
            >
          </div>
          <!-- Actions -->
          <div class="col-12 d-flex gap-2">
            <button
              type="button"
              class="btn btn-sm btn-primary"
              :disabled="uuLoading === 'configure'"
              @click="$emit('uu-configure', uuForm)"
            >
              <span
                v-if="uuLoading === 'configure'"
                class="spinner-border spinner-border-sm me-1"
              />
              {{ t('host.uuSaveButton') }}
            </button>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              :disabled="!!uuLoading"
              @click="$emit('uu-run-now')"
            >
              <span
                v-if="uuLoading === 'run'"
                class="spinner-border spinner-border-sm me-1"
              />
              {{ t('host.uuRunNowButton') }}
            </button>
          </div>
        </div>

        <!-- Run history -->
        <div v-if="uuRuns && uuRuns.length > 0">
          <div class="fw-semibold small mb-2">
            {{ t('host.uuHistoryTitle') }}
          </div>
          <div class="table-responsive scroll-table">
            <table class="table table-sm table-vcenter">
              <thead>
                <tr>
                  <th>{{ t('host.dateColumn') }}</th>
                  <th>{{ t('host.uuPackagesColumn') }}</th>
                  <th>{{ t('host.statusColumn') }}</th>
                  <th>{{ t('host.uuLogsColumn') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="run in uuRuns"
                  :key="run.run_at"
                >
                  <td class="text-nowrap small">
                    {{ formatDate(run.run_at) }}
                  </td>
                  <td class="small">
                    <span
                      v-if="run.packages && run.packages.length"
                      :title="run.packages.join(', ')"
                    >
                      {{ run.packages.slice(0, 3).join(', ') }}
                      <span v-if="run.packages.length > 3">… (+{{ run.packages.length - 3 }})</span>
                    </span>
                    <span
                      v-else
                      class="text-secondary"
                    >{{ t('host.uuNonePackages') }}</span>
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="run.had_error ? 'bg-danger-lt text-danger' : 'bg-success-lt text-success'"
                    >{{ run.had_error ? t('common.error') : 'OK' }}</span>
                  </td>
                  <td>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      :title="t('host.viewLogsTooltip')"
                      :disabled="!run.log_snippet"
                      @click="$emit('uu-log', run)"
                    >
                      <IconList
                        :size="16"
                        class="icon icon-sm"
                      />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div
          v-else-if="uuRuns"
          class="text-secondary small"
        >
          {{ t('host.uuNoAutoUpgrades') }}
        </div>
      </div>

      <!-- No data yet -->
      <div
        v-else
        class="d-flex align-items-center gap-3 text-secondary small"
      >
        <span>{{ t('host.uuWaitingForData') }}</span>
        <button
          v-if="canRunApt"
          type="button"
          class="btn btn-sm btn-outline-primary"
          :disabled="uuLoading === 'install'"
          @click="$emit('uu-install')"
        >
          <span
            v-if="uuLoading === 'install'"
            class="spinner-border spinner-border-sm me-1"
          />
          {{ t('host.uuInstallButton') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconList } from '@tabler/icons-vue'
import dayjs from '../../utils/dayjs'
import CVEList from '../apt/CVEList.vue'
import AptPendingPackagesList from '../apt/AptPendingPackagesList.vue'

interface AptStatus {
  last_upgrade?: string
  pending_packages?: number
  last_update?: string
  cve_list?: string | unknown[]
  package_list?: string | unknown[]
  [key: string]: unknown
}

interface UUStatus {
  last_run_at?: string
  [key: string]: unknown
}

interface UURun {
  run_at: string
  packages?: string[]
  had_error?: boolean
  log_snippet?: string
  [key: string]: unknown
}

interface UUConfig {
  security_only?: boolean
  remove_unused?: boolean
  auto_reboot?: boolean
  auto_reboot_time?: string
}

interface UUForm {
  config: UUConfig
  [key: string]: unknown
}

defineEmits<{
  (e: 'run-apt-command', action: string): void
  (e: 'uu-install'): void
  (e: 'uu-configure', form: UUForm): void
  (e: 'uu-run-now'): void
  (e: 'uu-log', run: UURun): void
}>()

const props = withDefaults(defineProps<{
  aptStatus?: AptStatus | null
  canRunApt?: boolean
  aptCmdLoading?: string
  uuStatus?: UUStatus | null
  uuRuns?: UURun[] | null
  uuForm?: UUForm | null
  uuLoading?: string
}>(), {
  aptStatus: null,
  canRunApt: false,
  aptCmdLoading: '',
  uuStatus: null,
  uuRuns: null,
  uuForm: null,
  uuLoading: '',
})

const { t } = useI18n()

const pendingPackages = computed<string[]>(() => {
  const raw = props.aptStatus?.package_list
  if (!raw) return []
  try {
    const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
    return Array.isArray(parsed) ? (parsed as string[]) : []
  } catch {
    return []
  }
})

const lastUpgradeDate = computed(() => {
  const aptUpgrade = props.aptStatus?.last_upgrade
  if (aptUpgrade && aptUpgrade !== '0001-01-01T00:00:00Z') return aptUpgrade
  const uuUpgrade = props.uuStatus?.last_run_at
  if (uuUpgrade && uuUpgrade !== '0001-01-01T00:00:00Z') return uuUpgrade
  return null
})

function formatDate(date: string | null | undefined): string {
  if (!date || date === '0001-01-01T00:00:00Z') return t('common.never')
  return dayjs.utc(date).local().fromNow()
}
</script>


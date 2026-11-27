<template>
  <div class="card">
    <div class="card-header">
      <div>
        <h3 class="card-title">
          {{ t('monitoring.sla.title') }}
        </h3>
        <div class="card-subtitle">
          {{ t('monitoring.sla.subtitle') }}
        </div>
      </div>
    </div>
    <div
      v-if="error"
      class="card-body"
    >
      <div class="alert alert-danger mb-0">
        {{ error }}
      </div>
    </div>
    <div
      v-else-if="loading && !report"
      class="card-body"
    >
      <LoadingSkeleton
        variant="table"
        :lines="3"
      />
    </div>
    <div
      v-else-if="!report?.probes.length"
      class="card-body text-secondary"
    >
      {{ t('monitoring.sla.empty') }}
    </div>
    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('monitoring.sla.colService') }}</th>
            <th class="text-end">
              {{ t('monitoring.sla.colTarget') }}
            </th>
            <th
              v-for="days in report.window_days"
              :key="days"
              class="text-end"
            >
              {{ t('monitoring.sla.colWindow', { days }) }}
            </th>
            <th>{{ t('monitoring.sla.colBudget') }}</th>
            <th>{{ t('monitoring.sla.colCoverage') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="probe in report.probes"
            :key="probe.probe_id"
            :data-probe="probe.probe_id"
          >
            <td>
              <router-link :to="`/monitoring/probes/${probe.probe_id}`">
                {{ probe.name }}
              </router-link>
              <span
                v-if="!probe.enabled"
                class="badge bg-secondary-lt text-secondary ms-2"
              >{{ t('monitoring.sla.disabledBadge') }}</span>
            </td>
            <td class="text-end text-nowrap">
              {{ formatPercent(probe.slo_target, 3) }} %
            </td>
            <td
              v-for="w in probe.windows"
              :key="w.days"
              class="text-end text-nowrap"
            >
              <span
                v-if="w.uptime_percent != null"
                class="badge"
                :class="slaStatusClass(w.status)"
                :title="t(`monitoring.sla.${slaStatusKey(w.status)}`)"
              >{{ formatPercent(w.uptime_percent, 3) }} %</span>
              <span
                v-else
                class="text-secondary"
              >{{ t('monitoring.sla.noData') }}</span>
              <div
                v-if="w.uptime_percent != null && w.covered_days < w.days"
                class="small text-secondary"
                :title="t('monitoring.sla.coverageTitle', { covered: w.covered_days, days: w.days })"
              >
                {{ t('monitoring.sla.coverage', { covered: w.covered_days, days: w.days }) }}
              </div>
            </td>
            <td style="min-width: 10rem">
              <template v-if="budgetWindow(probe)?.budget_remaining_percent != null">
                <div
                  class="d-flex justify-content-between small"
                  :title="t('monitoring.sla.budgetTitle', { minutes: formatPercent(budgetWindow(probe)?.allowed_downtime_minutes, 1) })"
                >
                  <span>{{ t('monitoring.sla.budgetLeft', { pct: formatPercent(budgetWindow(probe)?.budget_remaining_percent, 0) }) }}</span>
                  <span
                    class="badge"
                    :class="slaStatusClass(budgetWindow(probe)?.status ?? '')"
                  >{{ t(`monitoring.sla.${slaStatusKey(budgetWindow(probe)?.status ?? '')}`) }}</span>
                </div>
                <div
                  class="progress progress-sm mt-1"
                  role="progressbar"
                  :aria-label="t('monitoring.sla.colBudget')"
                  :aria-valuenow="Math.round(budgetWindow(probe)?.budget_remaining_percent ?? 0)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                >
                  <div
                    class="progress-bar"
                    :class="budgetBarClass(budgetWindow(probe)?.status ?? '')"
                    :style="{ width: `${budgetWindow(probe)?.budget_remaining_percent ?? 0}%` }"
                  />
                </div>
              </template>
              <span
                v-else
                class="text-secondary"
              >{{ t('monitoring.sla.noData') }}</span>
            </td>
            <td class="text-nowrap small text-secondary">
              {{ probe.data_since ? t('monitoring.sla.dataSince', { date: formatDay(probe.data_since) }) : t('monitoring.sla.noDataYet') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="card-footer small text-secondary">
      {{ t('monitoring.sla.retentionNote') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { getApiErrorMessage } from '../../api'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import type { ProbeSLA, SLAReport, SLAWindow } from '../../types/uptime'
import { formatPercent, slaStatusClass, slaStatusKey } from '../../utils/sla'
import { localeTag } from '../../i18n'

const { t } = useI18n()

const report = ref<SLAReport | null>(null)
const loading = ref(false)
const error = ref('')

/** The error budget column reads the 30-day window, the usual SLA period. */
const BUDGET_WINDOW_DAYS = 30

function budgetWindow(probe: ProbeSLA): SLAWindow | undefined {
  return probe.windows.find((w) => w.days === BUDGET_WINDOW_DAYS)
}

function budgetBarClass(status: string): string {
  if (status === 'breached') return 'bg-danger'
  if (status === 'at_risk') return 'bg-warning'
  return 'bg-success'
}

/** data_since is a UTC day: format it as such, not shifted to local time. */
function formatDay(iso: string): string {
  return new Intl.DateTimeFormat(localeTag(), { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(iso))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const { data } = await api.getUptimeSLA()
    report.value = data
    error.value = ''
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, t('monitoring.sla.loadError'))
  } finally {
    loading.value = false
  }
}

onMounted(() => { void load() })

defineExpose({ reload: load })
</script>

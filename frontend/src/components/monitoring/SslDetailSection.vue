<template>
  <div>
    <PageRefreshBar
      v-if="!hideRefreshBar"
      v-model="autoRefresh"
      :label="t('monitoring.certTypeToggle')"
      :interval-sec="REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

    <div
      v-if="loading"
      class="row row-cards"
    >
      <div class="col-12 col-md-3">
        <LoadingSkeleton
          variant="kpi"
          :lines="4"
        />
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <template v-else-if="cert">
      <!-- KPI row -->
      <div class="row row-cards mb-3">
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                {{ t('monitoring.statusColumn') }}
              </div>
              <div
                class="h2 mb-0 mt-1"
                :class="statusColor"
              >
                {{ statusLabel }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                {{ t('monitoring.expirationLabel') }}
              </div>
              <div
                class="h2 mb-0 mt-1"
                :class="daysColor"
              >
                {{ daysLabel }}
              </div>
              <div class="text-secondary small">
                {{ cert.valid_to ? formatDate(cert.valid_to) : '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                {{ t('monitoring.validSinceLabel') }}
              </div>
              <div class="h3 mb-0 mt-1">
                {{ cert.valid_from ? formatDate(cert.valid_from) : '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                {{ t('monitoring.lastCheckedColumn') }}
              </div>
              <div class="h3 mb-0 mt-1">
                <RelativeTime
                  v-if="cert.last_checked_at"
                  :date="cert.last_checked_at"
                />
                <span
                  v-else
                  class="text-secondary"
                >{{ t('monitoring.neverLabel') }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Cert details -->
      <div class="row row-cards mb-3">
        <div class="col-md-6">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title mb-0">
                {{ t('monitoring.certDetailsTitle') }}
              </h3>
            </div>
            <div class="card-body">
              <div class="datagrid">
                <div class="datagrid-item">
                  <div class="datagrid-title">
                    {{ t('monitoring.subjectLabel') }}
                  </div>
                  <div class="datagrid-content text-break">
                    {{ shortDN(cert.subject) || '—' }}
                  </div>
                </div>
                <div class="datagrid-item">
                  <div class="datagrid-title">
                    {{ t('monitoring.issuerLabel') }}
                  </div>
                  <div class="datagrid-content text-break">
                    {{ shortDN(cert.issuer) || '—' }}
                  </div>
                </div>
                <div class="datagrid-item">
                  <div class="datagrid-title">
                    {{ t('monitoring.serialNumberLabel') }}
                  </div>
                  <div class="datagrid-content font-monospace small text-break">
                    {{ cert.serial_number || '—' }}
                  </div>
                </div>
                <div class="datagrid-item">
                  <div class="datagrid-title">
                    {{ t('monitoring.sanDnsLabel') }}
                  </div>
                  <div class="datagrid-content">
                    <template v-if="cert.dns_names && cert.dns_names.length">
                      <code
                        v-for="n in cert.dns_names"
                        :key="n"
                        class="me-1 small"
                      >{{ n }}</code>
                    </template>
                    <span
                      v-else
                      class="text-secondary"
                    >—</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div
          v-if="cert.last_error"
          class="col-md-6"
        >
          <div class="card h-100 border-danger">
            <div class="card-header text-danger">
              <h3 class="card-title mb-0">
                {{ t('monitoring.verificationErrorTitle') }}
              </h3>
            </div>
            <div class="card-body">
              <p class="text-danger mb-0">
                {{ cert.last_error }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- Renewal timeline -->
      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title mb-0">
            {{ t('monitoring.renewalHistoryTitle') }}
          </h3>
          <small class="text-secondary">{{ t('monitoring.versionsDetectedCount', { n: events.length }, events.length) }}</small>
        </div>

        <div
          v-if="loadingEvents"
          class="card-body"
        >
          <LoadingSkeleton variant="list" />
        </div>

        <div
          v-else-if="!events.length"
          class="card-body"
        >
          <EmptyState
            :title="t('monitoring.noRenewalsTitle')"
            :subtitle="t('monitoring.noRenewalsSubtitle')"
          />
        </div>

        <div
          v-else
          class="table-responsive scroll-table"
        >
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>{{ t('monitoring.detectedOnColumn') }}</th>
                <th>{{ t('monitoring.validFromColumn') }}</th>
                <th>{{ t('monitoring.validToColumn') }}</th>
                <th>{{ t('monitoring.durationColumn') }}</th>
                <th>{{ t('monitoring.issuerLabel') }}</th>
                <th>{{ t('monitoring.serialNumberLabel') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(ev, idx) in events"
                :key="ev.id"
              >
                <td class="text-secondary small">
                  <RelativeTime :date="ev.detected_at" />
                  <div class="text-muted small">
                    {{ formatDate(ev.detected_at) }}
                  </div>
                </td>
                <td class="text-secondary small">
                  {{ ev.valid_from ? formatDate(ev.valid_from) : '—' }}
                </td>
                <td class="text-secondary small">
                  <span :class="idx === 0 ? daysColor : ''">
                    {{ ev.valid_to ? formatDate(ev.valid_to) : '—' }}
                  </span>
                </td>
                <td class="text-secondary small">
                  {{ certDuration(ev.valid_from, ev.valid_to) }}
                </td>
                <td class="text-secondary small">
                  {{ shortDN(ev.issuer) || '—' }}
                </td>
                <td class="font-monospace small text-secondary text-break">
                  {{ ev.serial_number }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import RelativeTime from '../RelativeTime.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import { useSSLCertificateDetail } from '../../composables/useSSLCertificateDetail'
import type { SSLCertificate } from '../../types/ssl'

// hideRefreshBar/autoRefresh: set together by MonitoringHostDetailView when
// both a probe and a cert are configured, so the two sections share one
// PageRefreshBar instead of each rendering its own "last updated" + dot.
// Neither is passed by the standalone /monitoring/ssl/:id route, which keeps
// its own independent bar exactly as before.
const { t } = useI18n()

const props = defineProps<{
  certId: string
  hideRefreshBar?: boolean
  autoRefresh?: boolean
}>()
const emit = defineEmits<{
  (e: 'loaded', cert: SSLCertificate | null): void
  (e: 'update:autoRefresh', value: boolean): void
}>()

// Gated on hideRefreshBar, not on `props.autoRefresh === undefined`: Vue
// casts an optional `boolean` prop that isn't bound at all (every caller
// except MonitoringHostDetailView's merged-bar case) to `false`, not
// `undefined` — checking the raw value here would permanently freeze
// autoRefresh off on every standalone route. hideRefreshBar and autoRefresh
// are always set together by the one caller that uses either.
const autoRefreshOverride = props.hideRefreshBar ? computed({
  get: () => props.autoRefresh as boolean,
  set: (v) => emit('update:autoRefresh', v),
}) : undefined

const {
  cert,
  events,
  loading,
  loadingEvents,
  error,
  autoRefresh,
  lastUpdatedAt,
  REFRESH_SEC,
  formatDate,
  shortDN,
  certDuration,
  statusLabel,
  statusColor,
  daysColor,
  daysLabel,
} = useSSLCertificateDetail(props.certId, autoRefreshOverride)

watch(cert, (c) => emit('loaded', c), { immediate: true })

// Read by MonitoringHostDetailView's merged PageRefreshBar to compute the
// most-recent of this section's and UptimeDetailSection's own lastUpdatedAt.
defineExpose({ lastUpdatedAt })
</script>

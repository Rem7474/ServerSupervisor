<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="NPM"
      :interval-sec="NPM_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          {{ t('npm.dashboardBreadcrumb') }}
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ t('npm.nginxProxyManagerLabel') }}</span>
      </div>
      <h2 class="page-title">
        {{ t('npm.proxyHostsTitle') }}
      </h2>
    </div>

    <div
      v-if="expiringCerts.length"
      class="alert mb-3"
      :class="expiringCerts.some((c) => c.ssl_days_remaining <= 7) ? 'alert-danger' : 'alert-warning'"
    >
      <div class="fw-medium mb-1">
        <IconLock
          :size="16"
          class="icon me-1"
        />
        {{ t('npm.certificatesExpiringLabel', { n: expiringCerts.length }, expiringCerts.length) }}
      </div>
      <div class="d-flex flex-wrap gap-2">
        <router-link
          v-for="c in expiringCerts"
          :key="c.id"
          :to="`/monitoring/host/${c.id}`"
          class="badge text-decoration-none"
          :class="sslBadge(c.ssl_days_remaining)"
        >
          {{ c.domain_names[0] }} — {{ c.ssl_days_remaining }}{{ t('npm.daySuffix') }}
        </router-link>
      </div>
    </div>

    <div class="card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title mb-0">
          {{ t('npm.allProxyHostsTitle') }}
        </h3>
      </div>

      <div
        v-if="loading"
        class="card-body"
      >
        <LoadingSkeleton variant="table" />
      </div>

      <div
        v-else-if="loadError"
        class="card-body"
      >
        <div class="alert alert-danger mb-0">
          {{ loadError }}
        </div>
      </div>

      <div
        v-else-if="hosts.length === 0"
        class="card-body"
      >
        <EmptyState
          :title="t('npm.noProxyHostFoundTitle')"
          :subtitle="t('npm.configureNpmConnectionHint')"
          :cta-label="t('npm.settingsIntegrationsLink')"
          cta-to="/settings?tab=integrations"
        />
      </div>

      <div
        v-else
        class="table-responsive scroll-table"
      >
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>
                <SortableHeader
                  :label="t('npm.connectionColumn')"
                  :active="sortKey === 'connection_name'"
                  :direction="sortDir"
                  @toggle="toggleSort('connection_name')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('npm.domainColumn')"
                  :active="sortKey === 'domain'"
                  :direction="sortDir"
                  @toggle="toggleSort('domain')"
                />
              </th>
              <th>
                <SortableHeader
                  :label="t('npm.forwardColumn')"
                  :active="sortKey === 'forward'"
                  :direction="sortDir"
                  @toggle="toggleSort('forward')"
                />
              </th>
              <th
                class="text-center"
                :title="t('npm.toggleNpmHeaderTooltip')"
              >
                <SortableHeader
                  :label="t('npm.npmActiveColumn')"
                  :active="sortKey === 'npm_enabled'"
                  :direction="sortDir"
                  @toggle="toggleSort('npm_enabled')"
                />
              </th>
              <th
                class="text-center"
                :title="t('npm.monitoringHeaderTooltip')"
              >
                <div class="d-flex justify-content-center gap-3">
                  <SortableHeader
                    label="Uptime"
                    :active="sortKey === 'uptime_status'"
                    :direction="sortDir"
                    @toggle="toggleSort('uptime_status')"
                  />
                  <SortableHeader
                    label="SSL"
                    :active="sortKey === 'ssl_days_remaining'"
                    :direction="sortDir"
                    @toggle="toggleSort('ssl_days_remaining')"
                  />
                </div>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="h in sortedHosts"
              :key="h.id"
              :class="{ 'opacity-60': !h.npm_enabled, 'table-warning': needsAttention(h) }"
            >
              <td class="text-muted small">
                {{ h.connection_name }}
              </td>
              <td>
                <div class="d-flex align-items-center gap-1">
                  <IconAlertTriangle
                    v-if="needsAttention(h)"
                    :size="14"
                    class="text-warning flex-shrink-0"
                    :title="t('npm.blindSpotTooltip')"
                  />
                  <router-link
                    v-if="h.uptime_probe_id || h.ssl_certificate_id"
                    :to="`/monitoring/host/${h.id}`"
                    class="fw-medium text-decoration-none"
                    :title="t('npm.viewMonitoringTooltip')"
                  >
                    {{ h.domain_names[0] }}
                  </router-link>
                  <span
                    v-else
                    class="fw-medium"
                  >{{ h.domain_names[0] }}</span>
                </div>
                <div
                  v-if="h.domain_names.length > 1"
                  class="d-flex flex-wrap gap-1 mt-1"
                >
                  <span
                    v-for="d in h.domain_names.slice(1)"
                    :key="d"
                    class="badge bg-secondary-lt text-secondary"
                  >{{ d }}</span>
                </div>
              </td>
              <td class="text-muted small">
                {{ h.forward_host }}:{{ h.forward_port }}
              </td>

              <!-- Active in NPM — direct call to the NPM API -->
              <td class="text-center">
                <label class="form-check form-switch mb-0 d-inline-flex justify-content-center">
                  <input
                    class="form-check-input"
                    type="checkbox"
                    :checked="h.npm_enabled"
                    :disabled="togglingNPM[h.id]"
                    :title="t('npm.toggleNpmInputTooltip')"
                    @change="toggleNPM(h, ($event.target as HTMLInputElement).checked)"
                  >
                </label>
              </td>

              <!-- Monitoring — uptime + SSL sub-toggles and badges grouped
                   under one column instead of two, since they're both just
                   "is monitoring active for this proxy host" controls (see
                   ExposureDomainsPanel.vue's Availability column for the
                   same merge on the host exposure tab). Each sub-toggle
                   still applies independently (a proxy host can monitor
                   uptime only, SSL only, both, or neither). -->
              <td class="text-center">
                <div class="d-flex justify-content-center gap-3">
                  <div class="d-flex flex-column align-items-center gap-1">
                    <label class="form-check form-switch mb-0">
                      <input
                        class="form-check-input"
                        type="checkbox"
                        :checked="h.uptime_monitoring_enabled"
                        :disabled="toggling[h.id] || !h.npm_enabled"
                        :title="t('npm.enableUptimeMonitoringTooltip')"
                        @change="toggle(h, 'uptime_monitoring_enabled', ($event.target as HTMLInputElement).checked)"
                      >
                    </label>
                    <router-link
                      v-if="h.uptime_probe_id && h.uptime_status"
                      :to="`/monitoring/host/${h.id}`"
                      class="badge small text-decoration-none"
                      :class="uptimeBadge(h.uptime_status)"
                      :title="t('npm.viewUptimeProbeTooltip')"
                    >
                      {{ h.uptime_status }}
                      <span
                        v-if="h.uptime_last_latency_ms"
                        class="ms-1 opacity-75"
                      >{{ h.uptime_last_latency_ms }}ms</span>
                    </router-link>
                    <span
                      v-else-if="!h.uptime_probe_id"
                      class="text-muted"
                      style="font-size:0.7rem"
                    >—</span>
                  </div>
                  <div class="d-flex flex-column align-items-center gap-1">
                    <label class="form-check form-switch mb-0">
                      <input
                        class="form-check-input"
                        type="checkbox"
                        :checked="h.ssl_monitoring_enabled"
                        :disabled="toggling[h.id] || !h.ssl_enabled || !h.npm_enabled"
                        :title="!h.ssl_enabled ? t('npm.noSslUsedTooltip') : t('npm.enableSslMonitoringTooltip')"
                        @change="toggle(h, 'ssl_monitoring_enabled', ($event.target as HTMLInputElement).checked)"
                      >
                    </label>
                    <router-link
                      v-if="h.ssl_certificate_id && h.ssl_days_remaining !== null && h.ssl_days_remaining !== undefined"
                      :to="`/monitoring/host/${h.id}`"
                      class="badge small text-decoration-none"
                      :class="sslBadge(h.ssl_days_remaining)"
                      :title="t('npm.viewSslCertificateTooltip')"
                    >
                      {{ h.ssl_days_remaining }}{{ t('npm.daySuffix') }}
                    </router-link>
                    <span
                      v-else-if="!h.ssl_enabled"
                      class="text-muted"
                      style="font-size:0.7rem"
                    >HTTP</span>
                    <span
                      v-else-if="!h.ssl_certificate_id"
                      class="text-muted"
                      style="font-size:0.7rem"
                    >—</span>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="actionError"
        class="card-footer"
      >
        <span class="small text-danger">{{ actionError }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconLock, IconAlertTriangle } from '@tabler/icons-vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import { useNPM } from '../composables/useNPM'

const { t } = useI18n()

const {
  hosts,
  sortedHosts,
  sortKey,
  sortDir,
  loading,
  loadError,
  actionError,
  toggling,
  togglingNPM,
  autoRefresh,
  lastUpdatedAt,
  NPM_REFRESH_SEC,
  toggleNPM,
  toggle,
  toggleSort,
  needsAttention,
} = useNPM()

// Surfaces certificates about to expire in a banner instead of requiring a
// full table scan — sorted most-urgent first.
const expiringCerts = computed(() =>
  hosts.value
    .filter((h): h is typeof h & { ssl_days_remaining: number } =>
      !!h.ssl_certificate_id && h.ssl_days_remaining != null && h.ssl_days_remaining <= 30)
    .sort((a, b) => a.ssl_days_remaining - b.ssl_days_remaining)
)

function uptimeBadge(status: string): string {
  if (status === 'up') return 'bg-success-lt text-success'
  if (status === 'down') return 'bg-danger-lt text-danger'
  return 'bg-secondary-lt text-secondary'
}

function sslBadge(days: number): string {
  if (days <= 7) return 'bg-danger-lt text-danger'
  if (days <= 30) return 'bg-warning-lt text-warning'
  return 'bg-success-lt text-success'
}
</script>

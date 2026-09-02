<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/npm"
          class="text-decoration-none"
        >
          NPM
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ host?.domain_names?.[0] || t('host.monitoringFallback') }}</span>
      </div>
      <h2 class="page-title">
        {{ host?.domain_names?.[0] || '...' }}
      </h2>
      <div
        v-if="host"
        class="text-secondary"
      >
        <code>{{ host.forward_host }}:{{ host.forward_port }}</code>
        <router-link
          to="/npm"
          class="badge bg-azure-lt text-azure text-decoration-none ms-2"
        >
          {{ t('host.viewInNpm') }}
        </router-link>
      </div>
    </div>

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
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <template v-else-if="host">
      <div
        v-if="!host.uptime_probe_id && !host.ssl_certificate_id"
        class="alert alert-info"
      >
        {{ t('host.noActiveTrackingIntro') }}
        <router-link to="/npm">
          NPM
        </router-link>.
      </div>

      <!-- Merged refresh bar — when both a probe and a cert are configured,
           UptimeDetailSection/SslDetailSection each hide their own
           PageRefreshBar (which used to duplicate the "last updated" text +
           status dot side by side) in favor of this single one, showing the
           most recent of the two updates and pausing both together. -->
      <PageRefreshBar
        v-if="host.uptime_probe_id && host.ssl_certificate_id"
        v-model="autoRefresh"
        :label="t('host.monitoringFallback')"
        :interval-sec="PROBE_REFRESH_SEC"
        :last-updated-at="lastUpdatedAt"
      />

      <!-- Combined at-a-glance summary — both detail sections below already
           carry their own full KPI row, but each only covers its own domain;
           this is the one place on the page a NPM-linked host's uptime AND
           SSL status are visible together without scrolling past both. -->
      <div
        v-if="host.uptime_probe_id && host.ssl_certificate_id"
        class="row row-cards mb-4"
      >
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                {{ t('host.uptimeProbeLabel') }}
              </div>
              <div
                class="h2 mb-0 mt-1"
                :class="probeStatusColor"
              >
                {{ probeStatusLabel }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                {{ t('host.sslCertLabel') }}
              </div>
              <div
                class="h2 mb-0 mt-1"
                :class="certDaysColor"
              >
                {{ certDaysLabel }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="row row-cards">
        <div
          v-if="host.uptime_probe_id"
          :class="host.ssl_certificate_id ? 'col-lg-6' : 'col-12'"
        >
          <h3 class="mb-2">
            {{ t('host.availabilityHeading') }}
          </h3>
          <UptimeDetailSection
            ref="uptimeSectionRef"
            :probe-id="host.uptime_probe_id"
            :hide-refresh-bar="!!host.ssl_certificate_id"
            :auto-refresh="host.ssl_certificate_id ? autoRefresh : undefined"
            @update:auto-refresh="autoRefresh = $event"
            @loaded="probeLoaded = $event"
          />
        </div>

        <div
          v-if="host.ssl_certificate_id"
          :class="host.uptime_probe_id ? 'col-lg-6' : 'col-12'"
        >
          <h3 class="mb-2">
            {{ t('host.sslCertLabel') }}
          </h3>
          <SslDetailSection
            ref="sslSectionRef"
            :cert-id="host.ssl_certificate_id"
            :hide-refresh-bar="!!host.uptime_probe_id"
            :auto-refresh="host.uptime_probe_id ? autoRefresh : undefined"
            @update:auto-refresh="autoRefresh = $event"
            @loaded="certLoaded = $event"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useTemplateRef } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import UptimeDetailSection from '../components/monitoring/UptimeDetailSection.vue'
import SslDetailSection from '../components/monitoring/SslDetailSection.vue'
import { useMonitoringHostDetail } from '../composables/useMonitoringHostDetail'
import { PROBE_REFRESH_SEC } from '../composables/useUptimeProbeDetail'
import type { UptimeProbe } from '../types/uptime'
import type { SSLCertificate } from '../types/ssl'

const { t } = useI18n()
const { host, loading, error } = useMonitoringHostDetail()

// Shared pause/resume + "last updated" state for the merged PageRefreshBar,
// only used when both a probe and a cert are configured (see the template's
// v-if) — otherwise the single configured section keeps its own independent
// bar exactly as on the standalone /monitoring/probes|ssl/:id routes.
const uptimeSectionRef = useTemplateRef<InstanceType<typeof UptimeDetailSection>>('uptimeSectionRef')
const sslSectionRef = useTemplateRef<InstanceType<typeof SslDetailSection>>('sslSectionRef')
const autoRefresh = ref(true)
const lastUpdatedAt = computed<Date | null>(() => {
  const a = uptimeSectionRef.value?.lastUpdatedAt ?? null
  const b = sslSectionRef.value?.lastUpdatedAt ?? null
  if (a && b) return a > b ? a : b
  return a ?? b
})

// Fed by UptimeDetailSection/SslDetailSection's own `@loaded` emit — reused
// here rather than re-fetched, purely to drive the combined summary row
// above. Same status wording/thresholds as useUptimeProbes.ts/
// useSslCertificates.ts's probeStatusLabel/daysLabel, kept local (not
// imported from those) since calling either composable here would also
// trigger its own full probes/certs list fetch for a page that only ever
// needs this one probe and this one cert.
const probeLoaded = ref<UptimeProbe | null>(null)
const certLoaded = ref<SSLCertificate | null>(null)

const probeStatusLabel = computed(() => {
  const status = probeLoaded.value?.last_status
  if (status === 'up') return 'UP'
  if (status === 'down') return 'DOWN'
  return t('host.unknownStatus')
})
const probeStatusColor = computed(() => {
  const status = probeLoaded.value?.last_status
  if (status === 'up') return 'text-success'
  if (status === 'down') return 'text-danger'
  return 'text-secondary'
})

const certDaysLabel = computed(() => {
  const d = certLoaded.value?.days_remaining
  if (d == null) return t('host.unknownCert')
  if (d < 0) return t('host.expiredDays', { days: Math.abs(d) })
  return t('host.daysRemaining', { days: d }, d)
})
const certDaysColor = computed(() => {
  const d = certLoaded.value?.days_remaining
  if (d == null) return 'text-secondary'
  if (d < 0 || d <= 7) return 'text-danger'
  if (d <= 30) return 'text-warning'
  return 'text-success'
})
</script>

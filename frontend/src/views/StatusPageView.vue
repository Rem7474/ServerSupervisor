<template>
  <div class="page status-page">
    <div class="container container-narrow py-4 px-3">
      <header class="d-flex align-items-start justify-content-between gap-3 mb-4">
        <div class="min-w-0">
          <h1 class="h2 mb-1 text-break">
            {{ page?.title || t('monitoring.publicStatus.defaultTitle') }}
          </h1>
          <div
            v-if="page?.description"
            class="text-secondary text-break"
          >
            {{ page.description }}
          </div>
        </div>
        <LocaleSwitcher />
      </header>

      <div
        v-if="state === 'loading'"
        class="text-secondary"
        role="status"
      >
        {{ t('monitoring.publicStatus.loading') }}
      </div>

      <div
        v-else-if="state === 'unavailable'"
        class="empty"
      >
        <p class="empty-title">
          {{ t('monitoring.publicStatus.unavailableTitle') }}
        </p>
        <p class="empty-subtitle text-secondary">
          {{ t('monitoring.publicStatus.unavailableText') }}
        </p>
      </div>

      <template v-else>
        <div
          v-if="state === 'error'"
          class="alert alert-warning"
          role="alert"
        >
          {{ t('monitoring.publicStatus.loadError') }}
        </div>

        <template v-if="page">
          <div
            class="card mb-4 overall-banner"
            :class="`overall-${page.status}`"
            data-test="overall"
          >
            <div class="card-body d-flex align-items-center gap-2 fw-semibold">
              <span
                class="status-dot status-dot-animated"
                :class="dotClass(page.status)"
                aria-hidden="true"
              />
              {{ t(`monitoring.publicStatus.overall_${statusKey(page.status)}`) }}
            </div>
          </div>

          <div class="card mb-4">
            <div
              v-if="!page.services.length"
              class="card-body text-secondary"
            >
              {{ t('monitoring.publicStatus.noServices') }}
            </div>
            <div
              v-else
              class="list-group list-group-flush"
            >
              <section
                v-for="svc in page.services"
                :key="svc.name"
                class="list-group-item"
                data-test="service"
              >
                <div class="d-flex align-items-center justify-content-between gap-2 mb-2">
                  <h2 class="h4 mb-0 text-break">
                    {{ svc.name }}
                  </h2>
                  <span
                    class="badge text-nowrap"
                    :class="publicStatusClass(svc.status)"
                  >{{ t(`monitoring.publicStatus.status_${statusKey(svc.status)}`) }}</span>
                </div>
                <div
                  class="uptime-bar"
                  role="img"
                  :aria-label="t('monitoring.publicStatus.barLabel', { name: svc.name })"
                >
                  <span
                    v-for="day in svc.days"
                    :key="day.date"
                    class="uptime-day"
                    :class="uptimeDayClass(day.uptime_percent)"
                    :title="dayTitle(day.date, day.uptime_percent)"
                  />
                </div>
                <div class="d-flex justify-content-between small text-secondary mt-1">
                  <span>{{ t('monitoring.publicStatus.daysAgo') }}</span>
                  <span>
                    {{ svc.uptime_percent_90d != null
                      ? t('monitoring.publicStatus.uptime90', { pct: formatPercent(svc.uptime_percent_90d, 2) })
                      : t('monitoring.publicStatus.uptime90NoData') }}
                  </span>
                  <span>{{ t('monitoring.publicStatus.today') }}</span>
                </div>
              </section>
            </div>
          </div>

          <h2 class="h3">
            {{ t('monitoring.publicStatus.incidentsTitle') }}
          </h2>
          <div class="card mb-4">
            <div
              v-if="!page.incidents.length"
              class="card-body text-secondary"
            >
              {{ t('monitoring.publicStatus.noIncidents') }}
            </div>
            <ul
              v-else
              class="list-group list-group-flush"
            >
              <li
                v-for="inc in page.incidents"
                :key="`${inc.service}-${inc.started_at}`"
                class="list-group-item"
                data-test="incident"
              >
                <div class="d-flex align-items-center gap-2">
                  <span
                    class="status-dot"
                    :class="inc.resolved_at ? 'status-green' : 'status-red status-dot-animated'"
                    aria-hidden="true"
                  />
                  <span class="fw-semibold text-break">{{ t('monitoring.publicStatus.incidentOutage', { service: inc.service }) }}</span>
                </div>
                <div class="small text-secondary">
                  <template v-if="inc.resolved_at">
                    {{ t('monitoring.publicStatus.incidentResolved', {
                      start: formatInstant(inc.started_at),
                      end: formatInstant(inc.resolved_at),
                      duration: formatDuration(minutesBetween(inc.started_at, inc.resolved_at)),
                    }) }}
                  </template>
                  <template v-else>
                    {{ t('monitoring.publicStatus.incidentOngoing', { start: formatInstant(inc.started_at) }) }}
                  </template>
                </div>
              </li>
            </ul>
          </div>

          <footer class="d-flex flex-wrap justify-content-between gap-2 small text-secondary">
            <span>{{ t('monitoring.publicStatus.updatedAt', { time: formatInstant(page.generated_at) }) }}</span>
            <span>{{ t('monitoring.publicStatus.poweredBy') }}</span>
          </footer>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { isAxiosError } from 'axios'
import api from '../api'
import LocaleSwitcher from '../components/LocaleSwitcher.vue'
import type { PublicStatusPage } from '../types/uptime'
import { formatPercent, minutesBetween, publicStatusClass, uptimeDayClass } from '../utils/sla'
import { localeTag } from '../i18n'

const { t } = useI18n()

const REFRESH_MS = 60_000
const KNOWN_STATUSES = ['operational', 'partial_outage', 'major_outage', 'unknown']

const page = ref<PublicStatusPage | null>(null)
const state = ref<'loading' | 'ready' | 'unavailable' | 'error'>('loading')
let timer: ReturnType<typeof setInterval> | null = null

function statusKey(status: string): string {
  return KNOWN_STATUSES.includes(status) ? status : 'unknown'
}

function dotClass(status: string): string {
  switch (status) {
    case 'operational': return 'status-green'
    case 'partial_outage': return 'status-yellow'
    case 'major_outage': return 'status-red'
    default: return 'status-secondary'
  }
}

function formatInstant(iso: string): string {
  return new Intl.DateTimeFormat(localeTag(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

function formatDuration(minutes: number): string {
  if (minutes < 1) return t('monitoring.publicStatus.lessThanMinute')
  if (minutes < 60) return t('monitoring.publicStatus.minutes', { n: minutes })
  return t('monitoring.publicStatus.hoursMinutes', { h: Math.floor(minutes / 60), m: minutes % 60 })
}

/** Days are UTC calendar days: format them without a local-time shift. */
function dayTitle(date: string, pct: number | null | undefined): string {
  const label = new Intl.DateTimeFormat(localeTag(), { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(`${date}T00:00:00Z`))
  return pct == null
    ? t('monitoring.publicStatus.dayNoData', { date: label })
    : t('monitoring.publicStatus.dayTitle', { date: label, pct: formatPercent(pct, 2) })
}

async function load(): Promise<void> {
  try {
    const { data } = await api.getPublicStatus()
    page.value = data
    state.value = 'ready'
  } catch (e: unknown) {
    if (isAxiosError(e) && e.response?.status === 404) {
      page.value = null
      state.value = 'unavailable'
      return
    }
    // Keep showing the last good snapshot, if any, under a warning.
    state.value = 'error'
  }
}

onMounted(() => {
  void load()
  timer = setInterval(() => { void load() }, REFRESH_MS)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.status-page {
  min-height: 100vh;
}

.container-narrow {
  max-width: 54rem;
}

.min-w-0 {
  min-width: 0;
}

.overall-operational {
  border-left: 4px solid var(--tblr-success);
}

.overall-partial_outage {
  border-left: 4px solid var(--tblr-warning);
}

.overall-major_outage {
  border-left: 4px solid var(--tblr-danger);
}

.overall-unknown {
  border-left: 4px solid var(--tblr-secondary);
}

.uptime-bar {
  display: flex;
  gap: 1px;
  height: 2rem;
}

.uptime-day {
  flex: 1 1 0;
  min-width: 0;
  border-radius: 1px;
}

.uptime-day-ok {
  background: var(--tblr-success);
}

.uptime-day-warn {
  background: var(--tblr-warning);
}

.uptime-day-down {
  background: var(--tblr-danger);
}

.uptime-day-none {
  background: var(--tblr-border-color);
}
</style>

<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Certificat SSL"
      :interval-sec="REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/monitoring?tab=ssl"
          class="text-decoration-none"
        >
          Monitoring
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ cert?.name || 'Certificat' }}</span>
      </div>
      <h2 class="page-title">
        {{ cert?.name || '...' }}
      </h2>
      <div
        v-if="cert"
        class="text-secondary"
      >
        <code>{{ cert.host }}:{{ cert.port }}</code>
        <span
          v-if="cert.server_name"
          class="ms-2 text-muted small"
        >(SNI: {{ cert.server_name }})</span>
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
                Statut
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
                Expiration
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
                Valide depuis
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
                Dernière vérification
              </div>
              <div class="h3 mb-0 mt-1">
                <RelativeTime
                  v-if="cert.last_checked_at"
                  :date="cert.last_checked_at"
                />
                <span
                  v-else
                  class="text-secondary"
                >Jamais</span>
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
                Détails du certificat
              </h3>
            </div>
            <div class="card-body">
              <dl class="row mb-0">
                <dt class="col-5 text-secondary">
                  Sujet
                </dt>
                <dd class="col-7 mb-2 text-break">
                  {{ shortDN(cert.subject) || '—' }}
                </dd>
                <dt class="col-5 text-secondary">
                  Émetteur
                </dt>
                <dd class="col-7 mb-2 text-break">
                  {{ shortDN(cert.issuer) || '—' }}
                </dd>
                <dt class="col-5 text-secondary">
                  Numéro de série
                </dt>
                <dd class="col-7 mb-2 font-monospace small text-break">
                  {{ cert.serial_number || '—' }}
                </dd>
                <dt class="col-5 text-secondary">
                  SAN / DNS
                </dt>
                <dd class="col-7 mb-0">
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
                </dd>
              </dl>
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
                Erreur de vérification
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
            Historique des renouvellements
          </h3>
          <small class="text-secondary">{{ events.length }} version(s) détectée(s)</small>
        </div>

        <div
          v-if="loadingEvents"
          class="card-body py-4 text-center text-secondary"
        >
          Chargement…
        </div>

        <div
          v-else-if="!events.length"
          class="card-body py-4 text-center text-secondary"
        >
          Aucun renouvellement enregistré. Les changements de certificat seront tracés lors des prochaines vérifications.
        </div>

        <div
          v-else
          class="table-responsive"
        >
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Détecté le</th>
                <th>Valide du</th>
                <th>Valide au</th>
                <th>Durée</th>
                <th>Émetteur</th>
                <th>Numéro de série</th>
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import type { SSLCertificate, SSLCertificateEvent } from '../types/ssl'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import RelativeTime from '../components/RelativeTime.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import dayjs from '../utils/dayjs'

type SSLCert = SSLCertificate

const route = useRoute()
const certId = route.params.id as string

const cert = ref<SSLCert | null>(null)
const events = ref<SSLCertificateEvent[]>([])
const loading = ref(false)
const loadingEvents = ref(false)
const error = ref('')
const autoRefresh = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
const REFRESH_SEC = 60

function formatDate(ts: string | undefined | null): string {
  return ts ? dayjs(ts).format('YYYY-MM-DD') : '—'
}

function shortDN(s: string | undefined): string {
  if (!s) return ''
  const cn = /CN=([^,]+)/.exec(s)
  return cn ? cn[1] : s.split(',')[0]
}

function certDuration(from: string | undefined, to: string | undefined): string {
  if (!from || !to) return '—'
  const days = dayjs(to).diff(dayjs(from), 'day')
  if (days >= 365) return `${Math.round(days / 365 * 10) / 10} ans`
  return `${days}j`
}

const statusLabel = computed(() => {
  if (!cert.value) return ''
  const d = cert.value.days_remaining
  if (d == null) return 'Inconnu'
  if (d < 0) return 'Expiré'
  if (d <= 7) return 'Critique'
  if (d <= 30) return 'Attention'
  return 'Valide'
})

const statusColor = computed(() => {
  if (!cert.value) return ''
  const d = cert.value.days_remaining
  if (d == null) return 'text-secondary'
  if (d < 0) return 'text-danger'
  if (d <= 7) return 'text-danger'
  if (d <= 30) return 'text-warning'
  return 'text-success'
})

const daysColor = computed(() => statusColor.value)

const daysLabel = computed(() => {
  if (!cert.value) return ''
  const d = cert.value.days_remaining
  if (d == null) return 'Inconnu'
  if (d < 0) return `Expiré (${Math.abs(d)}j)`
  return `${d} jour${d > 1 ? 's' : ''}`
})

async function fetchCert(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.getSSLCertificate(certId)
    cert.value = data
    lastUpdatedAt.value = new Date()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Impossible de charger le certificat'
  } finally {
    loading.value = false
  }
}

async function fetchEvents(): Promise<void> {
  loadingEvents.value = true
  try {
    const { data } = await api.getSSLCertificateHistory(certId)
    events.value = data?.events || []
  } catch {
    // non-fatal — history may be empty
  } finally {
    loadingEvents.value = false
  }
}

async function fetchAll(): Promise<void> {
  await Promise.all([fetchCert(), fetchEvents()])
}

let refreshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  fetchAll()
  refreshTimer = setInterval(() => { if (autoRefresh.value) fetchAll() }, REFRESH_SEC * 1000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

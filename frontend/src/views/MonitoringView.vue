<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Monitoring"
      :interval-sec="REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Monitoring
          </h2>
          <div class="text-muted">
            Sondes HTTP/TCP synthétiques et suivi des certificats SSL/TLS.
          </div>
        </div>
        <button
          v-if="auth.role === 'admin'"
          type="button"
          class="btn btn-primary"
          @click="tab === 'ssl' ? openCreateCert() : openCreateProbe()"
        >
          {{ tab === 'ssl' ? '+ Ajouter un certificat' : '+ Nouvelle sonde' }}
        </button>
      </div>
    </div>

    <!-- Tabs -->
    <div class="mb-3">
      <ul class="nav nav-tabs">
        <li class="nav-item">
          <button
            type="button"
            :class="['nav-link', tab === 'uptime' ? 'active' : '']"
            @click="setTab('uptime')"
          >
            <IconActivity
              :size="16"
              class="icon icon-sm me-1"
            />
            Sondes uptime
            <span
              v-if="downCount > 0"
              class="badge bg-red text-white ms-1"
            >{{ downCount }}</span>
          </button>
        </li>
        <li class="nav-item">
          <button
            type="button"
            :class="['nav-link', tab === 'ssl' ? 'active' : '']"
            @click="setTab('ssl')"
          >
            <IconLock
              :size="16"
              class="icon icon-sm me-1"
            />
            Certificats SSL
            <span
              v-if="expiringCount > 0"
              class="badge bg-yellow text-white ms-1"
            >{{ expiringCount }}</span>
          </button>
        </li>
      </ul>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <!-- ===== UPTIME TAB ===== -->
    <template v-if="tab === 'uptime'">
      <div
        v-if="loadingProbes && !probes.length"
        class="row row-cards"
      >
        <div class="col-12">
          <LoadingSkeleton
            variant="table"
            :lines="5"
          />
        </div>
      </div>

      <EmptyState
        v-else-if="!probes.length"
        title="Aucune sonde configurée"
        subtitle="Créez votre première sonde HTTP ou TCP pour surveiller un service."
        :cta-label="auth.role === 'admin' ? 'Nouvelle sonde' : ''"
        @cta="openCreateProbe"
      />

      <div
        v-else
        class="card"
      >
        <div
          v-if="loadingProbes && probes.length > 0"
          class="card-header py-2"
        >
          <div class="d-flex align-items-center gap-2 text-secondary small">
            <div class="spinner-border spinner-border-sm" />
            Actualisation…
          </div>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleProbeSort('name')"
                  >
                    Sonde <span class="sort-icon">{{ probeSortIcon('name') }}</span>
                  </button>
                </th>
                <th>Cible</th>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleProbeSort('status')"
                  >
                    Statut <span class="sort-icon">{{ probeSortIcon('status') }}</span>
                  </button>
                </th>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleProbeSort('uptime')"
                  >
                    Uptime 24h <span class="sort-icon">{{ probeSortIcon('uptime') }}</span>
                  </button>
                </th>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleProbeSort('latency')"
                  >
                    Latence <span class="sort-icon">{{ probeSortIcon('latency') }}</span>
                  </button>
                </th>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleProbeSort('last_checked')"
                  >
                    Dernière vérification <span class="sort-icon">{{ probeSortIcon('last_checked') }}</span>
                  </button>
                </th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="p in pagedProbes"
                :key="p.id"
              >
                <td>
                  <router-link
                    :to="`/monitoring/probes/${p.id}`"
                    class="fw-semibold text-decoration-none"
                  >
                    {{ p.name }}
                  </router-link>
                  <div class="text-secondary small">
                    {{ p.type.toUpperCase() }} · {{ p.interval_sec }}s
                  </div>
                </td>
                <td class="text-secondary">
                  <code>{{ p.target }}</code>
                </td>
                <td>
                  <span :class="['badge', probeBadge(p)]">
                    {{ probeStatusLabel(p) }}
                  </span>
                  <span
                    v-if="!p.enabled"
                    class="badge bg-secondary-lt text-secondary ms-1"
                  >désactivée</span>
                </td>
                <td>
                  <template v-if="probeStats[p.id]">
                    <span :class="['badge', uptimeBadgeClass(probeStats[p.id].uptime_percent)]">
                      {{ probeStats[p.id].uptime_percent.toFixed(1) }}%
                    </span>
                  </template>
                  <span
                    v-else
                    class="text-secondary small"
                  >—</span>
                </td>
                <td>
                  <template v-if="p.last_latency_ms != null && p.last_status === 'up'">
                    {{ p.last_latency_ms }} ms
                  </template>
                  <span
                    v-else
                    class="text-secondary"
                  >—</span>
                </td>
                <td class="text-secondary small">
                  <RelativeTime
                    v-if="p.last_checked_at"
                    :date="p.last_checked_at"
                  />
                  <span
                    v-else
                    class="text-secondary"
                  >Jamais</span>
                </td>
                <td class="text-end">
                  <div class="btn-list">
                    <button
                      v-if="auth.role === 'admin'"
                      type="button"
                      class="btn btn-sm btn-outline-secondary"
                      :disabled="checkingProbeId === p.id"
                      @click="checkProbeNow(p)"
                    >
                      {{ checkingProbeId === p.id ? '...' : 'Vérifier' }}
                    </button>
                    <button
                      v-if="auth.role === 'admin'"
                      type="button"
                      class="btn btn-sm btn-outline-secondary"
                      @click="openEditProbe(p)"
                    >
                      Modifier
                    </button>
                    <button
                      v-if="auth.role === 'admin'"
                      type="button"
                      class="btn btn-sm btn-outline-danger"
                      @click="confirmDeleteProbe(p)"
                    >
                      Supprimer
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div
          v-if="probeTotalPages > 1"
          class="card-footer d-flex align-items-center justify-content-between"
        >
          <div class="text-secondary small">
            {{ (probePage - 1) * PAGE_SIZE + 1 }}–{{ Math.min(probePage * PAGE_SIZE, probes.length) }} sur {{ probes.length }} sondes
          </div>
          <PaginationNav
            :current-page="probePage"
            :total-pages="probeTotalPages"
            @select="setProbesPage"
          />
        </div>
      </div>
    </template>

    <!-- ===== SSL TAB ===== -->
    <template v-if="tab === 'ssl'">
      <div
        v-if="loadingCerts && !certs.length"
        class="row row-cards"
      >
        <div class="col-12">
          <LoadingSkeleton
            variant="table"
            :lines="5"
          />
        </div>
      </div>

      <EmptyState
        v-else-if="!certs.length"
        title="Aucun certificat surveillé"
        subtitle="Ajoutez un domaine pour suivre l'expiration de son certificat TLS."
        :cta-label="auth.role === 'admin' ? 'Ajouter un certificat' : ''"
        @cta="openCreateCert"
      />

      <div
        v-else
        class="card"
      >
        <div
          v-if="loadingCerts && certs.length > 0"
          class="card-header py-2"
        >
          <div class="d-flex align-items-center gap-2 text-secondary small">
            <div class="spinner-border spinner-border-sm" />
            Actualisation…
          </div>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleCertSort('name')"
                  >
                    Nom <span class="sort-icon">{{ certSortIcon('name') }}</span>
                  </button>
                </th>
                <th>Endpoint</th>
                <th>Émetteur</th>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleCertSort('expiration')"
                  >
                    Expiration <span class="sort-icon">{{ certSortIcon('expiration') }}</span>
                  </button>
                </th>
                <th class="text-nowrap">
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleCertSort('days')"
                  >
                    Jours restants <span class="sort-icon">{{ certSortIcon('days') }}</span>
                  </button>
                </th>
                <th>
                  <button
                    type="button"
                    class="btn-sort"
                    @click="toggleCertSort('last_checked')"
                  >
                    Dernière vérification <span class="sort-icon">{{ certSortIcon('last_checked') }}</span>
                  </button>
                </th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="c in pagedCerts"
                :key="c.id"
              >
                <td>
                  <router-link
                    :to="`/monitoring/ssl/${c.id}`"
                    class="fw-semibold text-decoration-none"
                  >
                    {{ c.name }}
                  </router-link>
                  <span
                    v-if="!c.enabled"
                    class="badge bg-secondary-lt text-secondary ms-1"
                  >désactivé</span>
                </td>
                <td class="text-secondary">
                  <code>{{ c.host }}:{{ c.port }}</code>
                </td>
                <td class="text-secondary small">
                  {{ shortIssuer(c.issuer) || '—' }}
                </td>
                <td class="text-secondary small">
                  {{ c.valid_to ? formatDate(c.valid_to) : '—' }}
                </td>
                <td>
                  <span :class="['badge', daysBadge(c.days_remaining)]">
                    {{ daysLabel(c.days_remaining) }}
                  </span>
                </td>
                <td class="text-secondary small">
                  <RelativeTime
                    v-if="c.last_checked_at"
                    :date="c.last_checked_at"
                  />
                  <span
                    v-else
                    class="text-secondary"
                  >Jamais</span>
                  <div
                    v-if="c.last_error"
                    class="text-danger small"
                    :title="c.last_error"
                  >
                    {{ c.last_error.length > 40 ? c.last_error.slice(0, 40) + '...' : c.last_error }}
                  </div>
                </td>
                <td class="text-end">
                  <div class="btn-list">
                    <button
                      v-if="auth.role === 'admin'"
                      type="button"
                      class="btn btn-sm btn-outline-secondary"
                      :disabled="checkingCertId === c.id"
                      @click="checkCertNow(c)"
                    >
                      {{ checkingCertId === c.id ? '...' : 'Vérifier' }}
                    </button>
                    <button
                      v-if="auth.role === 'admin'"
                      type="button"
                      class="btn btn-sm btn-outline-secondary"
                      @click="openEditCert(c)"
                    >
                      Modifier
                    </button>
                    <button
                      v-if="auth.role === 'admin'"
                      type="button"
                      class="btn btn-sm btn-outline-danger"
                      @click="confirmDeleteCert(c)"
                    >
                      Supprimer
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div
          v-if="certTotalPages > 1"
          class="card-footer d-flex align-items-center justify-content-between"
        >
          <div class="text-secondary small">
            {{ (certPage - 1) * PAGE_SIZE + 1 }}–{{ Math.min(certPage * PAGE_SIZE, certs.length) }} sur {{ certs.length }} certificats
          </div>
          <PaginationNav
            :current-page="certPage"
            :total-pages="certTotalPages"
            @select="setCertPage"
          />
        </div>
      </div>
    </template>

    <!-- ===== MODAL SONDE ===== -->
    <div
      v-if="probeModalOpen"
      class="modal modal-blur fade show"
      style="display:block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ probeForm.id ? 'Modifier la sonde' : 'Nouvelle sonde' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              :disabled="savingProbe"
              @click="closeProbeModal"
            />
          </div>
          <form @submit.prevent="saveProbe">
            <div class="modal-body">
              <div
                v-if="probeFormError"
                class="alert alert-danger"
              >
                {{ probeFormError }}
              </div>
              <div class="row g-3">
                <div class="col-md-7">
                  <label class="form-label required">Nom</label>
                  <input
                    v-model="probeForm.name"
                    type="text"
                    class="form-control"
                    placeholder="Ex: API prod"
                    required
                  >
                </div>
                <div class="col-md-5">
                  <label class="form-label required">Type</label>
                  <select
                    v-model="probeForm.type"
                    class="form-select"
                  >
                    <option value="http">
                      HTTP/HTTPS
                    </option>
                    <option value="tcp">
                      TCP
                    </option>
                  </select>
                </div>
                <div class="col-12">
                  <label class="form-label required">{{ probeForm.type === 'http' ? 'URL' : 'host:port' }}</label>
                  <input
                    v-model="probeForm.target"
                    type="text"
                    class="form-control"
                    :placeholder="probeForm.type === 'http' ? 'https://example.com/health' : 'example.com:443'"
                    required
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label">Intervalle (sec)</label>
                  <input
                    v-model.number="probeForm.interval_sec"
                    type="number"
                    min="10"
                    class="form-control"
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label">Timeout (sec)</label>
                  <input
                    v-model.number="probeForm.timeout_sec"
                    type="number"
                    min="1"
                    max="60"
                    class="form-control"
                  >
                </div>
                <template v-if="probeForm.type === 'http'">
                  <div class="col-md-4">
                    <label class="form-label">Statut HTTP attendu</label>
                    <input
                      v-model.number="probeForm.expected_status"
                      type="number"
                      min="100"
                      max="599"
                      class="form-control"
                    >
                  </div>
                  <div class="col-12">
                    <label class="form-label">Regex corps attendu (optionnel)</label>
                    <input
                      v-model="probeForm.expected_body_regex"
                      type="text"
                      class="form-control"
                      placeholder="Ex: &quot;status&quot;:\s*&quot;ok&quot;"
                    >
                  </div>
                  <div class="col-md-6">
                    <label class="form-check">
                      <input
                        v-model="probeForm.follow_redirects"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Suivre les redirections</span>
                    </label>
                  </div>
                  <div class="col-md-6">
                    <label class="form-check">
                      <input
                        v-model="probeForm.verify_tls"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Vérifier le certificat TLS</span>
                    </label>
                  </div>
                </template>
                <div class="col-12">
                  <label class="form-check">
                    <input
                      v-model="probeForm.enabled"
                      type="checkbox"
                      class="form-check-input"
                    >
                    <span class="form-check-label">Activée</span>
                  </label>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn link-secondary"
                :disabled="savingProbe"
                @click="closeProbeModal"
              >
                Annuler
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="savingProbe"
              >
                {{ savingProbe ? 'Enregistrement...' : 'Enregistrer' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
    <div
      v-if="probeModalOpen"
      class="modal-backdrop fade show"
    />

    <!-- ===== MODAL CERTIFICAT ===== -->
    <div
      v-if="certModalOpen"
      class="modal modal-blur fade show"
      style="display:block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ certForm.id ? 'Modifier le certificat' : 'Nouveau certificat' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              :disabled="savingCert"
              @click="closeCertModal"
            />
          </div>
          <form @submit.prevent="saveCert">
            <div class="modal-body">
              <div
                v-if="certFormError"
                class="alert alert-danger"
              >
                {{ certFormError }}
              </div>
              <div class="mb-3">
                <label class="form-label required">Nom</label>
                <input
                  v-model="certForm.name"
                  type="text"
                  class="form-control"
                  placeholder="Ex: api.example.com"
                  required
                >
              </div>
              <div class="row g-3">
                <div class="col-md-8">
                  <label class="form-label required">Hôte</label>
                  <input
                    v-model="certForm.host"
                    type="text"
                    class="form-control"
                    placeholder="api.example.com"
                    required
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label required">Port</label>
                  <input
                    v-model.number="certForm.port"
                    type="number"
                    min="1"
                    max="65535"
                    class="form-control"
                  >
                </div>
                <div class="col-12">
                  <label class="form-label">SNI (override, optionnel)</label>
                  <input
                    v-model="certForm.server_name"
                    type="text"
                    class="form-control"
                    placeholder="Laisser vide pour utiliser l'hôte"
                  >
                </div>
                <div class="col-12">
                  <label class="form-check">
                    <input
                      v-model="certForm.enabled"
                      type="checkbox"
                      class="form-check-input"
                    >
                    <span class="form-check-label">Activé</span>
                  </label>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn link-secondary"
                :disabled="savingCert"
                @click="closeCertModal"
              >
                Annuler
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="savingCert"
              >
                {{ savingCert ? 'Enregistrement...' : 'Enregistrer' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
    <div
      v-if="certModalOpen"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { IconActivity, IconLock } from '@tabler/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api'
import type { UptimeProbe } from '../types/uptime'
import type { SSLCertificate } from '../types/ssl'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import RelativeTime from '../components/RelativeTime.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import PaginationNav from '../components/PaginationNav.vue'
import dayjs from '../utils/dayjs'
import { usePagination } from '../composables/usePagination'

type Probe = UptimeProbe
type SSLCert = SSLCertificate

interface ProbeForm {
  id: string
  name: string
  type: string
  target: string
  interval_sec: number
  timeout_sec: number
  expected_status: number
  expected_body_regex: string
  follow_redirects: boolean
  verify_tls: boolean
  enabled: boolean
}

interface CertForm {
  id: string
  name: string
  host: string
  port: number
  server_name: string
  enabled: boolean
}

const auth = useAuthStore()
const dialog = useConfirmDialog()
const route = useRoute()
const router = useRouter()

const REFRESH_SEC = 30
const autoRefresh = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
const error = ref('')

// ── tab ───────────────────────────────────────────────────────────────────────
type Tab = 'uptime' | 'ssl'
const tab = ref<Tab>((route.query.tab as Tab) === 'ssl' ? 'ssl' : 'uptime')

function setTab(t: Tab) {
  tab.value = t
  router.replace({ query: t !== 'uptime' ? { tab: t } : {} })
}

watch(() => route.query.tab, (v) => {
  tab.value = (v as Tab) === 'ssl' ? 'ssl' : 'uptime'
})

// ── Uptime ────────────────────────────────────────────────────────────────────
const probes = ref<Probe[]>([])
const loadingProbes = ref(false)
const probeStats = ref<Record<string, { uptime_percent: number }>>({})
const checkingProbeId = ref('')

const downCount = computed(() => probes.value.filter((p) => p.last_status === 'down').length)

type ProbeCol = 'name' | 'status' | 'uptime' | 'latency' | 'last_checked'
const probeSort = ref<{ col: ProbeCol; dir: 'asc' | 'desc' }>({ col: 'status', dir: 'asc' })

function toggleProbeSort(col: ProbeCol): void {
  if (probeSort.value.col === col) {
    probeSort.value = { col, dir: probeSort.value.dir === 'asc' ? 'desc' : 'asc' }
  } else {
    probeSort.value = { col, dir: 'asc' }
  }
}

function probeSortIcon(col: ProbeCol): string {
  if (probeSort.value.col !== col) return '⇅'
  return probeSort.value.dir === 'asc' ? '▲' : '▼'
}

const sortedProbes = computed(() => {
  const arr = [...probes.value]
  const { col, dir } = probeSort.value
  const m = dir === 'asc' ? 1 : -1
  arr.sort((a, b) => {
    switch (col) {
      case 'name': return m * a.name.localeCompare(b.name)
      case 'status': {
        const rank = (p: Probe) => p.last_status === 'down' ? 0 : p.last_status === 'up' ? 1 : 2
        return m * (rank(a) - rank(b))
      }
      case 'uptime': {
        const ua = probeStats.value[a.id]?.uptime_percent ?? -1
        const ub = probeStats.value[b.id]?.uptime_percent ?? -1
        return m * (ua - ub)
      }
      case 'latency': {
        const la = a.last_status === 'up' && a.last_latency_ms != null ? a.last_latency_ms : Infinity
        const lb = b.last_status === 'up' && b.last_latency_ms != null ? b.last_latency_ms : Infinity
        return m * (la - lb)
      }
      case 'last_checked': {
        const ta = a.last_checked_at ? new Date(a.last_checked_at).getTime() : 0
        const tb = b.last_checked_at ? new Date(b.last_checked_at).getTime() : 0
        return m * (ta - tb)
      }
    }
    return 0
  })
  return arr
})

function probeBadge(p: Probe): string {
  if (!p.enabled) return 'bg-secondary-lt text-secondary'
  if (p.last_status === 'up') return 'bg-green-lt text-green'
  if (p.last_status === 'down') return 'bg-red-lt text-red'
  return 'bg-secondary-lt text-secondary'
}

function probeStatusLabel(p: Probe): string {
  if (p.last_status === 'up') return 'UP'
  if (p.last_status === 'down') return 'DOWN'
  return 'Inconnue'
}

function uptimeBadgeClass(pct: number): string {
  if (pct >= 99) return 'bg-green-lt text-green'
  if (pct >= 95) return 'bg-yellow-lt text-yellow'
  return 'bg-red-lt text-red'
}

async function fetchProbes(): Promise<void> {
  loadingProbes.value = true
  try {
    const { data } = await api.getUptimeProbes()
    probes.value = data?.probes || []
    lastUpdatedAt.value = new Date()
    fetchAllProbeStats()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error
      || (e as { message?: string })?.message || 'Impossible de charger les sondes'
  } finally {
    loadingProbes.value = false
  }
}

async function fetchAllProbeStats(): Promise<void> {
  const results = await Promise.allSettled(
    probes.value.map((p) => api.getUptimeStats(p.id, 24).then((r) => ({ id: p.id, data: r.data })))
  )
  const map: Record<string, { uptime_percent: number }> = {}
  for (const r of results) {
    if (r.status === 'fulfilled') {
      map[r.value.id] = { uptime_percent: r.value.data?.uptime_percent ?? 0 }
    }
  }
  probeStats.value = map
}

async function checkProbeNow(p: Probe): Promise<void> {
  checkingProbeId.value = p.id
  try {
    await api.checkUptimeProbeNow(p.id)
    await fetchProbes()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Échec de la vérification'
  } finally {
    checkingProbeId.value = ''
  }
}

// probe form
const probeModalOpen = ref(false)
const savingProbe = ref(false)
const probeFormError = ref('')
const probeForm = ref<ProbeForm>(emptyProbeForm())

function emptyProbeForm(): ProbeForm {
  return { id: '', name: '', type: 'http', target: '', interval_sec: 60, timeout_sec: 10,
    expected_status: 200, expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true }
}

function openCreateProbe(): void {
  probeForm.value = emptyProbeForm()
  probeFormError.value = ''
  probeModalOpen.value = true
}

function openEditProbe(p: Probe): void {
  probeForm.value = {
    id: p.id, name: p.name, type: p.type, target: p.target,
    interval_sec: p.interval_sec, timeout_sec: p.timeout_sec,
    expected_status: p.expected_status, expected_body_regex: p.expected_body_regex || '',
    follow_redirects: p.follow_redirects, verify_tls: p.verify_tls, enabled: p.enabled,
  }
  probeFormError.value = ''
  probeModalOpen.value = true
}

function closeProbeModal(): void {
  probeModalOpen.value = false
  savingProbe.value = false
}

async function saveProbe(): Promise<void> {
  savingProbe.value = true
  probeFormError.value = ''
  try {
    const { id: _id, ...body } = probeForm.value
    if (probeForm.value.id) {
      await api.updateUptimeProbe(probeForm.value.id, body)
    } else {
      await api.createUptimeProbe(body)
    }
    closeProbeModal()
    await fetchProbes()
  } catch (e: unknown) {
    probeFormError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Erreur lors de l\'enregistrement'
  } finally {
    savingProbe.value = false
  }
}

async function confirmDeleteProbe(p: Probe): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Supprimer la sonde ?',
    message: `Cette action supprimera "${p.name}" et tout son historique.`,
    okLabel: 'Supprimer',
    destructive: true,
  })
  if (!ok) return
  try {
    await api.deleteUptimeProbe(p.id)
    await fetchProbes()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Suppression impossible'
  }
}

// ── SSL ───────────────────────────────────────────────────────────────────────
const certs = ref<SSLCert[]>([])
const loadingCerts = ref(false)
const checkingCertId = ref('')

const expiringCount = computed(() => certs.value.filter((c) => {
  const d = c.days_remaining
  return d != null && d <= 30
}).length)

type CertCol = 'name' | 'expiration' | 'days' | 'last_checked'
const certSort = ref<{ col: CertCol; dir: 'asc' | 'desc' }>({ col: 'days', dir: 'asc' })

function toggleCertSort(col: CertCol): void {
  if (certSort.value.col === col) {
    certSort.value = { col, dir: certSort.value.dir === 'asc' ? 'desc' : 'asc' }
  } else {
    certSort.value = { col, dir: 'asc' }
  }
}

function certSortIcon(col: CertCol): string {
  if (certSort.value.col !== col) return '⇅'
  return certSort.value.dir === 'asc' ? '▲' : '▼'
}

const sortedCerts = computed(() => {
  const arr = [...certs.value]
  const { col, dir } = certSort.value
  const m = dir === 'asc' ? 1 : -1
  arr.sort((a, b) => {
    switch (col) {
      case 'name': return m * a.name.localeCompare(b.name)
      case 'days': {
        const da = a.days_remaining ?? Infinity
        const db = b.days_remaining ?? Infinity
        return m * (da - db)
      }
      case 'expiration': {
        const ta = a.valid_to ? new Date(a.valid_to).getTime() : Infinity
        const tb = b.valid_to ? new Date(b.valid_to).getTime() : Infinity
        return m * (ta - tb)
      }
      case 'last_checked': {
        const ta = a.last_checked_at ? new Date(a.last_checked_at).getTime() : 0
        const tb = b.last_checked_at ? new Date(b.last_checked_at).getTime() : 0
        return m * (ta - tb)
      }
    }
    return 0
  })
  return arr
})

const PAGE_SIZE = 25

const {
  currentPage: probePage,
  totalPages: probeTotalPages,
  pagedItems: pagedProbes,
  resetPage: resetProbePage,
  setPage: setProbesPage,
} = usePagination({ items: sortedProbes, pageSize: PAGE_SIZE })

const {
  currentPage: certPage,
  totalPages: certTotalPages,
  pagedItems: pagedCerts,
  resetPage: resetCertPage,
  setPage: setCertPage,
} = usePagination({ items: sortedCerts, pageSize: PAGE_SIZE })

watch(probeSort, resetProbePage, { deep: true })
watch(certSort, resetCertPage, { deep: true })

function formatDate(ts: string | undefined | null): string {
  return ts ? dayjs(ts).format('YYYY-MM-DD') : '—'
}

function shortIssuer(s: string | undefined): string {
  if (!s) return ''
  const cn = /CN=([^,]+)/.exec(s)
  return cn ? cn[1] : s.split(',')[0]
}

function daysLabel(d: number | null | undefined): string {
  if (d == null) return 'Inconnu'
  if (d < 0) return `Expiré (${Math.abs(d)}j)`
  return `${d}j`
}

function daysBadge(d: number | null | undefined): string {
  if (d == null) return 'bg-secondary-lt text-secondary'
  if (d < 0) return 'bg-red text-white'
  if (d <= 7) return 'bg-red-lt text-red'
  if (d <= 30) return 'bg-yellow-lt text-yellow'
  return 'bg-green-lt text-green'
}

async function fetchCerts(): Promise<void> {
  loadingCerts.value = true
  try {
    const { data } = await api.getSSLCertificates()
    certs.value = data?.certificates || []
    lastUpdatedAt.value = new Date()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Impossible de charger les certificats'
  } finally {
    loadingCerts.value = false
  }
}

async function checkCertNow(c: SSLCert): Promise<void> {
  checkingCertId.value = c.id
  try {
    await api.checkSSLCertificateNow(c.id)
    await fetchCerts()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Échec de la vérification'
  } finally {
    checkingCertId.value = ''
  }
}

// cert form
const certModalOpen = ref(false)
const savingCert = ref(false)
const certFormError = ref('')
const certForm = ref<CertForm>(emptyCertForm())

function emptyCertForm(): CertForm {
  return { id: '', name: '', host: '', port: 443, server_name: '', enabled: true }
}

function openCreateCert(): void {
  certForm.value = emptyCertForm()
  certFormError.value = ''
  certModalOpen.value = true
}

function openEditCert(c: SSLCert): void {
  certForm.value = { id: c.id, name: c.name, host: c.host, port: c.port,
    server_name: c.server_name || '', enabled: c.enabled }
  certFormError.value = ''
  certModalOpen.value = true
}

function closeCertModal(): void {
  certModalOpen.value = false
  savingCert.value = false
}

async function saveCert(): Promise<void> {
  savingCert.value = true
  certFormError.value = ''
  try {
    const { id: _id, ...body } = certForm.value
    if (certForm.value.id) {
      await api.updateSSLCertificate(certForm.value.id, body)
    } else {
      await api.createSSLCertificate(body)
    }
    closeCertModal()
    await fetchCerts()
  } catch (e: unknown) {
    certFormError.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Erreur lors de l\'enregistrement'
  } finally {
    savingCert.value = false
  }
}

async function confirmDeleteCert(c: SSLCert): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Supprimer le certificat ?',
    message: `Cette action supprimera "${c.name}" du suivi.`,
    okLabel: 'Supprimer',
    destructive: true,
  })
  if (!ok) return
  try {
    await api.deleteSSLCertificate(c.id)
    await fetchCerts()
  } catch (e: unknown) {
    error.value = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Suppression impossible'
  }
}

// ── lifecycle ─────────────────────────────────────────────────────────────────
let refreshTimer: ReturnType<typeof setInterval> | undefined

function refreshAll() {
  fetchProbes()
  fetchCerts()
}

onMounted(() => {
  refreshAll()
  refreshTimer = setInterval(() => { if (autoRefresh.value) refreshAll() }, REFRESH_SEC * 1000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.btn-sort {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  font: inherit;
  font-weight: inherit;
  text-align: left;
  color: inherit;
  white-space: nowrap;
}
.btn-sort:hover {
  color: var(--tblr-primary);
}
.sort-icon {
  font-size: 0.7em;
  opacity: 0.5;
  margin-left: 0.25rem;
}
.btn-sort:hover .sort-icon {
  opacity: 0.9;
}
</style>

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
      v-if="tab === 'uptime' ? probeError : certError"
      class="alert alert-danger mb-3"
    >
      {{ tab === 'uptime' ? probeError : certError }}
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
                  <SortableHeader
                    label="Sonde"
                    :active="probeSort.col === 'name'"
                    :direction="probeSort.dir"
                    @toggle="toggleProbeSort('name')"
                  />
                </th>
                <th>Cible</th>
                <th>
                  <SortableHeader
                    label="Statut"
                    :active="probeSort.col === 'status'"
                    :direction="probeSort.dir"
                    @toggle="toggleProbeSort('status')"
                  />
                </th>
                <th>
                  <SortableHeader
                    label="Uptime 24h"
                    :active="probeSort.col === 'uptime'"
                    :direction="probeSort.dir"
                    @toggle="toggleProbeSort('uptime')"
                  />
                </th>
                <th>
                  <SortableHeader
                    label="Latence"
                    :active="probeSort.col === 'latency'"
                    :direction="probeSort.dir"
                    @toggle="toggleProbeSort('latency')"
                  />
                </th>
                <th>
                  <SortableHeader
                    label="Dernière vérification"
                    :active="probeSort.col === 'last_checked'"
                    :direction="probeSort.dir"
                    @toggle="toggleProbeSort('last_checked')"
                  />
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
                  <SortableHeader
                    label="Nom"
                    :active="certSort.col === 'name'"
                    :direction="certSort.dir"
                    @toggle="toggleCertSort('name')"
                  />
                </th>
                <th>Endpoint</th>
                <th>Émetteur</th>
                <th>
                  <SortableHeader
                    label="Expiration"
                    :active="certSort.col === 'expiration'"
                    :direction="certSort.dir"
                    @toggle="toggleCertSort('expiration')"
                  />
                </th>
                <th class="text-nowrap">
                  <SortableHeader
                    label="Jours restants"
                    :active="certSort.col === 'days'"
                    :direction="certSort.dir"
                    @toggle="toggleCertSort('days')"
                  />
                </th>
                <th>
                  <SortableHeader
                    label="Dernière vérification"
                    :active="certSort.col === 'last_checked'"
                    :direction="certSort.dir"
                    @toggle="toggleCertSort('last_checked')"
                  />
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
import { ref, watch } from 'vue'
import { IconActivity, IconLock } from '@tabler/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import RelativeTime from '../components/RelativeTime.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import PaginationNav from '../components/PaginationNav.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import { useMonitoring } from '../composables/useMonitoring'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

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

const {
  REFRESH_SEC,
  PAGE_SIZE,
  autoRefresh,
  lastUpdatedAt,
  probeError,
  certError,
  probes,
  loadingProbes,
  probeStats,
  checkingProbeId,
  downCount,
  probeSort,
  toggleProbeSort,
  pagedProbes,
  probeBadge,
  probeStatusLabel,
  uptimeBadgeClass,
  checkProbeNow,
  probeModalOpen,
  savingProbe,
  probeFormError,
  probeForm,
  openCreateProbe,
  openEditProbe,
  closeProbeModal,
  saveProbe,
  confirmDeleteProbe,
  probePage,
  probeTotalPages,
  setProbesPage,
  certs,
  loadingCerts,
  checkingCertId,
  expiringCount,
  certSort,
  toggleCertSort,
  pagedCerts,
  formatDate,
  shortIssuer,
  daysLabel,
  daysBadge,
  checkCertNow,
  certModalOpen,
  savingCert,
  certFormError,
  certForm,
  openCreateCert,
  openEditCert,
  closeCertModal,
  saveCert,
  confirmDeleteCert,
  certPage,
  certTotalPages,
  setCertPage,
} = useMonitoring()
</script>

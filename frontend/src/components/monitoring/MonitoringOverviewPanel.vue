<template>
  <div>
    <div class="row row-cards mb-3">
      <div class="col-6 col-md-4">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Sondes en panne
            </div>
            <div
              class="h2 mb-0 mt-1"
              :class="downCount > 0 ? 'text-danger' : 'text-success'"
            >
              {{ downCount }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-md-4">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Certificats expirant &lt;30j
            </div>
            <div
              class="h2 mb-0 mt-1"
              :class="expiringCount > 0 ? 'text-warning' : 'text-success'"
            >
              {{ expiringCount }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-md-4">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              Total surveillé
            </div>
            <div class="h2 mb-0 mt-1">
              {{ totalMonitored }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <PageRefreshBar
      v-model="autoRefresh"
      label="Monitoring"
      :interval-sec="REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div
      v-if="loading && !pagedRows.length"
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
      v-else-if="!pagedRows.length"
      title="Aucune sonde ni certificat configuré"
      subtitle="Créez une sonde uptime ou un certificat SSL pour commencer à surveiller un service."
      :cta-label="auth.role === 'admin' ? 'Nouvelle sonde' : ''"
      @cta="openCreateProbe"
    />

    <div
      v-else
      class="card"
    >
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>
                <SortableHeader
                  label="Nom / Cible"
                  :active="rowSort.col === 'name'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('name')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Statut"
                  :active="rowSort.col === 'status'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('status')"
                />
              </th>
              <th>Disponibilité</th>
              <th>
                <SortableHeader
                  label="Uptime 24h"
                  :active="rowSort.col === 'uptime'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('uptime')"
                />
              </th>
              <th>
                <SortableHeader
                  label="SSL"
                  :active="rowSort.col === 'ssl_days'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('ssl_days')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Dernière vérification"
                  :active="rowSort.col === 'last_checked'"
                  :direction="rowSort.dir"
                  @toggle="toggleRowSort('last_checked')"
                />
              </th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in pagedRows"
              :key="row.id"
              :class="{ 'opacity-60': row.probe && !row.probe.enabled }"
            >
              <td>
                <router-link
                  :to="rowLink(row)"
                  class="fw-semibold text-decoration-none"
                >
                  {{ row.name }}
                </router-link>
                <div
                  v-if="row.probe"
                  class="text-secondary small"
                >
                  <code>{{ row.probe.target }}</code>
                </div>
                <div
                  v-else-if="row.cert"
                  class="text-secondary small"
                >
                  <code>{{ row.cert.host }}:{{ row.cert.port }}</code>
                </div>
                <span
                  v-if="row.npmProxyHostId"
                  class="badge bg-azure-lt text-azure mt-1"
                >NPM</span>
              </td>
              <td>
                <template v-if="row.probe">
                  <span :class="['badge', probeBadge(row.probe)]">
                    {{ probeStatusLabel(row.probe) }}
                  </span>
                  <span
                    v-if="!row.probe.enabled"
                    class="badge bg-secondary-lt text-secondary ms-1"
                  >désactivée</span>
                </template>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td>
                <div
                  v-if="row.probe && probeHistory[row.probe.id]?.length"
                  class="d-flex align-items-end gap-1"
                  style="height: 20px; min-width: 110px;"
                >
                  <div
                    v-for="tick in probeHistory[row.probe.id]"
                    :key="tick.id"
                    class="flex-fill rounded-1"
                    :class="tick.success ? 'bg-success' : 'bg-danger'"
                    style="height: 100%; min-width: 2px;"
                    :title="`${formatDateTime(tick.checked_at)} — ${tick.success ? 'OK' : 'KO'}`"
                  />
                </div>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td>
                <template v-if="row.probe && probeStats[row.probe.id]">
                  <span :class="['badge', uptimeBadgeClass(probeStats[row.probe.id].uptime_percent)]">
                    {{ probeStats[row.probe.id].uptime_percent.toFixed(1) }}%
                  </span>
                </template>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td>
                <span
                  v-if="row.cert"
                  :class="['badge', daysBadge(row.cert.days_remaining)]"
                >
                  {{ daysLabel(row.cert.days_remaining) }}
                </span>
                <span
                  v-else
                  class="text-secondary small"
                >—</span>
              </td>
              <td class="text-secondary small">
                <RelativeTime
                  v-if="row.probe?.last_checked_at || row.cert?.last_checked_at"
                  :date="(row.probe?.last_checked_at || row.cert?.last_checked_at) as string"
                />
                <span
                  v-else
                  class="text-secondary"
                >Jamais</span>
              </td>
              <td class="text-end">
                <div class="btn-list flex-nowrap justify-content-end">
                  <button
                    v-if="auth.role === 'admin'"
                    type="button"
                    class="btn btn-sm btn-outline-secondary"
                    :disabled="checkingProbeId === row.probe?.id || checkingCertId === row.cert?.id"
                    @click="checkRowNow(row)"
                  >
                    Vérifier
                  </button>
                  <div
                    v-if="auth.role === 'admin' && row.probe"
                    class="btn-group btn-group-sm"
                  >
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      title="Modifier la sonde"
                      aria-label="Modifier la sonde"
                      @click="openEditProbe(row.probe)"
                    >
                      <IconActivity :size="14" />
                    </button>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-danger"
                      title="Supprimer la sonde"
                      aria-label="Supprimer la sonde"
                      @click="confirmDeleteProbe(row.probe)"
                    >
                      <IconTrash :size="14" />
                    </button>
                  </div>
                  <div
                    v-if="auth.role === 'admin' && row.cert"
                    class="btn-group btn-group-sm"
                  >
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      title="Modifier le certificat"
                      aria-label="Modifier le certificat"
                      @click="openEditCert(row.cert)"
                    >
                      <IconLock :size="14" />
                    </button>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-danger"
                      title="Supprimer le certificat"
                      aria-label="Supprimer le certificat"
                      @click="confirmDeleteCert(row.cert)"
                    >
                      <IconTrash :size="14" />
                    </button>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-if="rowTotalPages > 1"
        class="card-footer d-flex align-items-center justify-content-between"
      >
        <div class="text-secondary small">
          {{ (rowPage - 1) * PAGE_SIZE + 1 }}–{{ Math.min(rowPage * PAGE_SIZE, totalMonitored) }} sur {{ totalMonitored }}
        </div>
        <PaginationNav
          :current-page="rowPage"
          :total-pages="rowTotalPages"
          @select="setRowPage"
        />
      </div>
    </div>

    <!-- ===== MODAL SONDE ===== -->
    <div
      v-if="probeModalOpen"
      ref="probeModalRef"
      class="modal modal-blur fade show d-block"
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
      ref="certModalRef"
      class="modal modal-blur fade show d-block"
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
import { ref } from 'vue'
import { IconActivity, IconLock, IconTrash } from '@tabler/icons-vue'
import { useAuthStore } from '../../stores/auth'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import RelativeTime from '../RelativeTime.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import PaginationNav from '../PaginationNav.vue'
import SortableHeader from '../common/SortableHeader.vue'
import { formatDateTime } from '../../utils/formatters'
import { useMonitoringOverview, type MonitoringRow } from '../../composables/useMonitoringOverview'
import { useModalChrome } from '../../composables/useModalChrome'

const auth = useAuthStore()

const {
  PAGE_SIZE,
  loading,
  error,
  downCount,
  expiringCount,
  totalMonitored,
  rowSort,
  toggleRowSort,
  pagedRows,
  rowPage,
  rowTotalPages,
  setRowPage,
  checkingProbeId,
  checkingCertId,
  checkRowNow,
  probeStats,
  probeHistory,
  probeBadge,
  probeStatusLabel,
  uptimeBadgeClass,
  daysLabel,
  daysBadge,
  autoRefresh,
  lastUpdatedAt,
  REFRESH_SEC,
  probeModalOpen,
  savingProbe,
  probeFormError,
  probeForm,
  openCreateProbe,
  openEditProbe,
  closeProbeModal,
  saveProbe,
  confirmDeleteProbe,
  certModalOpen,
  savingCert,
  certFormError,
  certForm,
  openCreateCert,
  openEditCert,
  closeCertModal,
  saveCert,
  confirmDeleteCert,
} = useMonitoringOverview()

const probeModalRef = ref<HTMLElement | null>(null)
const certModalRef = ref<HTMLElement | null>(null)
useModalChrome(probeModalRef, () => probeModalOpen.value, { onClose: closeProbeModal })
useModalChrome(certModalRef, () => certModalOpen.value, { onClose: closeCertModal })

function rowLink(row: MonitoringRow): string {
  if (row.npmProxyHostId) return `/monitoring/host/${row.npmProxyHostId}`
  if (row.probe) return `/monitoring/probes/${row.probe.id}`
  return `/monitoring/ssl/${row.cert!.id}`
}

defineExpose({ openCreateProbe, openCreateCert })
</script>

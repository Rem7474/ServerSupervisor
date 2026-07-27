<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Certificats SSL"
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
import { watch } from 'vue'
import { useAuthStore } from '../../stores/auth'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import RelativeTime from '../RelativeTime.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import PaginationNav from '../PaginationNav.vue'
import SortableHeader from '../common/SortableHeader.vue'
import { useSslCertificates } from '../../composables/useSslCertificates'

const auth = useAuthStore()

const emit = defineEmits<{
  (e: 'update:expiring-count', value: number): void
}>()

const {
  REFRESH_SEC,
  PAGE_SIZE,
  autoRefresh,
  lastUpdatedAt,
  error,
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
} = useSslCertificates()

watch(expiringCount, (v) => emit('update:expiring-count', v), { immediate: true })

defineExpose({ openCreateCert })
</script>

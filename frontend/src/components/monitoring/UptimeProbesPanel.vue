<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Sondes uptime"
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
import { useUptimeProbes } from '../../composables/useUptimeProbes'

const auth = useAuthStore()

const emit = defineEmits<{
  (e: 'update:down-count', value: number): void
}>()

const {
  REFRESH_SEC,
  PAGE_SIZE,
  autoRefresh,
  lastUpdatedAt,
  error,
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
} = useUptimeProbes()

watch(downCount, (v) => emit('update:down-count', v), { immediate: true })

defineExpose({ openCreateProbe })
</script>

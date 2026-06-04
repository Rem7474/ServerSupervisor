<template>
  <div class="card">
    <!-- Header : identité + statut + actions par hôte -->
    <div class="card-header">
      <div class="d-flex align-items-center gap-3 flex-wrap w-100">
        <label class="form-check m-0">
          <input
            type="checkbox"
            class="form-check-input"
            :checked="selected"
            @change="$emit('update:selected', ($event.target as HTMLInputElement).checked)"
          >
          <span class="form-check-label" />
        </label>
        <div class="flex-fill min-w-0">
          <div class="d-flex align-items-center gap-2 flex-wrap">
            <router-link
              :to="`/hosts/${host.id}`"
              class="fw-semibold text-reset text-decoration-none"
            >
              {{ host.name || host.hostname }}
            </router-link>
            <span
              v-if="host.name && host.hostname && host.name !== host.hostname"
              class="text-secondary small"
            >
              {{ host.hostname }}
            </span>
            <span class="text-muted small">{{ host.ip_address }}</span>
          </div>
        </div>
        <span :class="host.status === 'online' ? 'status status-lime' : 'status status-red'">
          <span :class="['status-dot', host.status === 'online' ? 'status-dot-animated' : '']" />
          <span>{{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}</span>
        </span>
        <div
          v-if="canRunApt"
          class="d-flex gap-1 flex-shrink-0"
        >
          <div class="btn-group btn-group-sm">
            <button
              class="btn btn-outline-secondary"
              :disabled="isCmdLoading"
              @click="$emit('run-cmd', 'update')"
            >
              <span
                v-if="cmdLoading === 'update'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              update
            </button>
            <button
              class="btn btn-primary"
              :disabled="isCmdLoading"
              @click="$emit('run-cmd', 'upgrade')"
            >
              <span
                v-if="cmdLoading === 'upgrade'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              upgrade
            </button>
            <button
              class="btn btn-outline-danger"
              :disabled="isCmdLoading"
              @click="$emit('run-cmd', 'dist-upgrade')"
            >
              <span
                v-if="cmdLoading === 'dist-upgrade'"
                class="spinner-border spinner-border-sm me-1"
                role="status"
              />
              dist-upgrade
            </button>
          </div>
          <button
            class="btn btn-sm btn-outline-secondary"
            title="Planifier une commande APT"
            @click="$emit('schedule')"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="icon icon-sm"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect
                x="3"
                y="4"
                width="18"
                height="18"
                rx="2"
                ry="2"
              /><line
                x1="16"
                y1="2"
                x2="16"
                y2="6"
              /><line
                x1="8"
                y1="2"
                x2="8"
                y2="6"
              /><line
                x1="3"
                y1="10"
                x2="21"
                y2="10"
              />
            </svg>
          </button>
        </div>
        <span
          v-else
          class="text-secondary small flex-shrink-0"
        >Mode lecture seule</span>
        <button
          class="btn btn-sm btn-ghost-secondary flex-shrink-0"
          :title="expanded ? 'Réduire' : 'Développer'"
          @click="$emit('update:expanded', !expanded)"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            :style="{ transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }"
          >
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Corps : KPI + CVE + paquets + historique -->
    <div class="card-body">
      <!-- Pas de données -->
      <div
        v-if="!aptStatus"
        class="text-secondary small"
      >
        Données APT non disponibles — lancez <strong>apt update</strong> pour initialiser.
      </div>

      <template v-else>
        <!-- KPI stats (toujours visibles) -->
        <div class="row g-2 mb-1">
          <div class="col-3">
            <div
              class="text-center p-2 rounded"
              :class="aptStatus.pending_packages > 0 ? 'bg-yellow-lt' : 'bg-green-lt'"
            >
              <div
                class="fs-3 fw-bold lh-1 mb-1"
                :class="aptStatus.pending_packages > 0 ? 'text-yellow' : 'text-green'"
              >
                {{ aptStatus.pending_packages }}
              </div>
              <div class="text-secondary small">
                en attente
              </div>
            </div>
          </div>
          <div class="col-3">
            <div
              class="text-center p-2 rounded"
              :class="aptStatus.security_updates > 0 ? 'bg-red-lt' : 'bg-secondary-lt'"
            >
              <div
                class="fs-3 fw-bold lh-1 mb-1"
                :class="aptStatus.security_updates > 0 ? 'text-red' : 'text-secondary'"
              >
                {{ aptStatus.security_updates }}
              </div>
              <div class="text-secondary small">
                sécurité
              </div>
            </div>
          </div>
          <div class="col-3">
            <div
              class="text-center p-2 rounded"
              :class="cveList.length
                ? (cveList.some(c => c.severity === 'CRITICAL') ? 'bg-red-lt' : 'bg-orange-lt')
                : 'bg-secondary-lt'"
            >
              <div
                class="fs-3 fw-bold lh-1 mb-1"
                :class="cveList.length
                  ? (cveList.some(c => c.severity === 'CRITICAL') ? 'text-red' : 'text-orange')
                  : 'text-secondary'"
              >
                {{ cveList.length }}
              </div>
              <div class="text-secondary small">
                CVE
              </div>
            </div>
          </div>
          <div class="col-3">
            <div class="text-center p-2 rounded bg-secondary-lt">
              <div class="text-secondary small">
                Dernier apt update
              </div>
              <div class="fw-semibold small lh-1 mb-2 text-truncate">
                {{ aptStatus.last_update ? formatDate(aptStatus.last_update) : 'Jamais' }}
              </div>
              <div class="text-secondary small">
                Dernier upgrade
              </div>
              <div class="fw-semibold small lh-1 text-truncate">
                {{ aptStatus.last_upgrade ? formatDate(aptStatus.last_upgrade) : 'Jamais' }}
              </div>
            </div>
          </div>
        </div>

        <!-- Détail extensible -->
        <template v-if="expanded">
          <!-- CVE -->
          <div
            v-if="cveList.length"
            class="mt-3 mb-3"
          >
            <CVEList
              :cve-list="cveList"
              :show-max-severity="true"
              :always-expanded="false"
              :initially-collapsed="false"
              :limit="3"
            />
          </div>

          <!-- Paquets en attente -->
          <div
            v-if="packages.length > 0"
            class="mb-3"
          >
            <div class="d-flex align-items-center justify-content-between mb-2">
              <span class="small fw-semibold text-secondary">
                Paquets en attente
                <span class="badge bg-yellow-lt text-yellow ms-1">
                  {{ packages.length }}
                </span>
              </span>
              <button
                v-if="packages.length > PKG_PREVIEW_COUNT"
                class="btn btn-link btn-sm p-0 small text-secondary"
                @click="pkgShowAll = !pkgShowAll"
              >
                {{ pkgShowAll ? 'Réduire' : `Voir tout (${packages.length})` }}
              </button>
            </div>
            <div class="apt-packages-grid">
              <div
                v-for="pkg in visiblePackages"
                :key="pkg"
              >
                <code
                  class="small text-body apt-package-item"
                  :title="pkg"
                >{{ pkg }}</code>
              </div>
            </div>
          </div>

          <!-- Historique (2 dernières commandes) -->
          <div
            v-if="history?.length"
            class="border-top pt-2"
          >
            <div class="d-flex align-items-center justify-content-between mb-1">
              <span class="small fw-semibold text-secondary">Dernières commandes</span>
              <router-link
                to="/audit?module=apt"
                class="small text-secondary text-decoration-none"
              >
                Historique complet →
              </router-link>
            </div>
            <div
              v-for="cmd in history.slice(0, 2)"
              :key="cmd.id"
              class="d-flex align-items-center gap-2 py-1 flex-wrap"
            >
              <code class="small">apt {{ cmd.action }}</code>
              <span :class="statusClass(cmd.status)">{{ statusLabel(cmd.status) }}</span>
              <span class="text-secondary small flex-shrink-0">{{ formatDate(cmd.created_at) }}</span>
              <span
                v-if="cmd.triggered_by"
                class="text-muted small flex-shrink-0"
              >· {{ cmd.triggered_by }}</span>
              <button
                class="btn btn-sm btn-ghost-secondary ms-auto flex-shrink-0"
                title="Voir les logs"
                @click="$emit('watch-command', cmd)"
              >
                <svg
                  class="icon icon-sm"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  stroke-width="2"
                  stroke="currentColor"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                ><path
                  stroke="none"
                  d="M0 0h24v24H0z"
                  fill="none"
                /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
              </button>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import CVEList from './CVEList.vue'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { useStatusBadge } from '../../composables/useStatusBadge'

const props = defineProps<{
  host: any
  aptStatus: any
  history: any[] | undefined
  expanded: boolean
  selected: boolean
  canRunApt: boolean
  cmdLoading: string | null | undefined
}>()

defineEmits<{
  (e: 'update:selected', value: boolean): void
  (e: 'update:expanded', value: boolean): void
  (e: 'run-cmd', command: string): void
  (e: 'schedule'): void
  (e: 'watch-command', cmd: any): void
}>()

const { formatRelativeDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

const PKG_PREVIEW_COUNT = 15
const pkgShowAll = ref(false)

const isCmdLoading = computed(() => !!props.cmdLoading)

function parseJsonArray(value: any): any[] {
  if (!value) return []
  try {
    const parsed = typeof value === 'string' ? JSON.parse(value) : value
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

const cveList = computed(() => parseJsonArray(props.aptStatus?.cve_list))
const packages = computed(() => parseJsonArray(props.aptStatus?.package_list))
const visiblePackages = computed(() =>
  pkgShowAll.value ? packages.value : packages.value.slice(0, PKG_PREVIEW_COUNT),
)

function formatDate(date: string | undefined): string {
  return formatRelativeDate(date)
}

const STATUS_LABELS: Record<string, string> = {
  pending: 'En attente',
  running: 'En cours',
  completed: 'Terminé',
  failed: 'Échoué',
}

function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
}

function statusClass(status: string | undefined): string {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}
</script>

<style scoped>
.apt-packages-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.35rem 0.75rem;
}
.apt-package-item {
  display: block;
}
@media (min-width: 768px) {
  .apt-packages-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (min-width: 1200px) {
  .apt-packages-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>

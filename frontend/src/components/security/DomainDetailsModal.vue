<template>
  <div
    v-if="show"
    class="traffic-modal-backdrop"
    @click.self="$emit('close')"
  >
    <div class="traffic-modal card shadow-lg">
      <div class="card-header d-flex align-items-center justify-content-between">
        <div>
          <h3 class="card-title mb-0">
            Détails domaine: <span class="font-monospace">{{ domain }}</span>
          </h3>
          <div class="text-secondary small">
            Fenêtre de logs détaillée sur {{ period }}
          </div>
        </div>
        <button
          type="button"
          class="btn btn-sm btn-outline-secondary"
          @click="$emit('close')"
        >
          Fermer
        </button>
      </div>

      <div class="card-body traffic-modal-body">
        <div
          v-if="error"
          class="alert alert-danger"
        >
          {{ error }}
        </div>

        <div
          v-if="loading && !hasData"
          class="text-center py-4 text-secondary"
        >
          <span class="spinner-border spinner-border-sm me-2" />
          Chargement des détails...
        </div>

        <template v-else>
          <div class="row row-cards mb-3">
            <div class="col-6 col-lg-3">
              <button
                type="button"
                class="kpi-btn clickable-row border rounded p-2 text-center w-100"
                :class="{ active: filters.status === '' }"
                @click="$emit('update-filter', { key: 'status', value: '' })"
              >
                <div class="text-secondary small">
                  Hits
                </div>
                <div class="h3 mb-0">
                  {{ details.hits || 0 }}
                </div>
              </button>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 text-center">
                <div class="text-secondary small">
                  Bytes
                </div>
                <div class="h3 mb-0">
                  {{ formatBytes(details.bytes || 0) }}
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <button
                type="button"
                class="kpi-btn clickable-row border rounded p-2 text-center w-100"
                :class="{ active: filters.status === '4xx' }"
                title="Filtrer sur les statuts 4xx"
                @click="$emit('update-filter', { key: 'status', value: filters.status === '4xx' ? '' : '4xx' })"
              >
                <div class="text-secondary small">
                  4xx
                </div>
                <div class="h3 mb-0 text-yellow">
                  {{ details.status_4xx || 0 }}
                </div>
              </button>
            </div>
            <div class="col-6 col-lg-3">
              <button
                type="button"
                class="kpi-btn clickable-row border rounded p-2 text-center w-100"
                :class="{ active: filters.status === '5xx' }"
                title="Filtrer sur les statuts 5xx"
                @click="$emit('update-filter', { key: 'status', value: filters.status === '5xx' ? '' : '5xx' })"
              >
                <div class="text-secondary small">
                  5xx
                </div>
                <div class="h3 mb-0 text-red">
                  {{ details.status_5xx || 0 }}
                </div>
              </button>
            </div>
          </div>

          <div class="row row-cards mb-3">
            <div class="col-lg-6">
              <div class="card h-100">
                <div class="card-header">
                  <h4 class="card-title mb-0">
                    Top chemins
                  </h4>
                </div>
                <div class="card-body p-0">
                  <div
                    v-if="!(details.top_paths || []).length"
                    class="text-center py-3 text-secondary small"
                  >
                    Aucun chemin
                  </div>
                  <div
                    v-for="p in details.top_paths"
                    v-else
                    :key="p.path"
                    role="button"
                    tabindex="0"
                    class="row-btn clickable-row d-flex justify-content-between align-items-center border-bottom px-3 py-2"
                    :class="{ active: filters.path === p.path }"
                    :title="`Filtrer sur ${p.path}`"
                    @click="$emit('update-filter', { key: 'path', value: filters.path === p.path ? '' : p.path })"
                    @keydown.enter="$emit('update-filter', { key: 'path', value: filters.path === p.path ? '' : p.path })"
                    @keydown.space.prevent="$emit('update-filter', { key: 'path', value: filters.path === p.path ? '' : p.path })"
                  >
                    <span
                      class="font-monospace small text-truncate me-2 user-select-all"
                      style="max-width: 75%;"
                    >{{ p.path }}</span>
                    <span class="badge bg-azure-lt text-azure">{{ p.hits }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="col-lg-6">
              <div class="card h-100">
                <div class="card-header">
                  <h4 class="card-title mb-0">
                    Top IPs clientes
                  </h4>
                </div>
                <div class="card-body p-0">
                  <div
                    v-if="!(details.top_clients || []).length"
                    class="text-center py-3 text-secondary small"
                  >
                    Aucune IP
                  </div>
                  <div
                    v-for="c in details.top_clients"
                    v-else
                    :key="c.ip"
                    role="button"
                    tabindex="0"
                    class="row-btn clickable-row d-flex justify-content-between align-items-center gap-2 border-bottom px-3 py-2"
                    :class="{ active: filters.ip === c.ip }"
                    :title="`Filtrer sur ${c.ip}`"
                    @click="$emit('update-filter', { key: 'ip', value: filters.ip === c.ip ? '' : c.ip })"
                    @keydown.enter="$emit('update-filter', { key: 'ip', value: filters.ip === c.ip ? '' : c.ip })"
                    @keydown.space.prevent="$emit('update-filter', { key: 'ip', value: filters.ip === c.ip ? '' : c.ip })"
                  >
                    <span class="font-monospace small user-select-all">
                      {{ c.ip }}
                      <span
                        v-if="c.blocked"
                        class="badge bg-red-lt text-red ms-1"
                      >Bloquée</span>
                    </span>
                    <span class="d-flex align-items-center gap-1 flex-shrink-0">
                      <span class="badge bg-purple-lt text-purple">{{ c.hits }}</span>
                      <button
                        type="button"
                        class="btn btn-icon btn-sm btn-ghost-secondary"
                        title="Copier l'IP"
                        @click.stop="copyIP(c.ip)"
                      >
                        <IconCheck
                          v-if="copiedIP === c.ip"
                          :size="14"
                          class="icon text-success"
                        />
                        <IconCopy
                          v-else
                          :size="14"
                          class="icon"
                        />
                      </button>
                      <button
                        v-if="!c.blocked"
                        type="button"
                        class="btn btn-icon btn-sm"
                        :class="blockState?.[c.ip] === 'error' ? 'btn-ghost-danger' : 'btn-ghost-secondary'"
                        :disabled="blockState?.[c.ip] === 'loading' || !c.host_id"
                        :title="!c.host_id ? 'Hôte introuvable' : blockState?.[c.ip] === 'error' ? 'Erreur — Réessayer' : `Bloquer ${c.ip} (CrowdSec, 4h)`"
                        @click.stop="$emit('block-ip', { ip: c.ip, hostId: c.host_id })"
                      >
                        <span
                          v-if="blockState?.[c.ip] === 'loading'"
                          class="spinner-border spinner-border-sm"
                        />
                        <IconBan
                          v-else
                          :size="14"
                          class="icon"
                        />
                      </button>
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
              <h4 class="card-title mb-0">
                Logs récents
              </h4>
              <div class="d-flex align-items-center gap-2 flex-wrap">
                <select
                  class="form-select form-select-sm w-auto"
                  :value="filters.status"
                  @change="$emit('update-filter', { key: 'status', value: ($event.target as HTMLSelectElement).value })"
                >
                  <option value="">
                    Tous statuts
                  </option>
                  <option value="2xx">
                    2xx
                  </option>
                  <option value="3xx">
                    3xx
                  </option>
                  <option value="4xx">
                    4xx
                  </option>
                  <option value="5xx">
                    5xx
                  </option>
                  <option value="blocked">
                    Bloquées
                  </option>
                  <option value="suspicious">
                    Suspectes
                  </option>
                </select>
                <select
                  class="form-select form-select-sm w-auto"
                  :value="filters.method"
                  @change="$emit('update-filter', { key: 'method', value: ($event.target as HTMLSelectElement).value })"
                >
                  <option value="">
                    Toutes méthodes
                  </option>
                  <option
                    v-for="m in METHODS"
                    :key="m"
                    :value="m"
                  >
                    {{ m }}
                  </option>
                </select>
                <button
                  v-if="hasActiveFilters"
                  type="button"
                  class="btn btn-sm btn-outline-secondary"
                  @click="$emit('clear-filters')"
                >
                  Réinitialiser
                </button>
              </div>
            </div>

            <div
              v-if="hasActiveFilters"
              class="px-3 pt-2 d-flex align-items-center gap-2 flex-wrap"
            >
              <span class="text-secondary small">Filtres actifs :</span>
              <span
                v-if="filters.path"
                class="badge bg-azure-lt text-azure d-inline-flex align-items-center gap-1"
              >
                {{ filters.path }}
                <button
                  type="button"
                  class="btn-close btn-close-white ms-1"
                  style="font-size: 0.55rem;"
                  aria-label="Retirer le filtre chemin"
                  @click="$emit('update-filter', { key: 'path', value: '' })"
                />
              </span>
              <span
                v-if="filters.ip"
                class="badge bg-purple-lt text-purple d-inline-flex align-items-center gap-1"
              >
                {{ filters.ip }}
                <button
                  type="button"
                  class="btn-close btn-close-white ms-1"
                  style="font-size: 0.55rem;"
                  aria-label="Retirer le filtre IP"
                  @click="$emit('update-filter', { key: 'ip', value: '' })"
                />
              </span>
            </div>

            <div
              class="table-responsive scroll-table"
              style="max-height: 360px;"
            >
              <table class="table table-sm table-vcenter mb-0">
                <thead>
                  <tr>
                    <th>
                      <SortableHeader
                        label="Heure"
                        :active="sortKey === 'time'"
                        :direction="sortDir"
                        @toggle="$emit('toggle-sort', 'time')"
                      />
                    </th>
                    <th>IP</th>
                    <th>Méthode</th>
                    <th>Chemin</th>
                    <th>
                      <SortableHeader
                        label="Status"
                        :active="sortKey === 'status'"
                        :direction="sortDir"
                        @toggle="$emit('toggle-sort', 'status')"
                      />
                    </th>
                    <th>
                      <SortableHeader
                        label="Bytes"
                        :active="sortKey === 'bytes'"
                        :direction="sortDir"
                        @toggle="$emit('toggle-sort', 'bytes')"
                      />
                    </th>
                    <th>UA</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!(details.requests || []).length">
                    <td
                      colspan="7"
                      class="text-center text-secondary py-3"
                    >
                      Aucune requête disponible
                    </td>
                  </tr>
                  <tr
                    v-for="(r, idx) in details.requests || []"
                    :key="`${r.timestamp}-${idx}`"
                  >
                    <td class="small">
                      {{ formatDate(r.timestamp) }}
                    </td>
                    <td class="font-monospace small">
                      <span class="d-flex align-items-center gap-1">
                        <span class="user-select-all">{{ r.ip }}</span>
                        <button
                          type="button"
                          class="btn btn-icon btn-sm btn-ghost-secondary"
                          title="Copier l'IP"
                          @click="copyIP(r.ip)"
                        >
                          <IconCheck
                            v-if="copiedIP === r.ip"
                            :size="12"
                            class="icon text-success"
                          />
                          <IconCopy
                            v-else
                            :size="12"
                            class="icon"
                          />
                        </button>
                        <button
                          v-if="!r.blocked"
                          type="button"
                          class="btn btn-icon btn-sm"
                          :class="blockState?.[r.ip] === 'error' ? 'btn-ghost-danger' : 'btn-ghost-secondary'"
                          :disabled="blockState?.[r.ip] === 'loading' || !r.host_id"
                          :title="!r.host_id ? 'Hôte introuvable' : blockState?.[r.ip] === 'error' ? 'Erreur — Réessayer' : `Bloquer ${r.ip} (CrowdSec, 4h)`"
                          @click="$emit('block-ip', { ip: r.ip, hostId: r.host_id })"
                        >
                          <span
                            v-if="blockState?.[r.ip] === 'loading'"
                            class="spinner-border spinner-border-sm"
                          />
                          <IconBan
                            v-else
                            :size="12"
                            class="icon"
                          />
                        </button>
                      </span>
                    </td>
                    <td><span class="badge bg-blue-lt text-blue">{{ r.method }}</span></td>
                    <td
                      class="font-monospace small text-truncate domain-path"
                      :title="r.path"
                      style="max-width: 18rem;"
                    >
                      {{ r.path }}
                    </td>
                    <td>
                      <span
                        class="badge"
                        :class="statusClass(r.status)"
                      >{{ r.status }}</span>
                      <span
                        v-if="r.blocked"
                        class="badge bg-red-lt text-red ms-1"
                        title="Bloquée"
                      >B</span>
                      <span
                        v-if="r.suspicious"
                        class="badge bg-yellow-lt text-yellow ms-1"
                        title="Suspecte"
                      >S</span>
                    </td>
                    <td class="small">
                      {{ formatBytes(r.bytes || 0) }}
                    </td>
                    <td
                      class="small text-truncate domain-ua"
                      :title="r.user_agent || '-'"
                      style="max-width: 20rem;"
                    >
                      {{ r.user_agent || '-' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div
              v-if="totalPages > 1"
              class="card-footer d-flex align-items-center justify-content-between"
            >
              <div class="text-secondary small">
                Page {{ page }} sur {{ totalPages }} — {{ details.total || 0 }} résultats
              </div>
              <PaginationNav
                :current-page="page"
                :total-pages="totalPages"
                @select="$emit('update:page', $event)"
              />
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IconBan, IconCheck, IconCopy } from '@tabler/icons-vue'
import SortableHeader from '../common/SortableHeader.vue'
import PaginationNav from '../PaginationNav.vue'
import type { DomainDetailsFilterKey, DomainDetailsSortKey } from '../../composables/useDomainDetails'

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for the ad-hoc GetDomainDetails aggregate (no Go model)
type AnyRecord = Record<string, any>

const props = withDefaults(defineProps<{
  show: boolean
  domain: string
  loading: boolean
  error?: string
  details?: AnyRecord
  period: string
  filters: { status: string; method: string; path: string; ip: string }
  sortKey: DomainDetailsSortKey
  sortDir: 'asc' | 'desc'
  page: number
  totalPages: number
  hasActiveFilters: boolean
  blockState?: Record<string, 'loading' | 'error'>
}>(), {
  error: '',
  details: () => ({}),
  blockState: () => ({}),
})

defineEmits<{
  (e: 'close'): void
  (e: 'update-filter', payload: { key: DomainDetailsFilterKey; value: string }): void
  (e: 'clear-filters'): void
  (e: 'toggle-sort', key: DomainDetailsSortKey): void
  (e: 'update:page', page: number): void
  (e: 'block-ip', payload: { ip: string; hostId: string }): void
}>()

const METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'HEAD', 'OPTIONS', 'PATCH']

const copiedIP = ref('')

function copyIP(ip: string): void {
  navigator.clipboard.writeText(ip).catch(() => {})
  copiedIP.value = ip
  setTimeout(() => {
    if (copiedIP.value === ip) copiedIP.value = ''
  }, 1500)
}

// Keeps the previous page's KPIs/table visible (instead of flashing back to
// the full-page spinner) while a filter/sort/page change re-fetches — only
// the very first load, before anything has come back yet, shows the spinner.
const hasData = computed(() => typeof props.details?.hits !== 'undefined')

function formatBytes(bytes: number): string {
  const value = Number(bytes) || 0
  if (value < 1024) return `${value} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  return `${size.toFixed(1)} ${units[unit]}`
}

function formatDate(v: string): string {
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v || '-'
  return d.toLocaleString()
}

function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'bg-green-lt text-green'
  if (status >= 300 && status < 400) return 'bg-yellow-lt text-yellow'
  if (status >= 400) return 'bg-red-lt text-red'
  return 'bg-secondary-lt text-secondary'
}
</script>

<style scoped>
.traffic-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-index-modal-overlay);
  padding: 1rem;
}

.traffic-modal {
  width: min(1200px, 96vw);
  max-height: 92vh;
  overflow: auto;
}

.kpi-btn,
.row-btn {
  background: transparent;
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

/* Hover fill comes from the shared .clickable-row utility (style.css) —
   same color used for every clickable row/card across Threats/Traffic, so
   this modal's filter affordances read the same way. .active stays a
   distinct, persistent "currently filtered on this" indicator (border +
   tint), not a hover state. */
.kpi-btn.active,
.row-btn.active {
  border-color: var(--tblr-primary) !important;
  background: rgba(var(--tblr-primary-rgb), 0.08);
}

.row-btn {
  border-left: none !important;
  border-right: none !important;
  border-top: none !important;
  border-radius: 0;
  text-align: left;
}

@media (max-width: 992px) {
  .traffic-modal-backdrop {
    padding: 0;
  }

  .traffic-modal {
    width: 100vw;
    max-height: 100dvh;
    height: 100dvh;
    border-radius: 0;
  }

  .traffic-modal-body {
    padding: 0.75rem;
  }

  .domain-path,
  .domain-ua {
    max-width: 12rem !important;
  }
}
</style>

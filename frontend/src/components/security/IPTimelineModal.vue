<template>
  <div
    v-if="show"
    class="timeline-drawer-backdrop"
    @click.self="$emit('close')"
  >
    <div class="timeline-modal card shadow-lg">
      <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap timeline-header">
        <div>
          <h3 class="card-title mb-0">
            Chronologie IP: <span class="font-monospace">{{ ip }}</span>
          </h3>
          <div class="text-secondary small">
            Chronologie des requêtes suspectes
          </div>
        </div>
        <div class="d-flex gap-2 flex-wrap timeline-header-actions">
          <template v-if="!blocked">
            <select
              v-model="banDuration"
              class="form-select form-select-sm"
              style="width: auto;"
            >
              <option value="1h">
                1h
              </option>
              <option value="4h">
                4h
              </option>
              <option value="24h">
                24h
              </option>
              <option value="48h">
                48h
              </option>
              <option value="168h">
                7j
              </option>
            </select>
            <button
              class="btn btn-sm"
              :class="banError ? 'btn-danger' : 'btn-outline-danger'"
              :disabled="banLoading || !hostId"
              :title="!hostId ? 'Hôte non déterminé — renseigne le filtre Hôte' : ''"
              @click="handleBanClick"
            >
              <span
                v-if="banLoading"
                class="spinner-border spinner-border-sm me-1"
              />
              <span v-if="banLoading">Blocage…</span>
              <span v-else-if="banError">Erreur — Réessayer</span>
              <span v-else>Bloquer (CrowdSec)</span>
            </button>
          </template>
          <span
            v-else
            class="badge bg-success-lt text-success align-self-center"
          >
            IP bloquée par CrowdSec
          </span>
          <button
            class="btn btn-sm btn-outline-secondary"
            @click="$emit('close')"
          >
            Fermer
          </button>
        </div>
      </div>

      <div class="card-body p-0 timeline-body">
        <div
          v-if="loading"
          class="text-center py-4 text-secondary"
        >
          <span class="spinner-border spinner-border-sm me-2" />
          Chargement chronologie...
        </div>
        <div
          v-else-if="!timeline.length"
          class="text-center py-4 text-secondary"
        >
          Aucune requête
        </div>
        <template v-else>
          <div class="timeline-frieze border-bottom px-3 py-3">
            <div class="timeline-controls d-flex align-items-center justify-content-between mb-2 gap-2">
              <div class="timeline-interval-row">
                <span class="small text-secondary timeline-interval-label">Intervalle:</span>
                <div
                  class="timeline-interval-chips"
                  role="group"
                  aria-label="Intervalle timeline"
                >
                  <button
                    v-for="opt in timelineIntervalOptions"
                    :key="opt.value"
                    class="timeline-interval-chip btn btn-sm"
                    :class="selectedInterval === opt.value ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="setTimelineInterval(opt.value)"
                  >
                    {{ opt.label }}
                  </button>
                </div>
              </div>
              <div class="small text-secondary">
                Regroupement: {{ timelineBucketLabel }} · {{ timelineBuckets.length }} tranches
                <span
                  v-if="selectedInterval === 'auto'"
                  class="badge bg-azure-lt text-azure ms-1"
                >
                  Auto cible ~{{ AUTO_BUCKET_TARGET }}
                </span>
              </div>
              <button
                class="btn btn-sm btn-outline-secondary"
                @click="toggleBucketFilter"
              >
                {{ bucketFilterEnabled ? 'Mode focus: tranche sélectionnée' : 'Mode global: toutes les tranches' }}
              </button>
            </div>

            <details
              class="timeline-controls-collapsible"
              open
            >
              <summary class="timeline-controls-toggle">
                <span>Statistiques</span>
                <span class="timeline-controls-toggle-arrow">▾</span>
              </summary>
              <div class="timeline-kpis mb-3">
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Requêtes affichées</span>
                  <span class="timeline-kpi-value">{{ timelineStats.total }}</span>
                </div>
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Erreurs</span>
                  <span class="timeline-kpi-value text-red">{{ timelineStats.errors }}</span>
                </div>
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Chemins uniques</span>
                  <span class="timeline-kpi-value">{{ timelineStats.uniquePaths }}</span>
                </div>
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Domaines cibles uniques</span>
                  <span class="timeline-kpi-value">{{ timelineStats.uniqueVhosts }}</span>
                </div>
              </div>
              <div class="timeline-status-breakdown mb-3">
                <span
                  v-for="item in timelineStatusBreakdown"
                  :key="item.key"
                  class="badge"
                  :class="item.badgeClass"
                >
                  {{ item.label }}: {{ item.count }}
                </span>
              </div>
            </details>

            <div class="timeline-frieze-scroll">
              <div class="timeline-frieze-track">
                <div class="timeline-frieze-line" />
                <button
                  v-for="bucket in timelineBuckets"
                  :key="bucket.key"
                  class="timeline-frieze-item"
                  :class="{ active: selectedBucketKey === bucket.key }"
                  :title="bucket.title"
                  @click="selectBucket(bucket.key)"
                >
                  <span
                    class="timeline-frieze-dot"
                    :class="bucketToneClass(bucket)"
                  />
                  <span class="timeline-frieze-time">{{ bucket.label }}</span>
                  <span class="timeline-frieze-count">{{ bucket.count }}</span>
                </button>
              </div>
            </div>

            <div
              v-if="selectedBucket"
              class="small text-secondary mt-2"
            >
              Tranche sélectionnée: {{ selectedBucket.rangeLabel }} · {{ selectedBucket.count }} requête{{ selectedBucket.count > 1 ? 's' : '' }} · {{ selectedBucket.errorCount }} erreur{{ selectedBucket.errorCount > 1 ? 's' : '' }}
            </div>
          </div>

          <div class="timeline-groups">
            <div
              v-for="group in groupedTimeline"
              :key="group.key"
              class="timeline-group border-bottom"
            >
              <div class="timeline-group-header px-3 py-2">
                <div class="d-flex align-items-center gap-2 flex-wrap">
                  <span class="badge bg-azure-lt text-azure">{{ group.label }}</span>
                  <span class="small text-secondary">{{ group.rangeLabel }}</span>
                </div>
                <div class="timeline-group-kpis">
                  <span class="badge bg-blue-lt text-blue">{{ group.count }} req</span>
                  <span class="badge bg-red-lt text-red">{{ group.errorCount }} erreurs</span>
                  <span class="badge bg-yellow-lt text-yellow">{{ group.uniquePaths }} chemins</span>
                  <span class="badge bg-indigo-lt text-indigo">{{ group.uniqueVhosts }} domaines</span>
                </div>
              </div>

              <div class="timeline-group-events px-3 pb-3">
                <section
                  v-for="statusGroup in group.statusGroups"
                  :key="`${group.key}-${statusGroup.key}`"
                  class="timeline-status-group"
                >
                  <div class="timeline-status-group-header">
                    <span
                      class="badge"
                      :class="statusGroup.badgeClass"
                    >{{ statusGroup.label }}</span>
                    <span class="small text-secondary">{{ statusGroup.count }} log{{ statusGroup.count > 1 ? 's' : '' }}</span>
                  </div>

                  <div class="timeline-status-group-grid">
                    <article
                      v-for="(r, idx) in statusGroup.events"
                      :key="`${group.key}-${statusGroup.key}-${r.timestamp}-${idx}`"
                      class="timeline-event-card"
                    >
                      <div class="timeline-event-topline">
                        <div class="d-flex align-items-center gap-2 flex-wrap">
                          <span
                            class="badge"
                            :class="statusClass(r.status)"
                          >{{ r.status }}</span>
                          <span class="badge bg-blue-lt text-blue">{{ r.method }}</span>
                          <span class="badge bg-azure-lt text-azure">{{ r.source || 'log' }}</span>
                        </div>
                        <span class="small text-secondary">{{ formatDate(r.timestamp) }}</span>
                      </div>
                      <div class="timeline-event-path font-monospace small">
                        {{ r.domain || '(unknown)' }} {{ r.path }}
                      </div>
                      <div class="timeline-event-meta small text-secondary">
                        <span><strong>Domaine:</strong> {{ r.domain || '-' }}</span>
                        <span><strong>Hôte:</strong> {{ r.host_name || '-' }}</span>
                        <span
                          class="text-truncate"
                          :title="r.user_agent || '-'"
                        >
                          <strong>User-Agent:</strong> {{ r.user_agent || '-' }}
                        </span>
                        <span v-if="r.blocked">
                          <strong>Blocage:</strong>
                          <span
                            class="badge bg-success-lt text-success ms-1"
                            :title="r.blocked_reason || '-'"
                          >
                            {{ r.blocked_source || 'crowdsec' }}
                          </span>
                          <span
                            v-if="r.blocked_reason"
                            class="ms-1"
                          >
                            {{ truncate(r.blocked_reason, 64) }}
                          </span>
                        </span>
                        <span v-if="r.blocked_until">
                          <strong>Expire:</strong> {{ formatDate(String(r.blocked_until)) }}
                        </span>
                      </div>
                    </article>
                  </div>
                </section>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { useIpTimeline } from '../../composables/useIpTimeline'
import type { WebLogIPTimelineRow } from '../../types/security'

const props = defineProps({
  show: { type: Boolean, default: false },
  ip: { type: String, default: '' },
  timeline: { type: Array as () => WebLogIPTimelineRow[], default: () => [] },
  loading: { type: Boolean, default: false },
  blocked: { type: Boolean, default: false },
  banLoading: { type: Boolean, default: false },
  banError: { type: Boolean, default: false },
  hostId: { type: String, default: '' },
})

const emit = defineEmits(['close', 'ban'])

const dialog = useConfirmDialog()
const banDuration = ref('4h')

const {
  selectedInterval,
  selectedBucketKey,
  bucketFilterEnabled,
  timelineIntervalOptions,
  AUTO_BUCKET_TARGET,
  timelineBucketLabel,
  timelineBuckets,
  selectedBucket,
  timelineStats,
  timelineStatusBreakdown,
  groupedTimeline,
  setTimelineInterval,
  selectBucket,
  toggleBucketFilter,
  bucketToneClass,
  statusClass,
  formatDate,
  truncate,
} = useIpTimeline(() => props.timeline, () => props.show)

async function handleBanClick() {
  const confirmed = await dialog.confirm({
    title: `Bloquer l'IP ${props.ip}`,
    message: `Bloquer ${props.ip} via CrowdSec pour ${banDuration.value} ?`,
    variant: 'danger',
  })
  if (!confirmed) return
  emit('ban', banDuration.value)
}
</script>

<style scoped>
.timeline-drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(var(--tblr-dark-rgb, 15, 23, 42), 0.58);
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0.75rem;
  z-index: 1060;
}

.timeline-modal {
  width: min(1400px, 98vw);
  height: min(96dvh, 960px);
  border-radius: 0.75rem;
  border: 1px solid var(--tblr-border-color);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.timeline-header {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--tblr-bg-surface);
  border-bottom: 1px solid var(--tblr-border-color);
}

.timeline-header-actions {
  justify-content: flex-end;
}

.timeline-frieze-scroll {
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.timeline-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  background: linear-gradient(
    180deg,
    rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.04) 0%,
    rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.015) 100%
  );
}

.timeline-frieze {
  position: sticky;
  top: 0;
  z-index: 1;
  backdrop-filter: blur(3px);
  background: linear-gradient(180deg, var(--tblr-bg-surface-secondary, #f8fafc) 0%, var(--tblr-bg-surface, #ffffff) 100%);
}

.timeline-interval-row {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.timeline-interval-chips {
  display: flex;
  gap: 0.3rem;
  flex-wrap: nowrap;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  padding-bottom: 2px;
}

.timeline-interval-chips::-webkit-scrollbar {
  display: none;
}

.timeline-interval-chip {
  flex: 0 0 auto;
  scroll-snap-align: start;
  border-radius: 1rem;
  min-width: 2.5rem;
  padding: 0.2rem 0.6rem;
  font-size: 0.78rem;
}

.timeline-controls-collapsible {
  /* transparent wrapper */
}

.timeline-controls-collapsible > summary {
  display: none;
  list-style: none;
}

.timeline-controls-collapsible > summary::-webkit-details-marker {
  display: none;
}

.timeline-controls-toggle-arrow {
  display: inline-block;
  transition: transform 180ms ease;
}

.timeline-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.5rem;
}

.timeline-kpi-chip {
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.5rem;
  padding: 0.45rem 0.6rem;
  background: var(--tblr-bg-surface-secondary, #f8fafc);
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.timeline-kpi-label {
  font-size: 0.72rem;
  color: var(--tblr-secondary);
}

.timeline-kpi-value {
  font-weight: 700;
  font-size: 1rem;
}

.timeline-frieze-track {
  position: relative;
  min-width: max-content;
  display: flex;
  gap: 1rem;
  padding: 1.25rem 0.25rem 0.2rem;
}

.timeline-frieze-line {
  position: absolute;
  left: 0;
  right: 0;
  top: 1.9rem;
  height: 4px;
  border-radius: 4px;
  background: linear-gradient(90deg, var(--tblr-primary) 0%, var(--tblr-azure) 100%);
}

.timeline-frieze-item {
  position: relative;
  z-index: 1;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: var(--tblr-bg-surface-secondary, #f8fafc);
  color: inherit;
  min-width: 64px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.25rem;
  transition: transform 120ms ease, border-color 120ms ease, background-color 120ms ease;
}

.timeline-frieze-item:hover {
  border-color: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.35);
  background: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.08);
  transform: translateY(-1px);
}

.timeline-frieze-dot {
  display: inline-block;
  border-radius: 999px;
  background: var(--tblr-bg-surface, #ffffff);
  width: 14px;
  height: 14px;
  border: 3px solid var(--tblr-primary);
  box-shadow: 0 1px 0 rgba(var(--tblr-dark-rgb, 15, 23, 42), 0.15);
}

.timeline-frieze-dot.is-calm {
  border-color: var(--tblr-blue);
  background: var(--tblr-blue-lt);
}

.timeline-frieze-dot.is-warm {
  border-color: var(--tblr-yellow);
  background: var(--tblr-yellow-lt);
}

.timeline-frieze-dot.is-hot {
  border-color: var(--tblr-red);
  background: var(--tblr-red-lt);
}

.timeline-frieze-item.active .timeline-frieze-dot {
  border-color: var(--tblr-red);
}

.timeline-frieze-item.active {
  border-color: rgba(var(--tblr-red-rgb, 214, 57, 57), 0.35);
  background: rgba(var(--tblr-red-rgb, 214, 57, 57), 0.08);
}

.timeline-frieze-time {
  font-size: 0.72rem;
  color: var(--tblr-secondary);
}

.timeline-frieze-count {
  font-size: 0.72rem;
  font-weight: 600;
}

.timeline-groups {
  display: flex;
  flex-direction: column;
}

.timeline-group {
  background: var(--tblr-bg-surface, #ffffff);
}

.timeline-group-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
  background: var(--tblr-bg-surface-secondary, #f8fafc);
  border-bottom: 1px solid var(--tblr-border-color);
}

.timeline-group-kpis {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.timeline-group-events {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.timeline-status-breakdown {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}

.timeline-status-group {
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.6rem;
  padding: 0.5rem;
  background: var(--tblr-bg-surface, #ffffff);
}

.timeline-status-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  margin-bottom: 0.5rem;
}

.timeline-status-group-grid {
  display: grid;
  gap: 0.55rem;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
}

.timeline-event-card {
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.6rem;
  padding: 0.55rem 0.65rem;
  background: var(--tblr-bg-surface, #ffffff);
}

.timeline-event-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  flex-wrap: wrap;
  margin-bottom: 0.35rem;
}

.timeline-event-path {
  margin-bottom: 0.35rem;
  overflow-wrap: anywhere;
}

.timeline-event-meta {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.25rem;
}

@media (max-width: 992px) {
  .timeline-modal {
    width: min(1400px, 100vw);
    height: 100dvh;
    border-radius: 0;
  }

  .timeline-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .timeline-modal {
    height: 100dvh;
    border-radius: 0;
  }

  .timeline-header.card-header {
    padding-top: 0.5rem;
    padding-bottom: 0.5rem;
  }

  .timeline-header .text-secondary.small {
    display: none;
  }

  .timeline-header .card-title {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: 0.9rem;
  }

  .timeline-header-actions {
    width: 100%;
    flex-wrap: nowrap;
    gap: 0.4rem;
  }

  .timeline-header-actions .form-select {
    flex: 0 0 auto;
    width: auto;
    min-width: 0;
    font-size: 0.8rem;
    padding-inline: 0.4rem;
  }

  .timeline-header-actions .btn-outline-secondary {
    flex: 0 0 auto;
  }

  .timeline-header-actions .btn:not(.btn-outline-secondary) {
    flex: 1 1 auto;
    font-size: 0.78rem;
    padding: 0.25rem 0.5rem;
  }

  .timeline-interval-label {
    display: none;
  }

  .timeline-controls {
    flex-direction: column;
    align-items: stretch;
    gap: 0.35rem;
  }

  .timeline-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.3rem;
  }

  .timeline-kpi-chip {
    padding: 0.3rem 0.45rem;
  }

  .timeline-kpi-label {
    font-size: 0.67rem;
  }

  .timeline-kpi-value {
    font-size: 0.85rem;
  }

  .timeline-controls-collapsible > summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.3rem 0;
    margin-bottom: 0.4rem;
    cursor: pointer;
    font-size: 0.78rem;
    color: var(--tblr-secondary);
    border-bottom: 1px solid var(--tblr-border-color);
    user-select: none;
    -webkit-tap-highlight-color: transparent;
  }

  .timeline-controls-collapsible:not([open]) .timeline-controls-toggle-arrow {
    transform: rotate(-90deg);
  }

  .timeline-frieze {
    padding: 0.5rem 0.75rem;
  }

  .timeline-frieze-item {
    min-width: 52px;
    padding: 0.15rem 0.2rem;
  }

  .timeline-frieze-dot {
    width: 11px;
    height: 11px;
    border-width: 2px;
  }

  .timeline-frieze-time,
  .timeline-frieze-count {
    font-size: 0.65rem;
  }

  .timeline-group-header {
    padding-top: 0.35rem;
    padding-bottom: 0.35rem;
  }

  .timeline-group-kpis .badge:nth-child(n+3) {
    display: none;
  }

  .timeline-group-events {
    gap: 0.6rem;
  }

  .timeline-status-group-grid {
    grid-template-columns: 1fr;
  }

  .timeline-event-card {
    padding: 0.4rem 0.5rem;
  }

  .timeline-event-topline {
    margin-bottom: 0.2rem;
  }

  .timeline-event-path {
    margin-bottom: 0.2rem;
  }

  .timeline-event-meta {
    gap: 0.15rem;
  }
}
</style>

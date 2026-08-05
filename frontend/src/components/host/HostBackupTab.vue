<template>
  <div class="card">
    <div class="card-header d-flex flex-wrap align-items-center justify-content-between gap-2">
      <h3 class="card-title mb-0">
        Sauvegardes Restic
      </h3>
      <div class="d-flex flex-wrap align-items-center gap-2">
        <span
          v-if="statusBadge"
          class="badge text-nowrap flex-shrink-0"
          :class="statusBadge.badgeClass"
        >{{ statusBadge.label }}</span>
        <RestrictedSelect
          v-if="canRun"
          v-model="runProfile"
          :options="resticOptionGroups"
          empty-label="Profil (défaut)"
          placeholder="Profil (défaut)"
          style="min-width: 160px; width: auto"
          :disabled="backupLoading === 'run' || liveStatus === 'running'"
          :title="resticGroups.length ? 'Un groupe lance plusieurs profils en une seule exécution.' : undefined"
        />
        <button
          v-if="canRun"
          type="button"
          class="btn btn-sm btn-primary"
          :disabled="backupLoading === 'run' || liveStatus === 'running'"
          @click="onRunBackup"
        >
          <span
            v-if="backupLoading === 'run' || liveStatus === 'running'"
            class="spinner-border spinner-border-sm me-1"
          />
          Lancer un backup
        </button>
      </div>
    </div>

    <div class="card-body">
      <!-- Statut -->
      <div
        v-if="latestRun || passiveState"
        class="row row-cards mb-3"
      >
        <div class="col-md-3">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="text-secondary small">
                Dernier backup
              </div>
              <div class="fw-semibold">
                {{ formatDate(latestRun?.finished_at || latestRun?.started_at || passiveState?.last_run_at) }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-3">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="text-secondary small">
                Durée
              </div>
              <div class="fw-semibold">
                {{ latestRun?.duration_sec != null ? formatDuration(latestRun.duration_sec) : '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-3">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="text-secondary small">
                Snapshot
              </div>
              <div
                class="fw-semibold text-truncate"
                :title="latestRun?.snapshot_id || passiveState?.snapshot_id"
              >
                {{ (latestRun?.snapshot_id || passiveState?.snapshot_id) || '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-3">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="text-secondary small">
                Profil
              </div>
              <div class="fw-semibold">
                {{ latestRun?.profile || 'défaut' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <EmptyState
        v-else-if="!liveStatus || liveStatus === 'idle'"
        title="Aucune donnée de sauvegarde"
        message="Lancez un backup manuellement ou attendez le prochain rapport de l'agent."
      />

      <!-- Progression live -->
      <div
        v-if="liveStatus !== 'idle'"
        class="mt-3"
      >
        <div class="d-flex align-items-center justify-content-between mb-1">
          <span class="text-secondary small">
            {{ liveStatus === 'running' ? 'Backup en cours…' : liveStatus === 'completed' ? 'Backup terminé' : 'Backup en échec' }}
          </span>
          <span
            v-if="liveProgress?.eta_seconds"
            class="text-secondary small"
          >ETA : {{ formatDuration(liveProgress.eta_seconds) }}</span>
        </div>
        <div
          v-if="liveProgress?.percent_done != null"
          class="progress mb-2"
          style="height: 8px"
        >
          <div
            class="progress-bar"
            :class="liveStatus === 'failed' ? 'bg-danger' : 'bg-primary'"
            :style="{ width: Math.min(100, Math.max(0, liveProgress.percent_done)) + '%' }"
          />
        </div>
        <div
          v-else
          class="d-flex align-items-center gap-2 mb-2 text-secondary small"
        >
          <span
            v-if="liveStatus === 'running'"
            class="spinner-border spinner-border-sm"
          />
          En attente de progression…
        </div>
        <div
          v-if="liveProgress"
          class="text-secondary small mb-2"
        >
          {{ liveProgress.files_done ?? 0 }}<span v-if="liveProgress.files_total">/{{ liveProgress.files_total }}</span> fichiers
          — {{ formatBytes(liveProgress.bytes_done) }}<span v-if="liveProgress.bytes_total"> / {{ formatBytes(liveProgress.bytes_total) }}</span>
        </div>
        <pre
          v-if="liveLogLines.length"
          class="bg-dark text-white p-2 rounded small backup-log"
        >{{ liveLogLines.join('\n') }}</pre>
      </div>

      <!-- Aide à la configuration -->
      <div class="mt-3">
        <a
          :href="configDocURL"
          target="_blank"
          rel="noopener noreferrer"
          class="btn btn-sm btn-ghost-secondary"
        >
          <IconExternalLink :size="14" />
          Aide à la configuration
        </a>
      </div>

      <!-- Historique -->
      <div class="mt-4">
        <div class="fw-semibold small mb-2">
          Historique
        </div>
        <div
          v-if="backupRuns.length"
          class="table-responsive"
        >
          <table class="table table-sm table-vcenter card-table">
            <thead>
              <tr>
                <th>Date</th>
                <th>Cible</th>
                <th>Statut</th>
                <th>Durée</th>
                <th>Volume</th>
                <th>Déclencheur</th>
                <th>Erreur</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="run in backupRuns"
                :key="run.id"
              >
                <td class="text-nowrap small">
                  {{ formatDate(run.started_at) }}
                </td>
                <td class="small">
                  {{ run.profile || 'défaut' }}
                </td>
                <td>
                  <span
                    class="badge"
                    :class="runBadgeClass(run.status)"
                  >{{ run.status }}</span>
                </td>
                <td class="small">
                  {{ run.duration_sec != null ? formatDuration(run.duration_sec) : '—' }}
                </td>
                <td class="small">
                  {{ formatBytes(run.bytes_done) }}
                </td>
                <td class="small">
                  {{ run.triggered_by === 'scheduled_task' ? 'Planifié' : (run.triggered_by || 'Manuel') }}
                </td>
                <td
                  class="small text-truncate"
                  style="max-width: 240px"
                  :title="run.error_message"
                >
                  {{ run.error_message || '—' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div
          v-else
          class="text-secondary small"
        >
          Aucun backup enregistré pour le moment.
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { IconExternalLink } from '@tabler/icons-vue'
import dayjs from '../../utils/dayjs'
import EmptyState from '../EmptyState.vue'
import RestrictedSelect from '../RestrictedSelect.vue'
import type { OptionGroup } from '../RestrictedSelect.vue'
import { useBackup } from '../../composables/useBackup'
import { RESTIC_BACKUP_DOC_URL as configDocURL } from '../../utils/docLinks'

const props = withDefaults(defineProps<{
  hostId: string
  canRun?: boolean
}>(), {
  canRun: false,
})

const {
  backupStatus,
  backupRuns,
  resticProfiles,
  resticGroups,
  backupLoading,
  liveStatus,
  liveProgress,
  liveLogLines,
  loadBackupData,
  handleRunBackup,
  stopWatchingLiveBackup,
} = useBackup(props.hostId)

const runProfile = ref('')

const resticOptionGroups = computed<OptionGroup[]>(() => {
  const groups: OptionGroup[] = []
  if (resticProfiles.value.length) groups.push({ label: 'Profils', options: resticProfiles.value })
  if (resticGroups.value.length) groups.push({ label: 'Groupes', options: resticGroups.value })
  return groups
})

const latestRun = computed(() => backupStatus.value?.latest_run || null)
const passiveState = computed(() => backupStatus.value?.passive_state || null)

const statusBadge = computed(() => {
  const status = latestRun.value?.status || passiveState.value?.last_status
  if (!status) return null
  return { label: status, badgeClass: runBadgeClass(status) }
})

function runBadgeClass(status: string): string {
  switch (status) {
    case 'ok':
      return 'bg-success-lt text-success'
    case 'running':
      return 'bg-primary-lt text-primary'
    case 'warning':
      return 'bg-warning-lt text-warning'
    case 'error':
      return 'bg-danger-lt text-danger'
    default:
      return 'bg-secondary-lt text-secondary'
  }
}

function onRunBackup(): void {
  void handleRunBackup(runProfile.value.trim() || undefined)
}

function formatDate(date: string | null | undefined): string {
  if (!date) return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}min`
  const hours = Math.floor(minutes / 60)
  return `${hours}h${minutes % 60}min`
}

function formatBytes(bytes: number | null | undefined): string {
  if (bytes == null) return '—'
  const units = ['o', 'Ko', 'Mo', 'Go', 'To']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`
}

onMounted(() => {
  void loadBackupData()
})

onUnmounted(() => {
  stopWatchingLiveBackup()
})
</script>

<style scoped>
.backup-log {
  max-height: 240px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>

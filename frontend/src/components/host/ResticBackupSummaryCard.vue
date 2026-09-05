<template>
  <div>
    <div class="d-flex align-items-center gap-2 mb-3">
      <span
        class="badge"
        :class="summary.status === 'ok' ? 'bg-success-lt text-success' : 'bg-danger-lt text-danger'"
      >{{ summary.status }}</span>
      <span class="text-secondary small">{{ summary.profile || t('host.defaultProfileLabel') }}</span>
    </div>

    <div
      v-if="summary.error_message"
      class="alert alert-danger py-2 mb-3"
    >
      {{ summary.error_message }}
    </div>

    <dl class="row mb-0">
      <dt class="col-5 text-muted">
        {{ t('host.durationLabel') }}
      </dt>
      <dd class="col-7">
        {{ formatDuration(summary.duration_sec) }}
      </dd>

      <template v-if="summary.files_new != null || summary.files_changed != null">
        <dt class="col-5 text-muted">
          {{ t('host.filesLabel') }}
        </dt>
        <dd class="col-7">
          <span v-if="summary.files_new != null">{{ t('host.newFilesCount', { count: summary.files_new }, summary.files_new) }}</span>
          <span v-if="summary.files_new != null && summary.files_changed != null"> · </span>
          <span v-if="summary.files_changed != null">{{ t('host.changedFilesCount', { count: summary.files_changed }, summary.files_changed) }}</span>
        </dd>
      </template>

      <template v-if="summary.bytes_added != null">
        <dt class="col-5 text-muted">
          {{ t('host.volumeAddedLabel') }}
        </dt>
        <dd class="col-7">
          {{ formatBytes(summary.bytes_added) }}
        </dd>
      </template>

      <template v-if="summary.snapshot_id">
        <dt class="col-5 text-muted">
          Snapshot
        </dt>
        <dd
          class="col-7 text-truncate"
          :title="summary.snapshot_id"
        >
          {{ summary.snapshot_id }}
        </dd>
      </template>

      <template v-if="summary.repo_size_bytes != null">
        <dt class="col-5 text-muted">
          {{ t('host.repoSizeLabel') }}
        </dt>
        <dd class="col-7">
          {{ formatBytes(summary.repo_size_bytes) }}
        </dd>
      </template>
    </dl>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

// Mirrors agent/internal/collector.ResticBackupSummary — the terminal Output
// of a module=restic action=run_backup command. Same shape (and same field
// set) as the "Historique" row / summary card in HostBackupTab.vue, just
// rendered here for the one-off case of viewing this exact command's log
// entry via the generic console instead of the Sauvegardes tab.
export interface ResticBackupSummary {
  status: string
  profile?: string
  duration_sec: number
  files_new?: number
  files_changed?: number
  bytes_added?: number
  snapshot_id?: string
  repo_size_bytes?: number
  error_message?: string
}

defineProps<{
  summary: ResticBackupSummary
}>()

const { t } = useI18n()

// Duplicated from HostBackupTab.vue rather than shared — this codebase
// consistently keeps a small local formatBytes/formatDuration per
// component (20+ existing copies) rather than a single shared one.
function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}min`
  const hours = Math.floor(minutes / 60)
  return `${hours}h${minutes % 60}min`
}

function formatBytes(bytes: number | null | undefined): string {
  if (bytes == null) return '—'
  const units = [t('host.unitByte'), t('host.unitKB'), t('host.unitMB'), t('host.unitGB'), t('host.unitTB')]
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`
}
</script>

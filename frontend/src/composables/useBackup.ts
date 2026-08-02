import { ref } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { useCommandStream } from './useCommandStream'
import type { CommandStreamChunkMsg, CommandStatusUpdateMsg } from '../types/ws'

interface BackupRun {
  id: string
  host_id: string
  profile?: string
  command_id?: string
  triggered_by?: string
  status: string
  started_at: string
  finished_at?: string
  duration_sec?: number
  files_done?: number
  bytes_done?: number
  snapshot_id?: string
  repo_size_bytes?: number
  error_message?: string
  [key: string]: unknown
}

interface ResticPassiveState {
  installed: boolean
  last_run_at?: string
  last_status?: string
  snapshot_id?: string
  repo_size_bytes?: number
  error_message?: string
  source: string
}

interface BackupStatus {
  latest_run?: BackupRun | null
  passive_state?: ResticPassiveState | null
}

interface ResticProgressEvent {
  phase?: string
  percent_done?: number
  files_done?: number
  files_total?: number
  bytes_done?: number
  bytes_total?: number
  seconds_elapsed?: number
  eta_seconds?: number
}

export function useBackup(hostId: string) {
  const backupStatus = ref<BackupStatus | null>(null)
  const backupRuns = ref<BackupRun[]>([])
  const backupLoading = ref('')
  const backupError = ref('')

  const liveStatus = ref<'idle' | 'running' | 'completed' | 'failed'>('idle')
  const liveProgress = ref<ResticProgressEvent | null>(null)
  const liveLogLines = ref<string[]>([])

  const { openCommandStream, closeStream } = useCommandStream()

  async function loadBackupData(): Promise<void> {
    try {
      const [statusRes, runsRes] = await Promise.all([
        apiClient.getBackupStatus(hostId),
        apiClient.getBackupRuns(hostId),
      ])
      backupStatus.value = statusRes.data || null
      backupRuns.value = runsRes.data?.runs || []
    } catch {
      // non-critical — the tab still renders with empty state
    }
  }

  function resetLiveState(): void {
    liveStatus.value = 'idle'
    liveProgress.value = null
    liveLogLines.value = []
  }

  function handleStreamChunk(payload: CommandStreamChunkMsg): void {
    const chunk = payload.chunk || ''
    try {
      const parsed = JSON.parse(chunk) as ResticProgressEvent
      if (parsed && typeof parsed === 'object' && parsed.phase) {
        liveProgress.value = parsed
        return
      }
    } catch {
      // not JSON — a redacted text log line
    }
    liveLogLines.value.push(chunk)
    if (liveLogLines.value.length > 500) {
      liveLogLines.value = liveLogLines.value.slice(-500)
    }
  }

  function handleStreamStatus(payload: CommandStatusUpdateMsg): void {
    if (payload.status === 'completed') {
      liveStatus.value = 'completed'
    } else if (payload.status === 'failed') {
      liveStatus.value = 'failed'
    } else if (payload.status === 'running') {
      liveStatus.value = 'running'
    }
    if (payload.status === 'completed' || payload.status === 'failed') {
      // backup_runs is updated asynchronously by the server's completion
      // listener right after this status arrives — reload shortly after so
      // the history table/status card reflect the persisted result.
      window.setTimeout(() => {
        void loadBackupData()
      }, 1200)
    }
  }

  async function handleRunBackup(profile?: string): Promise<void> {
    backupLoading.value = 'run'
    backupError.value = ''
    resetLiveState()
    try {
      const res = await apiClient.runBackup(hostId, profile)
      const commandId = res.data?.command_id
      if (commandId) {
        liveStatus.value = 'running'
        openCommandStream(commandId, {
          onChunk: handleStreamChunk,
          onStatus: handleStreamStatus,
        })
      }
      await loadBackupData()
    } catch (e: unknown) {
      backupError.value = getApiErrorMessage(e)
    } finally {
      backupLoading.value = ''
    }
  }

  function stopWatchingLiveBackup(): void {
    closeStream()
  }

  return {
    backupStatus,
    backupRuns,
    backupLoading,
    backupError,
    liveStatus,
    liveProgress,
    liveLogLines,
    loadBackupData,
    handleRunBackup,
    stopWatchingLiveBackup,
  }
}

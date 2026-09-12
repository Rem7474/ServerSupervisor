import { ref, toValue, type MaybeRef } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { getApiErrorMessage } from '../api'
import { useCommandStream } from './useCommandStream'

export interface HostProcess {
  pid: number
  name: string
  user: string
  cpu_pct: number
  mem_pct: number
  mem_rss_kb: number
  state: string
}

const STREAM_TIMEOUT_MS = 60_000

export function useHostProcesses(hostId: MaybeRef<string>) {
  const { t } = useI18n()
  const processes = ref<HostProcess[]>([])
  const loading = ref(false)
  const error = ref('')
  const { collectCommandOutput } = useCommandStream()

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await api.sendProcessesCommand(toValue(hostId))
      const cmdId = res.data.command_id
      await collectCommandOutput(cmdId, { timeoutMs: STREAM_TIMEOUT_MS })
        .then((output: string) => {
          try {
            processes.value = JSON.parse(output)
          } catch {
            error.value = t('host.parseProcessesError')
          }
        })
        .catch((e: unknown) => {
          error.value = e instanceof Error ? e.message : t('host.loadProcessesError')
        })
    } catch (e) {
      error.value = getApiErrorMessage(e, t('host.sendCommandError'))
    } finally {
      loading.value = false
    }
  }

  return { processes, loading, error, load }
}

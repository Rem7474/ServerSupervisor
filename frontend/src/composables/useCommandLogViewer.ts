import { ref, type Ref, type UnwrapRef } from 'vue'
import { useCommandStream } from './useCommandStream'

/** The fields of a remote_commands row the log viewer reads and patches. */
export interface CommandLogRow {
  id: string
  status?: string
  output?: string
}

interface UseCommandLogViewerOptions {
  /**
   * Called on every status message of the followed command (init included),
   * so a caller can keep its own list row in step with the console.
   */
  onStatus?: (commandId: string, status: string, output?: string) => void
}

interface UseCommandLogViewerApi<T extends CommandLogRow> {
  selected: Ref<UnwrapRef<T> | null>
  visible: Ref<boolean>
  show: (command: T) => void
  close: () => void
}

function isLive(status: string | undefined): boolean {
  return status === 'pending' || status === 'running'
}

/**
 * Console for one remote command's output: shows the row it is given and,
 * while the command is still pending/running, follows its live stream
 * (/ws/commands/stream/:id), patching status and output as they arrive.
 * Messages for a command that is no longer the selected one are dropped, so
 * switching rows mid-stream never mixes two outputs.
 */
export function useCommandLogViewer<T extends CommandLogRow>(options: UseCommandLogViewerOptions = {}): UseCommandLogViewerApi<T> {
  const { openCommandStream, closeStream } = useCommandStream()
  const selected = ref(null) as Ref<UnwrapRef<T> | null>
  const visible = ref(false)

  function patch(commandId: string, next: Partial<CommandLogRow>): void {
    if (!selected.value || selected.value.id !== commandId) return
    selected.value = { ...selected.value, ...next } as UnwrapRef<T>
  }

  function follow(commandId: string): void {
    openCommandStream(commandId, {
      onInit(p) {
        patch(commandId, { status: p.status || selected.value?.status, output: p.output ?? selected.value?.output })
        if (p.status) options.onStatus?.(commandId, p.status, p.output)
      },
      onChunk(p) {
        patch(commandId, { output: (selected.value?.output || '') + (p.chunk || '') })
      },
      onStatus(p) {
        patch(commandId, { status: p.status || selected.value?.status, output: p.output ?? selected.value?.output })
        if (p.status) options.onStatus?.(commandId, p.status, p.output || undefined)
      },
    })
  }

  function show(command: T): void {
    closeStream()
    selected.value = { ...command } as UnwrapRef<T>
    visible.value = true
    if (isLive(command.status)) follow(command.id)
  }

  function close(): void {
    closeStream()
    selected.value = null
    visible.value = false
  }

  return { selected, visible, show, close }
}

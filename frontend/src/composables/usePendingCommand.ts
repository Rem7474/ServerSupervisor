import { ref } from 'vue'
import { useCommandStream } from './useCommandStream'

/**
 * Tracks dispatched-but-not-yet-terminal `remote_commands` so a button's
 * loading/disabled state can span the real operation instead of just the
 * dispatch HTTP round-trip. Extracted from useBackup.ts's liveStatus, the one
 * call site that already gated its button on real completion (via the WS
 * command-stream) rather than `loading = true; await axios(); loading = false`
 * — the shape every other mutating action (apt, docker, systemd, agent
 * update, scheduled tasks, …) independently reinvented, stopping the spinner
 * the instant the dispatch ack came back rather than when the agent/PVE-side
 * operation actually finished.
 *
 * Deliberately separate from any console/live-output UI a caller might also
 * open for the same command (openCommand/connectStream) — this composable
 * only tracks "is it still running", nothing about display. A caller with no
 * console at all (docker, systemd, scheduled tasks) can use it standalone.
 */
export function usePendingCommand() {
  const pendingIds = ref<Set<string>>(new Set())
  const { collectCommandOutput } = useCommandStream()

  function isPending(commandId: string | number | null | undefined): boolean {
    return commandId != null && pendingIds.value.has(String(commandId))
  }

  /**
   * Awaits the given command's terminal status (completed/failed) before
   * resolving. Never throws — a failed/timed-out command is still "no longer
   * pending" as far as button state goes; the caller's own error/toast/
   * console handling (unaffected by this composable) covers surfacing it.
   *
   * timeoutMs defaults high (10 min) relative to collectCommandOutput's own
   * 20s default: that default suits a quick status read, not an apt upgrade
   * or an agent self-update that can legitimately run for minutes.
   */
  async function track(
    commandId: string | number | null | undefined,
    options: { timeoutMs?: number } = {}
  ): Promise<void> {
    if (commandId == null) return
    const id = String(commandId)
    pendingIds.value.add(id)
    try {
      await collectCommandOutput(id, { timeoutMs: options.timeoutMs ?? 600_000 })
    } catch {
      // failed / timed out / stream closed early — see doc comment above.
    } finally {
      pendingIds.value.delete(id)
    }
  }

  return { isPending, track }
}

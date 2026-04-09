import { ref, Ref } from 'vue'

type CommandStatus = 'pending' | 'running' | 'completed' | 'failed' | 'success' | 'error'

interface NormalizedHostCommand {
  id: string | number
  host_name?: string
  module?: string
  action?: string
  target?: string
  prefix: string
  command: string
  status: CommandStatus | string
  output: string
}

interface RawCommand {
  [key: string]: unknown
}

function toText(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : value == null ? fallback : String(value)
}

function toCommandId(value: unknown, fallback = 'tmp-command'): string | number {
  if (typeof value === 'string' || typeof value === 'number') return value
  return fallback
}

interface UseHostCommandConsoleApi {
  liveCommand: Ref<NormalizedHostCommand | null>
  showConsole: Ref<boolean>
  openCommand: (command: RawCommand | null | undefined) => void
  closeConsole: () => void
  updateCommand: (command: NormalizedHostCommand | null) => void
  normalizeCommand: (command: RawCommand | null | undefined) => NormalizedHostCommand | null
}

export function useHostCommandConsole(): UseHostCommandConsoleApi {
  const liveCommand: Ref<NormalizedHostCommand | null> = ref(null)
  const showConsole: Ref<boolean> = ref(false)

  /**
   * Normalizes multiple command payload shapes into a single console-friendly shape.
   */
  function normalizeCommand(command: RawCommand | null | undefined): NormalizedHostCommand | null {
    if (!command) return null

    const host_name = toText(command.host_name)

    if (command.commandId) {
      return {
        id: toCommandId(command.commandId),
        host_name,
        module: toText(command.module, 'custom'),
        action: toText(command.action, toText(command.command)),
        target: toText(command.target),
        prefix: toText(command.prefix),
        command: toText(command.command),
        status: toText(command.status, 'running'),
        output: toText(command.output),
      }
    }

    if (command.command && Object.prototype.hasOwnProperty.call(command, 'prefix')) {
      return {
        id: toCommandId(command.id),
        host_name,
        module: toText(command.module, 'custom'),
        action: toText(command.action, toText(command.command)),
        target: toText(command.target),
        prefix: toText(command.prefix),
        command: toText(command.command),
        status: toText(command.status, 'pending'),
        output: toText(command.output),
      }
    }

    let displayCommand: string
    let prefix = ''
    const module = toText(command.module, 'apt')
    if (module === 'apt') {
      prefix = 'apt '
      displayCommand = toText(command.action, toText(command.command))
    } else if (module === 'journal') {
      displayCommand = `journalctl -u ${toText(command.target, toText(command.container_name))}`
    } else {
      displayCommand = `${toText(command.action)} ${toText(command.target, toText(command.container_name))}`.trim()
    }

    return {
      id: toCommandId(command.id),
      host_name,
      module,
      action: toText(command.action),
      target: toText(command.target, toText(command.container_name)),
      prefix,
      command: displayCommand,
      status: toText(command.status, 'pending'),
      output: toText(command.output),
    }
  }

  function openCommand(command: RawCommand | null | undefined): void {
    liveCommand.value = normalizeCommand(command)
    showConsole.value = true
  }

  function closeConsole(): void {
    showConsole.value = false
    liveCommand.value = null
  }

  function updateCommand(command: NormalizedHostCommand | null): void {
    liveCommand.value = command
  }

  return {
    liveCommand,
    showConsole,
    openCommand,
    closeConsole,
    updateCommand,
    normalizeCommand,
  }
}

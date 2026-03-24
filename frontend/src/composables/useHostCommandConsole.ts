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
  [key: string]: any
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

    const host_name = command.host_name || ''

    if (command.commandId) {
      return {
        id: command.commandId,
        host_name,
        module: command.module || 'custom',
        action: command.action || command.command || '',
        target: command.target || '',
        prefix: command.prefix || '',
        command: command.command || '',
        status: command.status || 'running',
        output: command.output || '',
      }
    }

    if (command.command && Object.prototype.hasOwnProperty.call(command, 'prefix')) {
      return {
        id: command.id,
        host_name,
        module: command.module || 'custom',
        action: command.action || command.command || '',
        target: command.target || '',
        prefix: command.prefix || '',
        command: command.command || '',
        status: command.status || 'pending',
        output: command.output || '',
      }
    }

    let displayCommand: string
    let prefix = ''
    const module = command.module || 'apt'
    if (module === 'apt') {
      prefix = 'apt '
      displayCommand = command.action || command.command
    } else if (module === 'journal') {
      displayCommand = `journalctl -u ${command.target || command.container_name}`
    } else {
      displayCommand = `${command.action || ''} ${command.target || command.container_name || ''}`.trim()
    }

    return {
      id: command.id,
      host_name,
      module,
      action: command.action || '',
      target: command.target || command.container_name || '',
      prefix,
      command: displayCommand,
      status: command.status || 'pending',
      output: command.output || '',
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

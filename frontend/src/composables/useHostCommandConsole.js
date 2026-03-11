import { ref } from 'vue'

/**
 * @typedef {'pending' | 'running' | 'completed' | 'failed' | 'success' | 'error'} CommandStatus
 */

/**
 * @typedef {{
 *  id: string | number,
 *  prefix: string,
 *  command: string,
 *  status: CommandStatus | string,
 *  output: string,
 * }} NormalizedHostCommand
 */

/**
 * @typedef {Record<string, any>} RawCommand
 */

/**
 * @returns {{
 *  liveCommand: import('vue').Ref<NormalizedHostCommand | null>,
 *  showConsole: import('vue').Ref<boolean>,
 *  openCommand: (command: RawCommand | null | undefined) => void,
 *  closeConsole: () => void,
 *  updateCommand: (command: NormalizedHostCommand | null) => void,
 *  normalizeCommand: (command: RawCommand | null | undefined) => NormalizedHostCommand | null,
 * }}
 */
export function useHostCommandConsole() {
  const liveCommand = ref(null)
  const showConsole = ref(false)

  /**
   * Normalizes multiple command payload shapes into a single console-friendly shape.
   * @param {RawCommand | null | undefined} command
   * @returns {NormalizedHostCommand | null}
   */
  function normalizeCommand(command) {
    if (!command) return null

    if (command.commandId) {
      return {
        id: command.commandId,
        prefix: command.prefix || '',
        command: command.command || '',
        status: command.status || 'running',
        output: command.output || '',
      }
    }

    if (command.command && Object.prototype.hasOwnProperty.call(command, 'prefix')) {
      return {
        id: command.id,
        prefix: command.prefix || '',
        command: command.command || '',
        status: command.status || 'pending',
        output: command.output || '',
      }
    }

    let displayCommand
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
      prefix,
      command: displayCommand,
      status: command.status || 'pending',
      output: command.output || '',
    }
  }

  /** @param {RawCommand | null | undefined} command */
  function openCommand(command) {
    liveCommand.value = normalizeCommand(command)
    showConsole.value = true
  }

  function closeConsole() {
    showConsole.value = false
    liveCommand.value = null
  }

  /** @param {NormalizedHostCommand | null} command */
  function updateCommand(command) {
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

<template>
  <!-- Side panel -->
  <div
    v-show="show"
    :class="wrapperClass"
  >
    <div class="card d-flex flex-column h-100">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title">
          <IconChevronRight
            :size="24"
            class="icon icon-tabler me-1"
          />
          {{ title }}
        </h3>
        <div class="d-flex gap-1">
          <!-- Copy -->
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            :title="copied ? 'Copié !' : 'Copier la sortie'"
            :disabled="!command"
            @click="copy"
          >
            <IconCopy
              :size="18"
              class="icon"
            />
            <IconCheck
              :size="18"
              class="icon text-success"
            />
          </button>
          <!-- Download -->
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            title="Télécharger (.txt)"
            :disabled="!command"
            @click="download"
          >
            <IconDownload
              :size="18"
              class="icon"
            />
          </button>
          <!-- Clear (optional) -->
          <button
            v-if="clearable"
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            title="Vider la console"
            :disabled="!command"
            @click="$emit('clear')"
          >
            <IconTrash
              :size="18"
              class="icon"
            />
          </button>
          <!-- Close -->
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            title="Fermer"
            @click="$emit('close')"
          >
            <IconX
              :size="24"
              class="icon"
            />
          </button>
        </div>
      </div>

      <div class="card-body d-flex flex-column flex-fill p-0 console-body">
        <!-- Empty state -->
        <div
          v-if="!command"
          class="d-flex align-items-center justify-content-center flex-fill text-secondary console-empty"
        >
          <div class="text-center p-4">
            <IconChevronRight
              :size="48"
              class="icon icon-tabler mb-2 opacity-50"
              :stroke-width="1.5"
            />
            <div class="opacity-75">
              {{ emptyText }}
            </div>
            <div class="small mt-1 opacity-50">
              Cliquez sur "Logs" pour afficher la sortie d'une commande
            </div>
          </div>
        </div>

        <!-- Active viewer -->
        <div
          v-else
          class="d-flex flex-column flex-fill console-viewer"
        >
          <div class="console-header px-3 pt-3 pb-2">
            <div class="d-flex align-items-start justify-content-between mb-2">
              <div
                class="flex-fill"
                style="min-width: 0;"
              >
                <div
                  class="fw-semibold text-light"
                  style="font-size: 0.95rem;"
                >
                  {{ command.host_name || command.host_id }}
                </div>
                <div class="d-flex align-items-center gap-2 mt-1 flex-wrap">
                  <span :class="moduleClass(command.module || '')">{{ moduleLabel(command.module || '') }}</span>
                  <code class="console-cmd-label">
                    {{ cmdLabel(command) }}
                  </code>
                </div>
                <div
                  v-if="command.created_at"
                  class="text-secondary small mt-1"
                >
                  Exécutée {{ formatRelativeTime(command.created_at || '', '—', true) }}
                </div>
              </div>
              <span
                :class="statusClass(command.status)"
                class="ms-2"
              >{{ command.status }}</span>
            </div>
          </div>
          <pre
            ref="outputEl"
            class="console-output mb-0 flex-fill"
          >{{ outputText }}</pre>
        </div>
      </div>
    </div>
  </div>

  <!-- Floating reopen button -->
  <button
    v-show="!show"
    type="button"
    class="btn btn-primary console-fab"
    @click="$emit('open')"
  >
    <IconChevronRight
      :size="24"
      class="icon me-1"
    />
    {{ title }}
  </button>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { IconCheck, IconChevronRight, IconCopy, IconDownload, IconTrash, IconX } from '@tabler/icons-vue'
import { copyConsoleOutput, downloadConsoleOutput } from '../../utils/consoleOutput'
import { moduleLabel, moduleClass } from '../../utils/moduleMeta'
import { useStatusBadge } from '../../composables/useStatusBadge'
import { useDateFormatter } from '../../composables/useDateFormatter'

interface CommandRecord {
  host_name?: string
  host_id?: string
  module?: string
  action?: string
  target?: string
  status?: string
  output?: string
  created_at?: string
  [key: string]: unknown
}

const props = withDefaults(defineProps<{
  command?: CommandRecord | null
  show?: boolean
  title?: string
  emptyText?: string
  wrapperClass?: string
  clearable?: boolean
}>(), {
  command: null,
  show: false,
  title: 'Logs',
  emptyText: 'Aucun log sélectionné',
  wrapperClass: '',
  clearable: false,
})

defineEmits<{
  (e: 'close'): void
  (e: 'open'): void
  (e: 'clear'): void
}>()

const { getStatusBadgeClass } = useStatusBadge()
const { formatRelativeTime } = useDateFormatter()

const outputEl = ref<HTMLElement | null>(null)
const copied = ref(false)

function cmdLabel(cmd: CommandRecord): string {
  return [cmd.action, cmd.target].filter(Boolean).join(' ')
}

function statusClass(status: string | undefined): string {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}

function processCarriageReturns(text: string): string {
  const lines = []
  let currentLine = ''
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (ch === '\r') {
      if (i + 1 < text.length && text[i + 1] === '\n') {
        lines.push(currentLine)
        currentLine = ''
        i++
      } else {
        // carriage return alone: go back to start of current line (overwrite)
        currentLine = ''
      }
    } else if (ch === '\n') {
      lines.push(currentLine)
      currentLine = ''
    } else {
      currentLine += ch
    }
  }
  if (currentLine) lines.push(currentLine)
  return lines.join('\n')
}

const outputText = computed(() => {
  const raw = props.command?.output
  if (!raw) return 'Aucune sortie disponible.'
  return processCarriageReturns(raw)
})

// Scroll to bottom whenever output changes
watch(outputText, () => {
  nextTick(() => {
    if (outputEl.value) outputEl.value.scrollTop = outputEl.value.scrollHeight
  })
})

async function copy(): Promise<void> {
  await copyConsoleOutput(props.command?.output || '')
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function download(): void {
  const name = [props.command?.module, props.command?.action, props.command?.target]
    .filter(Boolean).join('-')
  downloadConsoleOutput(props.command?.output || '', `log-${name || 'command'}.txt`)
}
</script>

<style scoped>
.console-body {
  min-height: 0;
}

.console-viewer {
  min-height: 0;
}

.console-empty {
  background: var(--ss-panel-solid);
  border-radius: 0 0 0.5rem 0.5rem;
}

.console-header {
  background: var(--ss-panel-solid);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.console-cmd-label {
  background: rgba(0, 0, 0, 0.3);
  padding: 0.15rem 0.4rem;
  border-radius: 0.25rem;
  color: var(--ss-text-muted-on-dark);
}

.console-output {
  background: var(--ss-panel-solid-darker);
  color: var(--ss-text-on-dark);
  padding: 1rem;
  margin: 0;
  overflow: auto;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.813rem;
  line-height: 1.5;
  border-radius: 0 0 0.5rem 0.5rem;
  white-space: pre;
  word-break: normal;
  /* flex-fill handles the height — no max-height cap needed */
  min-height: 0;
}

.console-fab {
  position: fixed;
  bottom: 1.5rem;
  right: 1.5rem;
  z-index: 100;
}
</style>

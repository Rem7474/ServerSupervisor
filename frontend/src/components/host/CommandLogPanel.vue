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
          {{ displayTitle }}
          <slot name="title-suffix" />
        </h3>
        <div class="d-flex gap-1">
          <template v-if="mode === 'log'">
            <!-- Copy -->
            <button
              type="button"
              class="btn btn-sm btn-ghost-secondary"
              :title="copied ? t('common.commandLogCopiedTooltip') : t('common.commandLogCopyTooltip')"
              :disabled="!command"
              @click="copy"
            >
              <IconCopy
                v-if="!copied"
                :size="18"
                class="icon"
              />
              <IconCheck
                v-else
                :size="18"
                class="icon text-success"
              />
            </button>
            <!-- Download -->
            <button
              type="button"
              class="btn btn-icon btn-sm btn-ghost-secondary"
              :title="t('common.commandLogDownloadTooltip')"
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
              class="btn btn-icon btn-sm btn-ghost-secondary"
              :title="t('common.commandLogClearTooltip')"
              :disabled="!command"
              @click="$emit('clear')"
            >
              <IconTrash
                :size="18"
                class="icon"
              />
            </button>
          </template>
          <slot name="header-actions" />
          <!-- Close -->
          <button
            type="button"
            class="btn btn-icon btn-sm btn-ghost-secondary"
            :title="t('common.close')"
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
        <slot v-if="mode === 'custom'" />
        <template v-else>
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
                {{ displayEmptyText }}
              </div>
              <div class="small mt-1 opacity-50">
                {{ t('common.commandLogEmptyHint') }}
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
                    {{ t('common.commandLogExecutedPrefix') }} {{ formatRelativeTime(command.created_at || '', '—', true) }}
                  </div>
                </div>
                <span
                  :class="statusClass(command.status)"
                  class="ms-2"
                >{{ command.status }}</span>
              </div>
            </div>
            <div
              v-if="structuredOutput?.kind === 'processes'"
              class="console-processes flex-fill p-3"
            >
              <ProcessesTable :processes="structuredOutput.data" />
            </div>
            <div
              v-else-if="structuredOutput?.kind === 'systemd'"
              class="console-processes flex-fill p-3"
            >
              <SystemdTable
                :services="structuredOutput.data"
                readonly
              />
            </div>
            <div
              v-else-if="structuredOutput?.kind === 'restic_backup_summary'"
              class="console-processes flex-fill p-3"
            >
              <ResticBackupSummaryCard :summary="structuredOutput.data" />
            </div>
            <pre
              v-else
              ref="outputEl"
              class="console-output mb-0 flex-fill"
            >{{ outputText }}</pre>
          </div>
        </template>
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
    {{ displayTitle }}
  </button>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconCheck, IconChevronRight, IconCopy, IconDownload, IconTrash, IconX } from '@tabler/icons-vue'
import { copyConsoleOutput, downloadConsoleOutput } from '../../utils/consoleOutput'
import { moduleLabel, moduleClass } from '../../utils/moduleMeta'
import { useStatusBadge } from '../../composables/useStatusBadge'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { resolveStructuredOutput } from '../../utils/structuredCommandOutput'
import ProcessesTable from './ProcessesTable.vue'
import SystemdTable from './SystemdTable.vue'
import ResticBackupSummaryCard from './ResticBackupSummaryCard.vue'

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
  // 'log' (default) renders the command-output viewer this component was
  // built for. 'custom' hides the copy/download/clear buttons (meaningless
  // outside a command log) and renders the default slot instead — reused by
  // ProxmoxConsole.vue for a live terminal, so every "console" panel in the
  // app shares the same card/header/FAB shell instead of each screen
  // inventing its own.
  mode?: 'log' | 'custom'
}>(), {
  command: null,
  show: false,
  wrapperClass: '',
  clearable: false,
  mode: 'log',
})

defineEmits<{
  (e: 'close'): void
  (e: 'open'): void
  (e: 'clear'): void
}>()

const { t } = useI18n()
const { getStatusBadgeClass } = useStatusBadge()
const { formatRelativeTime } = useDateFormatter()

const displayTitle = computed(() => props.title || t('common.commandLogTitle'))
const displayEmptyText = computed(() => props.emptyText || t('common.commandLogEmptyText'))

const outputEl = ref<HTMLElement | null>(null)
const copied = ref(false)

function cmdLabel(cmd: CommandRecord): string {
  return [cmd.action, cmd.target].filter(Boolean).join(' ')
}

function statusClass(status: string | undefined): string {
  return getStatusBadgeClass(status, 'badge bg-warning-lt text-warning')
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
  if (!raw) return t('common.commandLogNoOutput')
  return processCarriageReturns(raw)
})

// Some module/action combinations report structured JSON instead of a human
// log (see utils/structuredCommandOutput.ts) — render those with a proper
// table instead of dumping the JSON as plain text. Falls back to the raw
// <pre> view while the command is still streaming (output isn't valid JSON
// yet) or for any module/action with no known structured shape.
const structuredOutput = computed(() => resolveStructuredOutput(props.command?.module, props.command?.action, props.command?.output))

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

.console-processes {
  background: var(--ss-panel-solid-darker);
  overflow: auto;
  border-radius: 0 0 0.5rem 0.5rem;
  min-height: 0;
}

/* ProcessesTable/SystemdTable's own .scroll-table (style.css) caps itself at
   70vh with its own overflow-y — fine as a standalone page table, but nested
   here inside .console-processes (which is already the scroll container for
   this panel) it produced a second, inner scrollbar. Let .console-processes
   be the single scrolling ancestor in this context; the table's sticky
   header still works, just pinned against the outer scroll instead. */
.console-processes :deep(.scroll-table) {
  max-height: none;
  overflow-y: visible;
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

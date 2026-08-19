<template>
  <CommandLogPanel
    mode="custom"
    :show="show"
    :title="`Console — ${guestName || guestId}`"
    wrapper-class="side-panel"
    @close="$emit('close')"
    @open="$emit('open')"
  >
    <template #title-suffix>
      <span
        :class="statusBadgeClass"
        class="ms-2"
      >{{ statusLabel }}</span>
    </template>
    <template #header-actions>
      <button
        v-if="status === 'disconnected' || status === 'error'"
        type="button"
        class="btn btn-sm btn-outline-secondary"
        @click="connect"
      >
        Rouvrir
      </button>
    </template>

    <div class="d-flex flex-column flex-fill console-term-body">
      <div
        v-if="status === 'error' && errorMessage"
        class="alert alert-danger m-3 mb-0"
      >
        {{ errorMessage }}
      </div>
      <div
        ref="containerEl"
        class="flex-fill console-term-container"
      />
    </div>
  </CommandLogPanel>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import CommandLogPanel from '../host/CommandLogPanel.vue'
import { useProxmoxConsole } from '../../composables/useProxmoxConsole'
import { useStatusBadge } from '../../composables/useStatusBadge'

const props = withDefaults(defineProps<{
  guestId: string
  guestName?: string
  show?: boolean
}>(), {
  guestName: '',
  show: false,
})

defineEmits<{
  (e: 'close'): void
  (e: 'open'): void
}>()

const containerEl = ref<HTMLElement | null>(null)
const { status, errorMessage, open, resize, close: closeSocket } = useProxmoxConsole()

const { getStatusBadgeClass } = useStatusBadge({
  map: {
    idle: 'badge bg-secondary-lt text-secondary',
    connecting: 'badge bg-warning-lt text-warning',
    connected: 'badge bg-success-lt text-success',
    disconnected: 'badge bg-secondary-lt text-secondary',
    error: 'badge bg-danger-lt text-danger',
  },
})
const statusBadgeClass = computed(() => getStatusBadgeClass(status.value))

const statusLabels: Record<string, string> = {
  idle: 'Inactif',
  connecting: 'Connexion…',
  connected: 'Connecté',
  disconnected: 'Déconnecté',
  error: 'Erreur',
}
const statusLabel = computed(() => statusLabels[status.value] ?? status.value)

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let resizeObserver: ResizeObserver | null = null

function connect(): void {
  if (!term) return
  open(props.guestId, term)
}

// Deferred to the next frame instead of running straight inside the
// ResizeObserver callback: fitAddon.fit() itself changes the terminal's
// internal canvas layout, which can trigger another observation in the same
// synchronous pass — the classic cause of the (otherwise harmless, but this
// app's global window 'error' handler treats ANY uncaught error as fatal —
// see main.ts) "ResizeObserver loop completed with undelivered
// notifications" error. Also skips entirely while the container has no
// layout (panel hidden via CommandLogPanel's v-show — see below), since
// fitting a zero-size box produces meaningless cols/rows.
function scheduleFit(): void {
  window.requestAnimationFrame(() => {
    if (!fitAddon || !term || !containerEl.value) return
    if (containerEl.value.offsetWidth === 0 || containerEl.value.offsetHeight === 0) return
    fitAddon.fit()
    resize(term.cols, term.rows)
  })
}

// Mounting xterm.js into a hidden (display:none) container yields a
// zero-size canvas, so the Terminal is only ever constructed once `show`
// actually flips true the first time and the container has real layout.
// CommandLogPanel hides with v-show afterwards (not v-if), so a later
// close/reopen just toggles visibility — the session stays connected in the
// background exactly like every other console panel in the app, instead of
// being torn down and losing the shell on every close.
async function mountTerminal(): Promise<void> {
  await nextTick()
  if (!containerEl.value || term) return

  term = new Terminal({
    convertEol: true,
    fontFamily: "'Consolas', 'Monaco', 'Courier New', monospace",
    fontSize: 13,
    cursorBlink: true,
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(containerEl.value)
  scheduleFit()

  connect()

  resizeObserver = new ResizeObserver(scheduleFit)
  resizeObserver.observe(containerEl.value)
}

function teardownTerminal(): void {
  closeSocket()
  resizeObserver?.disconnect()
  resizeObserver = null
  term?.dispose()
  term = null
  fitAddon = null
}

watch(() => props.show, (visible) => {
  if (visible && !term) void mountTerminal()
  else if (visible) scheduleFit() // re-fit: the container may have been resized while hidden
}, { immediate: true })

onBeforeUnmount(() => {
  teardownTerminal()
})
</script>

<style scoped>
.console-term-body {
  min-height: 0;
  height: 100%;
}

.console-term-container {
  min-height: 0;
  padding: 0.5rem;
  background: var(--ss-panel-solid-darker);
  border-radius: 0 0 0.5rem 0.5rem;
}

.console-term-container :deep(.xterm) {
  height: 100%;
}
</style>

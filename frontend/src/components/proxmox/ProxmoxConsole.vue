<template>
  <div v-if="show">
    <div
      ref="modalRef"
      class="modal modal-blur fade show"
      style="display: block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-dialog-centered modal-fullscreen">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title d-flex align-items-center gap-2 mb-0">
              <IconTerminal2
                :size="20"
                class="icon"
              />
              Console — {{ guestName || guestId }}
              <span :class="statusBadgeClass">{{ statusLabel }}</span>
            </h5>
            <div class="d-flex align-items-center gap-2">
              <button
                v-if="status === 'disconnected' || status === 'error'"
                type="button"
                class="btn btn-sm btn-outline-secondary"
                @click="connect"
              >
                Rouvrir
              </button>
              <button
                type="button"
                class="btn-close"
                aria-label="Fermer"
                @click="$emit('close')"
              />
            </div>
          </div>
          <div class="modal-body p-0 d-flex flex-column flex-fill console-term-body">
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
        </div>
      </div>
    </div>
    <div class="modal-backdrop fade show" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { IconTerminal2 } from '@tabler/icons-vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useProxmoxConsole } from '../../composables/useProxmoxConsole'
import { useModalChrome } from '../../composables/useModalChrome'
import { useStatusBadge } from '../../composables/useStatusBadge'

const props = withDefaults(defineProps<{
  guestId: string
  guestName?: string
  show?: boolean
}>(), {
  guestName: '',
  show: false,
})

const emit = defineEmits<{
  (e: 'close'): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => props.show, { onClose: () => emit('close') })

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

// Mounting xterm.js into a hidden (display:none via v-if) container yields a
// zero-size canvas, so the Terminal is only ever constructed once `show`
// actually flips true and the container has real layout.
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
  fitAddon.fit()

  connect()

  resizeObserver = new ResizeObserver(() => {
    if (!fitAddon || !term) return
    fitAddon.fit()
    resize(term.cols, term.rows)
  })
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
  if (visible) {
    void mountTerminal()
  } else {
    teardownTerminal()
  }
}, { immediate: true })

onBeforeUnmount(() => {
  teardownTerminal()
})
</script>

<style scoped>
.console-term-body {
  min-height: 0;
}

.console-term-container {
  min-height: 0;
  padding: 0.5rem;
  background: var(--ss-panel-solid-darker);
}

.console-term-container :deep(.xterm) {
  height: 100%;
}
</style>

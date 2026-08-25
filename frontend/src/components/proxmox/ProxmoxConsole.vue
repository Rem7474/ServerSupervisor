<template>
  <CommandLogPanel
    mode="custom"
    :show="show"
    :title="`Console — ${guestName || guestId}`"
    wrapper-class="side-panel side-panel-terminal"
    @close="handleClose"
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
        v-if="status !== 'connecting' && status !== 'connected'"
        type="button"
        class="btn btn-sm btn-ghost-secondary"
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

      <!-- mousedown.prevent on the bar (not click) keeps the focus — and so
           the soft keyboard — on xterm's hidden textarea: a button that
           takes focus dismisses the keyboard on every tap. -->
      <div
        class="console-touch-bar"
        @mousedown.prevent
      >
        <button
          type="button"
          :class="ctrlArmed ? 'btn btn-sm btn-primary' : 'btn btn-sm btn-ghost-secondary'"
          :disabled="!inputEnabled"
          title="Touche Ctrl — s'applique au caractère suivant"
          @click="toggleCtrl"
        >
          Ctrl
        </button>
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Interrompre (Ctrl+C)"
          @click="sendKey('\u0003')"
        >
          ^C
        </button>
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Échap"
          @click="sendKey('\u001b')"
        >
          Échap
        </button>
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Tabulation (complétion)"
          @click="sendKey('\t')"
        >
          Tab
        </button>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Haut (historique)"
          @click="sendArrow('\u001b[A')"
        >
          <IconArrowUp
            :size="14"
            class="icon"
          />
        </button>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Bas (historique)"
          @click="sendArrow('\u001b[B')"
        >
          <IconArrowDown
            :size="14"
            class="icon"
          />
        </button>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Gauche"
          @click="sendArrow('\u001b[D')"
        >
          <IconArrowLeft
            :size="14"
            class="icon"
          />
        </button>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :disabled="!inputEnabled"
          title="Droite"
          @click="sendArrow('\u001b[C')"
        >
          <IconArrowRight
            :size="14"
            class="icon"
          />
        </button>
      </div>
    </div>
  </CommandLogPanel>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { IconArrowDown, IconArrowLeft, IconArrowRight, IconArrowUp } from '@tabler/icons-vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import CommandLogPanel from '../host/CommandLogPanel.vue'
import { useProxmoxConsole } from '../../composables/useProxmoxConsole'
import { useStatusBadge } from '../../composables/useStatusBadge'
import { useModalChrome } from '../../composables/useModalChrome'

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
  (e: 'open'): void
}>()

const containerEl = ref<HTMLElement | null>(null)
const { status, errorMessage, open, sendInput, resize, close: closeSocket } = useProxmoxConsole()

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

// Below 991px (style.css's .side-panel-terminal breakpoint) the panel
// becomes a `position: fixed` full-viewport overlay instead of the desktop
// sticky sidebar — without a real body scroll lock, iOS Safari can still
// rubber-band/reveal the page behind it. Desktop's sidebar sits in the
// normal page flow, so it must NOT lock scroll. ESC/Tab are real terminal
// keys here (not modal-dismiss/focus-trap gestures), so only the lock is
// wired in — closeOnEsc/trapFocus stay off, and the DOM ref they'd need is
// therefore unused.
const mobileQuery = window.matchMedia('(max-width: 991px)')
const isMobileViewport = ref(mobileQuery.matches)
const onMobileQueryChange = (e: MediaQueryListEvent) => { isMobileViewport.value = e.matches }
mobileQuery.addEventListener('change', onMobileQueryChange)
onBeforeUnmount(() => mobileQuery.removeEventListener('change', onMobileQueryChange))
useModalChrome(ref(null), () => props.show && isMobileViewport.value, {
  closeOnEsc: false,
  trapFocus: false,
  lockScroll: true,
})

// Feeds style.css's --console-vh: some mobile browsers report a `100dvh`
// that doesn't track the actually-visible area as the address bar
// shows/hides (dvh browser support/behavior is inconsistent, unlike
// visualViewport which reflects the real visible viewport directly) — a
// stale/too-tall dvh pushes the touch bar at the bottom of the full-screen
// console past the visible fold. window.visualViewport is undefined only on
// very old browsers, where the CSS var stays unset and style.css's
// `var(--console-vh, 100dvh)` falls back to the previous dvh-only behavior.
function updateConsoleVh(): void {
  const h = window.visualViewport?.height ?? window.innerHeight
  document.documentElement.style.setProperty('--console-vh', `${h}px`)
}
updateConsoleVh()
window.visualViewport?.addEventListener('resize', updateConsoleVh)
window.addEventListener('resize', updateConsoleVh)
onBeforeUnmount(() => {
  window.visualViewport?.removeEventListener('resize', updateConsoleVh)
  window.removeEventListener('resize', updateConsoleVh)
})

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let resizeObserver: ResizeObserver | null = null

// A phone/tablet soft keyboard has no Ctrl, Esc, Tab or arrow keys, which
// makes a terminal close to useless there — no way to interrupt a running
// command, complete a path, or recall the previous one. The touch bar sends
// those bytes itself; "Ctrl" is a sticky modifier applied to the next
// character the user types (same interaction as Termux/PVE's own mobile
// console) rather than a chord nothing on the device can produce.
const ctrlArmed = ref(false)
const inputEnabled = computed(() => status.value === 'connected')

// Ctrl+<key> is just the ASCII control code: @ A-Z [ \ ] ^ _ (0x40-0x5F)
// map to 0x00-0x1F, and Ctrl+? is DEL. Anything else (a digit, an accented
// letter, a multi-char paste) has no control form — send it unchanged
// rather than mangling it.
function toControlChar(data: string): string {
  if (data.length !== 1) return data
  const code = data.toUpperCase().charCodeAt(0)
  if (code >= 0x40 && code <= 0x5f) return String.fromCharCode(code - 0x40)
  if (code === 0x3f) return '\u007f'
  return data
}

function transformInput(data: string): string {
  if (!ctrlArmed.value) return data
  ctrlArmed.value = false
  return toControlChar(data)
}

function toggleCtrl(): void {
  ctrlArmed.value = !ctrlArmed.value
  term?.focus()
}

// Refocus after every touch key so the soft keyboard stays up and the next
// keystroke still lands in the terminal. Only for keys the user is meant to
// keep typing after (Ctrl/^C/Esc/Tab) — an arrow tap is a complete, one-shot
// action with nothing to follow it, so it must NOT call this: on a phone
// with the keyboard not already open (e.g. browsing history via the arrows
// right after opening the console), term.focus() pops the soft keyboard up
// over the touch bar itself, burying the very buttons just tapped.
function sendKey(sequence: string): void {
  sendInput(sequence)
  ctrlArmed.value = false
  term?.focus()
}

function sendArrow(sequence: string): void {
  sendInput(sequence)
  ctrlArmed.value = false
}

function connect(): void {
  if (!term) return
  ctrlArmed.value = false
  open(props.guestId, term, { transformInput })
}

// The close (X) button ends the remote shell — a live PTY session left
// running server-side after the user thinks they've closed it would be
// surprising (unlike CommandLogPanel's other uses, where "close" only ever
// hides a static log). Reopening (the "Rouvrir" button, or the parent
// re-showing the panel) starts a fresh session via connect().
function handleClose(): void {
  closeSocket()
  emit('close')
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
// The xterm.js instance itself (scrollback, etc.) is kept alive across a
// close/reopen — CommandLogPanel hides with v-show, not v-if — but the PVE
// session is not (see handleClose): reopening shows the same screen buffer
// with a fresh shell connected underneath, same as PVE's own web console.
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
  if (!visible) return
  if (!term) {
    void mountTerminal()
    return
  }
  scheduleFit() // re-fit: the container may have been resized while hidden
  // Reopening (e.g. via the guest page's "Console" button) after an explicit
  // close reconnects automatically — one click, not "open panel" then
  // "Rouvrir" as two separate steps.
  if (status.value !== 'connecting' && status.value !== 'connected') connect()
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

.console-touch-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  padding: 0.375rem 0.5rem;
  background: var(--ss-panel-solid);
  border-top: 1px solid var(--tblr-border-color);
}

/* Only worth the vertical space where the keys are actually missing: a
   desktop pointer means a physical keyboard with real Ctrl/Esc/Tab/arrows.
   Width alone isn't the test — a tablet in landscape is wide and still has
   no Ctrl key. */
@media (min-width: 992px) and (pointer: fine) {
  .console-touch-bar {
    display: none;
  }
}
</style>

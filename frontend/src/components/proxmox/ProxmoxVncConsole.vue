<template>
  <div
    v-if="show"
    ref="modalRef"
    class="modal modal-blur show d-block vnc-modal"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="titleId"
  >
    <div class="modal-dialog modal-fullscreen-lg-down vnc-dialog">
      <div
        ref="contentEl"
        class="modal-content vnc-content"
      >
        <div class="modal-header gap-2 flex-wrap">
          <h3
            :id="titleId"
            class="modal-title d-flex align-items-center gap-2 me-auto"
          >
            <IconDeviceDesktop
              :size="18"
              class="icon"
            />
            {{ t('proxmox.vncTitle', { name: guestName || guestId }) }}
            <span :class="statusBadgeClass">{{ statusLabel }}</span>
          </h3>
          <div class="d-flex gap-2 flex-wrap">
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              :disabled="status !== 'connected'"
              :title="t('proxmox.vncCtrlAltDelTooltip')"
              @click="handleCtrlAltDel"
            >
              <IconKeyboard
                :size="16"
                class="icon me-1"
              />
              {{ t('proxmox.vncCtrlAltDel') }}
            </button>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              :aria-pressed="isFullscreen"
              @click="toggleFullscreen"
            >
              <IconMinimize
                v-if="isFullscreen"
                :size="16"
                class="icon me-1"
              />
              <IconMaximize
                v-else
                :size="16"
                class="icon me-1"
              />
              {{ isFullscreen ? t('proxmox.vncExitFullscreen') : t('proxmox.vncFullscreen') }}
            </button>
            <button
              v-if="status === 'disconnected' || status === 'error'"
              type="button"
              class="btn btn-sm btn-primary"
              @click="connect"
            >
              <IconRefresh
                :size="16"
                class="icon me-1"
              />
              {{ t('proxmox.vncReconnect') }}
            </button>
            <button
              type="button"
              class="btn-close ms-1"
              :aria-label="t('common.close')"
              @click="handleClose"
            />
          </div>
        </div>
        <div class="modal-body p-0 vnc-body">
          <div
            ref="screenEl"
            class="vnc-screen"
            role="application"
            :aria-label="t('proxmox.vncScreenLabel')"
          />
          <div
            v-if="status !== 'connected'"
            class="vnc-overlay"
            aria-live="polite"
          >
            <div
              v-if="status === 'connecting'"
              class="text-center"
            >
              <span class="spinner-border spinner-border-sm me-2" />
              {{ t('proxmox.vncConnecting') }}
            </div>
            <div
              v-else-if="status === 'error'"
              class="alert alert-danger mb-0 vnc-alert"
              role="alert"
            >
              {{ errorMessage || t('proxmox.vncConnectionFailed') }}
            </div>
            <div
              v-else-if="status === 'disconnected'"
              class="text-center"
            >
              {{ t('proxmox.vncDisconnected') }}
            </div>
          </div>
        </div>
        <div class="modal-footer py-2 text-secondary small justify-content-start">
          {{ t('proxmox.vncKeyboardHint') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconDeviceDesktop, IconKeyboard, IconMaximize, IconMinimize, IconRefresh } from '@tabler/icons-vue'
import { useProxmoxVnc } from '../../composables/useProxmoxVnc'
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
}>()

const { t } = useI18n()
const titleId = useId()

const modalRef = ref<HTMLElement | null>(null)
const contentEl = ref<HTMLElement | null>(null)
const screenEl = ref<HTMLElement | null>(null)
const { status, errorMessage, open, sendCtrlAltDel, focus, close: closeSession } = useProxmoxVnc()

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

const statusLabel = computed(() => {
  switch (status.value) {
    case 'connecting': return t('proxmox.consoleStatusConnecting')
    case 'connected': return t('proxmox.consoleStatusConnected')
    case 'disconnected': return t('proxmox.consoleStatusDisconnected')
    case 'error': return t('common.error')
    default: return t('proxmox.consoleStatusIdle')
  }
})

// Escape and Tab are keys for the VM (closing a dialog or leaving a login
// prompt on Escape would be surprising), so only the scroll lock is wired.
// Armed once mounted: the chrome must not run against a ref the first render
// has not filled yet.
const mounted = ref(false)
onMounted(() => { mounted.value = true })
useModalChrome(modalRef, () => mounted.value && props.show, {
  closeOnEsc: false,
  trapFocus: false,
  lockScroll: true,
})

async function connect(): Promise<void> {
  await nextTick()
  if (!screenEl.value) return
  await open(props.guestId, screenEl.value)
}

function handleCtrlAltDel(): void {
  sendCtrlAltDel()
  focus()
}

// The close button ends the VNC session: the server-side proxy and the PVE
// vncproxy worker only live as long as the WebSocket.
function handleClose(): void {
  if (document.fullscreenElement) void document.exitFullscreen().catch(() => {})
  closeSession()
  emit('close')
}

const isFullscreen = ref(false)
function onFullscreenChange(): void {
  isFullscreen.value = !!document.fullscreenElement && document.fullscreenElement === contentEl.value
}

async function toggleFullscreen(): Promise<void> {
  try {
    if (document.fullscreenElement) await document.exitFullscreen()
    else await contentEl.value?.requestFullscreen()
  } catch {
    // Fullscreen refused (iframe without allowfullscreen, user gesture
    // rules): the dialog simply stays as it is.
  }
  focus()
}

document.addEventListener('fullscreenchange', onFullscreenChange)
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  closeSession()
})

watch(() => props.show, (visible) => {
  if (visible) void connect()
  else closeSession()
}, { immediate: true })
</script>

<style scoped>
.vnc-modal {
  background: var(--ss-panel-medium);
}

.vnc-dialog {
  max-width: min(1280px, 96vw);
}

.vnc-content {
  height: min(90vh, 900px);
}

.vnc-content:fullscreen {
  height: 100%;
  border-radius: 0;
}

.vnc-body {
  position: relative;
  min-height: 0;
  flex: 1 1 auto;
  background: var(--ss-panel-solid-darker);
}

.vnc-screen {
  position: absolute;
  inset: 0;
}

.vnc-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  color: var(--tblr-light);
}

.vnc-alert {
  max-width: 36rem;
}

@media (max-width: 991.98px) {
  .vnc-content {
    height: 100%;
  }
}
</style>

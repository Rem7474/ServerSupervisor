<template>
  <Teleport to="body">
    <div
      class="toast-container position-fixed bottom-0 end-0 p-3"
      aria-live="polite"
      aria-label="Notifications"
    >
      <TransitionGroup name="ss-toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          class="toast show ss-toast"
          :class="`ss-toast--${toast.type}`"
          role="alert"
          aria-atomic="true"
        >
          <div class="toast-body d-flex align-items-center gap-2">
            <IconCircleCheck
              v-if="toast.type === 'success'"
              :size="18"
              class="ss-toast-icon flex-shrink-0"
            />
            <IconCircleX
              v-else-if="toast.type === 'error'"
              :size="18"
              class="ss-toast-icon flex-shrink-0"
            />
            <IconAlertTriangle
              v-else-if="toast.type === 'warning'"
              :size="18"
              class="ss-toast-icon flex-shrink-0"
            />
            <IconInfoCircle
              v-else
              :size="18"
              class="ss-toast-icon flex-shrink-0"
            />
            <span class="flex-fill ss-toast-message">{{ toast.message }}</span>
            <button
              type="button"
              class="ss-toast-close flex-shrink-0"
              aria-label="Fermer"
              @click="removeToast(toast.id)"
            >
              <IconX
                :size="14"
                :stroke-width="2.5"
              />
            </button>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useGlobalToast } from '../composables/useGlobalToast'
import { IconCircleCheck, IconCircleX, IconAlertTriangle, IconInfoCircle, IconX } from '@tabler/icons-vue'

const { toasts, removeToast } = useGlobalToast()
</script>

<style scoped>
/*
 * .toast/.toast-container already supply background/border/border-radius/
 * box-shadow/width/font-size/z-index (see the app-wide `.toast { z-index:
 * var(--z-index-toast) }` rule in style.css) via Tabler's tokens. Only the
 * per-type accent (a colored left border + matching icon, which Tabler's
 * .toast has no variant system for) and pointer-events (so empty gaps in the
 * fixed container don't block clicks on the page beneath) stay custom here.
 */
.toast-container {
  pointer-events: none;
}

.ss-toast {
  pointer-events: auto;
  border-left: 3px solid transparent;
}

.ss-toast--success {
  border-left-color: var(--tblr-success);
}
.ss-toast--error {
  border-left-color: var(--tblr-danger);
}
.ss-toast--warning {
  border-left-color: var(--tblr-warning);
}
.ss-toast--info {
  border-left-color: var(--tblr-info);
}

.ss-toast--success .ss-toast-icon {
  color: var(--tblr-success);
}
.ss-toast--error .ss-toast-icon {
  color: var(--tblr-danger);
}
.ss-toast--warning .ss-toast-icon {
  color: var(--tblr-warning);
}
.ss-toast--info .ss-toast-icon {
  color: var(--tblr-info);
}

.ss-toast-message {
  word-break: break-word;
}

.ss-toast-close {
  background: none;
  border: none;
  padding: 0.1rem;
  cursor: pointer;
  color: var(--tblr-muted);
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.ss-toast-close:hover {
  color: var(--tblr-body-color);
}

/* Transitions */
.ss-toast-enter-active {
  transition: all 0.25s ease;
}
.ss-toast-leave-active {
  transition: all 0.2s ease;
}
.ss-toast-enter-from {
  opacity: 0;
  transform: translateX(1.5rem);
}
.ss-toast-leave-to {
  opacity: 0;
  transform: translateX(1.5rem);
}
.ss-toast-move {
  transition: transform 0.2s ease;
}
</style>

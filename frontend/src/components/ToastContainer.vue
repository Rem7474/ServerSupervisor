<template>
  <Teleport to="body">
    <div
      class="ss-toast-container"
      aria-live="polite"
      aria-label="Notifications"
    >
      <TransitionGroup name="ss-toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          class="ss-toast"
          :class="`ss-toast--${toast.type}`"
          role="alert"
        >
          <IconCircleCheck
            v-if="toast.type === 'success'"
            :size="18"
            class="ss-toast-icon"
          />
          <IconCircleX
            v-else-if="toast.type === 'error'"
            :size="18"
            class="ss-toast-icon"
          />
          <IconAlertTriangle
            v-else-if="toast.type === 'warning'"
            :size="18"
            class="ss-toast-icon"
          />
          <IconInfoCircle
            v-else
            :size="18"
            class="ss-toast-icon"
          />
          <span class="ss-toast-message">{{ toast.message }}</span>
          <button
            type="button"
            class="ss-toast-close"
            aria-label="Fermer"
            @click="removeToast(toast.id)"
          >
            <IconX
              :size="14"
              :stroke-width="2.5"
            />
          </button>
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
.ss-toast-container {
  position: fixed;
  bottom: 1.5rem;
  right: 1.5rem;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-width: 24rem;
  pointer-events: none;
}

.ss-toast {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.65rem 0.85rem;
  border-radius: 0.4rem;
  border: 1px solid;
  background: var(--tblr-bg-surface);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.35);
  pointer-events: all;
  min-width: 16rem;
}

.ss-toast--success {
  border-color: var(--tblr-success);
  color: var(--tblr-success);
}
.ss-toast--error {
  border-color: var(--tblr-danger);
  color: var(--tblr-danger);
}
.ss-toast--warning {
  border-color: var(--tblr-warning);
  color: var(--tblr-warning);
}
.ss-toast--info {
  border-color: var(--tblr-info);
  color: var(--tblr-info);
}

.ss-toast-icon {
  flex-shrink: 0;
}

.ss-toast-message {
  flex: 1;
  font-size: 0.875rem;
  color: var(--tblr-body-color);
  word-break: break-word;
}

.ss-toast-close {
  flex-shrink: 0;
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

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
          <svg
            class="ss-toast-icon"
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <!-- success -->
            <template v-if="toast.type === 'success'">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </template>
            <!-- error -->
            <template v-else-if="toast.type === 'error'">
              <circle
                cx="12"
                cy="12"
                r="10"
              />
              <line
                x1="15"
                y1="9"
                x2="9"
                y2="15"
              />
              <line
                x1="9"
                y1="9"
                x2="15"
                y2="15"
              />
            </template>
            <!-- warning -->
            <template v-else-if="toast.type === 'warning'">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
              <line
                x1="12"
                y1="9"
                x2="12"
                y2="13"
              />
              <line
                x1="12"
                y1="17"
                x2="12.01"
                y2="17"
              />
            </template>
            <!-- info -->
            <template v-else>
              <circle
                cx="12"
                cy="12"
                r="10"
              />
              <line
                x1="12"
                y1="8"
                x2="12"
                y2="12"
              />
              <line
                x1="12"
                y1="16"
                x2="12.01"
                y2="16"
              />
            </template>
          </svg>
          <span class="ss-toast-message">{{ toast.message }}</span>
          <button
            type="button"
            class="ss-toast-close"
            aria-label="Fermer"
            @click="removeToast(toast.id)"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
            >
              <line
                x1="18"
                y1="6"
                x2="6"
                y2="18"
              />
              <line
                x1="6"
                y1="6"
                x2="18"
                y2="18"
              />
            </svg>
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useGlobalToast } from '../composables/useGlobalToast'

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

<template>
  <Teleport to="body">
    <Transition name="bulk-bar">
      <div
        v-if="count > 0"
        class="bulk-action-bar"
        role="toolbar"
        :aria-label="`Actions groupées — ${count} hôte(s) sélectionné(s)`"
      >
        <span class="bulk-action-bar__count">
          <IconCheck
            :size="16"
            :stroke-width="2.5"
          />
          {{ count }} hôte{{ count > 1 ? 's' : '' }} sélectionné{{ count > 1 ? 's' : '' }}
        </span>

        <div class="bulk-action-bar__actions">
          <slot />
        </div>

        <button
          class="bulk-action-bar__close"
          type="button"
          aria-label="Annuler la sélection"
          @click="$emit('clear')"
        >
          <IconX
            :size="16"
            :stroke-width="2.5"
          />
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { IconCheck, IconX } from '@tabler/icons-vue'

defineProps<{ count: number }>()
defineEmits<{ clear: [] }>()
</script>

<style scoped>
.bulk-action-bar {
  position: fixed;
  bottom: 1.5rem;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1040;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 1rem;
  background: var(--tblr-bg-surface);
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.5rem;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  max-width: calc(100vw - 2rem);
  flex-wrap: wrap;
}

.bulk-action-bar__count {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--tblr-body-color);
  white-space: nowrap;
}

.bulk-action-bar__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.bulk-action-bar__close {
  background: none;
  border: none;
  padding: 0.2rem;
  cursor: pointer;
  color: var(--tblr-muted);
  display: flex;
  align-items: center;
  margin-left: auto;
}
.bulk-action-bar__close:hover {
  color: var(--tblr-body-color);
}

/* Transitions */
.bulk-bar-enter-active {
  transition: all 0.22s cubic-bezier(0.16, 1, 0.3, 1);
}
.bulk-bar-leave-active {
  transition: all 0.15s ease;
}
.bulk-bar-enter-from,
.bulk-bar-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(1rem);
}
</style>

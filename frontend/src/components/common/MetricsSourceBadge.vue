<template>
  <span
    :class="badgeClass"
    :aria-label="ariaLabel"
    :title="badgeText"
  >
    {{ badgeText }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  source: {
    type: String,
    required: true,
    validator: (value) => ['proxmox', 'agent', 'auto'].includes(value),
  },
})

const badgeText = computed(() => {
  switch (props.source) {
    case 'proxmox':
      return 'proxmox'
    case 'agent':
      return 'Source : Agent'
    case 'auto':
      return 'Source : Automatique'
    default:
      return ''
  }
})

const badgeClass = computed(() => {
  switch (props.source) {
    case 'proxmox':
      return 'badge bg-orange-lt text-orange'
    case 'agent':
      return 'badge bg-cyan-lt text-cyan'
    case 'auto':
      return 'badge bg-blue-lt text-blue'
    default:
      return 'badge bg-secondary-lt text-secondary'
  }
})

const ariaLabel = computed(() => badgeText.value)
</script>

<style scoped>
/* Ensure badge displays inline when needed */
:deep(.badge) {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}
</style>

<template>
  <BadgePill
    :tone="badgeTone"
    :text="badgeText"
    :aria-label="ariaLabel"
    :title="badgeText"
    compact
  />
</template>

<script setup>
import { computed } from 'vue'
import BadgePill from './BadgePill.vue'

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

const badgeTone = computed(() => {
  switch (props.source) {
    case 'proxmox':
      return 'orange'
    case 'agent':
      return 'cyan'
    case 'auto':
      return 'blue'
    default:
      return 'secondary'
  }
})

const ariaLabel = computed(() => badgeText.value)
</script>

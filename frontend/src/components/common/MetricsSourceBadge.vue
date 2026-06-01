<template>
  <BadgePill
    :tone="badgeTone"
    :text="badgeText"
    :aria-label="ariaLabel"
    :title="badgeText"
    compact
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BadgePill from './BadgePill.vue'

type Source = 'proxmox' | 'agent' | 'auto'

const props = defineProps<{
  source: Source
}>()

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

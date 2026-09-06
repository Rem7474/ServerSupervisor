<template>
  <span>{{ relativeTime }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRelativeTime } from '../composables/useRelativeTime'

const props = defineProps({
  date: {
    type: [String, Date],
    required: true
  }
})

// Wrap in a computed so the date stays reactive
const dateGetter = computed(() => props.date)
const relativeTime = useRelativeTime(dateGetter)
</script>

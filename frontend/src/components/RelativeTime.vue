<template>
  <span>{{ relativeTime }}</span>
</template>

<script setup>
import { computed } from 'vue'
import { useRelativeTime } from '../composables/useRelativeTime'

const props = defineProps({
  date: {
    type: [String, Date],
    required: true
  }
})

// Créer une fonction computed pour que la date soit réactive
const dateGetter = computed(() => props.date)
const relativeTime = useRelativeTime(dateGetter)
</script>

<template>
  <button
    type="button"
    class="btn btn-link btn-sm p-0 text-decoration-none ui-sortable-header"
    @click="$emit('toggle')"
  >
    {{ label }} {{ indicator }}
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  active: { type: Boolean, default: false },
  direction: { type: String, default: 'asc' },
})

defineEmits(['toggle'])

const indicator = computed(() => {
  if (!props.active) return '↕'
  return props.direction === 'asc' ? '↑' : '↓'
})
</script>

<style scoped>
.ui-sortable-header {
  color: inherit;
  font-weight: 600;
}

.ui-sortable-header:hover {
  color: var(--tblr-primary);
}
</style>

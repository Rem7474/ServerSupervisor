<template>
  <ul class="nav nav-tabs mb-4">
    <li
      v-for="tab in tabs"
      :key="tab.key"
      class="nav-item"
    >
      <button
        class="nav-link"
        :class="{ active: modelValue === tab.key }"
        @click="$emit('update:modelValue', tab.key)"
      >
        {{ tab.label }}
        <span
          v-if="tab.badge"
          class="badge bg-azure-lt text-azure ms-1"
        >{{ tab.badge }}</span>
      </button>
    </li>
  </ul>
</template>

<script setup>
defineProps({
  modelValue: String,
  tabs: {
    type: Array,
    required: true
    // [{ key: String, label: String, badge?: Number|String }]
  }
})

defineEmits(['update:modelValue'])
</script>

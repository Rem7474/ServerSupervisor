<template>
  <ul
    class="nav nav-tabs mb-4"
    role="tablist"
    @keydown="handleKeyDown"
  >
    <li
      v-for="tab in tabs"
      :key="tab.key"
      class="nav-item"
    >
      <button
        role="tab"
        class="nav-link"
        :class="{ active: modelValue === tab.key }"
        :aria-selected="modelValue === tab.key"
        :tabindex="modelValue === tab.key ? 0 : -1"
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
  modelValue: {
    type: String,
    required: true,
  },
  tabs: {
    type: Array,
    required: true
    // [{ key: String, label: String, badge?: Number|String }]
  }
})

const emit = defineEmits(['update:modelValue'])

const handleKeyDown = (e) => {
  if (!tabs || tabs.length === 0) return
  
  const currentIndex = tabs.findIndex(t => t.key === modelValue)
  if (currentIndex === -1) return
  
  let nextIndex
  
  if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
    e.preventDefault()
    nextIndex = (currentIndex + 1) % tabs.length
    emit('update:modelValue', tabs[nextIndex].key)
  } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
    e.preventDefault()
    nextIndex = (currentIndex - 1 + tabs.length) % tabs.length
    emit('update:modelValue', tabs[nextIndex].key)
  } else if (e.key === 'Home') {
    e.preventDefault()
    emit('update:modelValue', tabs[0].key)
  } else if (e.key === 'End') {
    e.preventDefault()
    emit('update:modelValue', tabs[tabs.length - 1].key)
  }
}
</script>

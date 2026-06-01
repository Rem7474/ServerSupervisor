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

<script setup lang="ts">
interface Tab {
  key: string
  label: string
  badge?: number | string
}

const props = defineProps<{
  modelValue: string
  tabs: Tab[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', key: string): void
}>()

const handleKeyDown = (e: KeyboardEvent): void => {
  if (!props.tabs || props.tabs.length === 0) return

  const currentIndex = props.tabs.findIndex((t) => t.key === props.modelValue)
  if (currentIndex === -1) return

  let nextIndex: number

  if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
    e.preventDefault()
    nextIndex = (currentIndex + 1) % props.tabs.length
    emit('update:modelValue', props.tabs[nextIndex].key)
  } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
    e.preventDefault()
    nextIndex = (currentIndex - 1 + props.tabs.length) % props.tabs.length
    emit('update:modelValue', props.tabs[nextIndex].key)
  } else if (e.key === 'Home') {
    e.preventDefault()
    emit('update:modelValue', props.tabs[0].key)
  } else if (e.key === 'End') {
    e.preventDefault()
    emit('update:modelValue', props.tabs[props.tabs.length - 1].key)
  }
}
</script>

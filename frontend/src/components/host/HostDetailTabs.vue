<template>
  <ul class="nav nav-tabs mb-3">
    <li
      v-for="tab in visibleTabs"
      :key="tab.key"
      class="nav-item"
    >
      <a
        class="nav-link"
        :class="{ active: modelValue === tab.key }"
        href="#"
        @click.prevent="$emit('update:modelValue', tab.key)"
      >
        {{ tab.label }}
        <span
          v-if="tab.badge"
          :class="tab.badgeClass"
        >{{ tab.badge }}</span>
      </a>
    </li>
  </ul>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: String,
    required: true,
  },
  canRunApt: {
    type: Boolean,
    default: false,
  },
  containersCount: {
    type: Number,
    default: 0,
  },
  pendingPackages: {
    type: Number,
    default: 0,
  },
  commandsCount: {
    type: Number,
    default: 0,
  },
  tasksCount: {
    type: Number,
    default: 0,
  },
})

defineEmits(['update:modelValue'])

const visibleTabs = computed(() => {
  const tabs = [
    { key: 'metrics', label: 'Metriques' },
    {
      key: 'docker',
      label: 'Docker',
      badge: props.containersCount || null,
      badgeClass: 'badge bg-blue-lt text-blue ms-1',
    },
    {
      key: 'apt',
      label: 'APT',
      badge: props.pendingPackages > 0 ? props.pendingPackages : null,
      badgeClass: 'badge bg-yellow-lt text-yellow ms-1',
    },
    {
      key: 'commandes',
      label: 'Commandes',
      badge: props.commandsCount || null,
      badgeClass: 'badge bg-secondary-lt text-secondary ms-1',
    },
  ]

  if (props.canRunApt) {
    tabs.push(
      { key: 'systeme', label: 'Systeme' },
      { key: 'processus', label: 'Processus' }
    )
  }

  tabs.push({
    key: 'planifiees',
    label: 'Tâches planifiées',
    badge: props.tasksCount || null,
    badgeClass: 'badge bg-secondary-lt text-secondary ms-1',
  })

  return tabs
})
</script>


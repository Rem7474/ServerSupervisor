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

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string
  canRunApt?: boolean
  containersCount?: number
  pendingPackages?: number
  commandsCount?: number
  tasksCount?: number
  securityUpdates?: number
}>(), {
  canRunApt: false,
  containersCount: 0,
  pendingPackages: 0,
  commandsCount: 0,
  tasksCount: 0,
  securityUpdates: 0,
})

interface Tab {
  key: string
  label: string
  badge?: number | null
  badgeClass?: string
}

defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const visibleTabs = computed<Tab[]>(() => {
  const tabs: Tab[] = [
    { key: 'metrics', label: 'Métriques' },
    {
      key: 'docker',
      label: 'Docker',
      badge: props.containersCount || null,
      badgeClass: 'badge bg-blue-lt text-blue ms-1',
    },
    {
      key: 'apt',
      label: 'APT',
      badge: props.securityUpdates > 0 ? props.securityUpdates : (props.pendingPackages > 0 ? props.pendingPackages : null),
      badgeClass: props.securityUpdates > 0 ? 'badge bg-red-lt text-red ms-1' : 'badge bg-yellow-lt text-yellow ms-1',
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
    key: 'securite',
    label: 'Sécurité',
  })

  tabs.push({
    key: 'planifiees',
    label: 'Tâches planifiées',
    badge: props.tasksCount || null,
    badgeClass: 'badge bg-secondary-lt text-secondary ms-1',
  })

  return tabs
})
</script>


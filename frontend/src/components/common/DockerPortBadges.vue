<template>
  <div class="d-flex flex-wrap align-items-center gap-1 port-badges">
    <template v-if="visiblePorts.length">
      <span
        v-for="entry in visiblePorts"
        :key="entry.key"
        class="d-inline-flex align-items-center flex-wrap gap-1 port-badge-group"
      >
        <span :class="badgeClass">{{ formatLabel(entry.mapping) }}</span>
      </span>
      <span
        v-if="showGlobalIPv6Badge"
        class="badge bg-secondary-lt text-secondary port-badge-global-v6"
      >IPv6</span>
    </template>
    <span
      v-else
      class="text-muted"
    >—</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  formatExposedPort,
  formatInternalPort,
  groupGlobalDockerPorts,
  type DockerPortBadgeGroup,
  type DockerPortMapping,
} from '../../utils/dockerPorts'

const props = withDefaults(defineProps<{
  ports?: DockerPortMapping[]
  kind: 'internal' | 'exposed'
}>(), {
  ports: () => [],
})

const visiblePorts = computed<DockerPortBadgeGroup[]>(() => {
  const grouped = groupGlobalDockerPorts(props.ports)

  if (props.kind === 'internal') {
    return grouped.filter((entry) => Boolean(entry.mapping.internalPort))
  }

  return grouped.filter((entry) => Boolean(entry.mapping.hostPort))
})

const badgeClass = computed(() => {
  return props.kind === 'internal'
    ? 'badge bg-azure-lt text-azure'
    : 'badge bg-green-lt text-green'
})

const showGlobalIPv6Badge = computed(() => {
  if (props.kind !== 'exposed') {
    return false
  }

  return visiblePorts.value.some((entry) => entry.hasGlobalIPv6)
})

function formatLabel(mapping: DockerPortMapping): string {
  return props.kind === 'internal'
    ? formatInternalPort(mapping)
    : formatExposedPort(mapping)
}
</script>

<style scoped>
.port-badges .badge {
  white-space: nowrap;
}

.port-badge-global-v6 {
  font-size: 0.625rem;
  line-height: 1.1;
  padding: 0.15rem 0.35rem;
}

@media (max-width: 576px) {
  .port-badges {
    gap: 0.25rem !important;
  }

  .port-badge-group {
    gap: 0.25rem !important;
  }

  .port-badges .badge {
    font-size: 0.675rem;
    padding: 0.2rem 0.35rem;
  }
}
</style>

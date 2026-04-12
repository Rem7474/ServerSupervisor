<template>
  <div class="d-flex flex-wrap align-items-center gap-1 port-badges">
    <template v-if="visiblePorts.length">
      <span
        v-for="mapping in visiblePorts"
        :key="portMappingKey(mapping)"
        :class="badgeClass"
      >{{ formatLabel(mapping) }}</span>
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
  portMappingKey,
  type DockerPortMapping,
} from '../../utils/dockerPorts'

const props = withDefaults(defineProps<{
  ports?: DockerPortMapping[]
  kind: 'internal' | 'exposed'
}>(), {
  ports: () => [],
})

const visiblePorts = computed(() => {
  if (props.kind === 'internal') {
    return props.ports.filter((mapping) => Boolean(mapping.internalPort))
  }

  return props.ports.filter((mapping) => Boolean(mapping.hostPort))
})

const badgeClass = computed(() => {
  return props.kind === 'internal'
    ? 'badge bg-azure-lt text-azure'
    : 'badge bg-green-lt text-green'
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

@media (max-width: 576px) {
  .port-badges {
    gap: 0.25rem !important;
  }

  .port-badges .badge {
    font-size: 0.675rem;
    padding: 0.2rem 0.35rem;
  }
}
</style>

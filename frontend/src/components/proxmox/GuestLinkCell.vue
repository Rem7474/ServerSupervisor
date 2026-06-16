<template>
  <span
    v-if="!link"
    class="text-muted small"
  >—</span>
  <div
    v-else-if="link.status === 'suggested'"
    class="d-flex align-items-center gap-1"
  >
    <span class="badge bg-warning-lt text-warning">Suggéré</span>
    <span class="text-muted small">{{ link.host_hostname || link.host_name }}</span>
    <button
      type="button"
      class="btn btn-xs btn-success ms-1"
      @click="emit('confirm')"
    >
      ✓
    </button>
    <button
      type="button"
      class="btn btn-xs btn-outline-secondary"
      @click="emit('ignore')"
    >
      ✗
    </button>
  </div>
  <div
    v-else-if="link.status === 'confirmed'"
    class="d-flex align-items-center gap-1"
  >
    <span class="badge bg-success-lt text-success">Lié</span>
    <button
      type="button"
      class="btn btn-xs btn-outline-primary ms-1"
      title="Voir la fiche hôte"
      @click="emit('go')"
    >
      {{ link.host_hostname || link.host_name }}
    </button>
  </div>
  <span
    v-else
    class="text-muted small"
  >—</span>
</template>

<script setup lang="ts">
interface GuestLink {
  status?: string
  host_hostname?: string
  host_name?: string
}

defineProps<{ link?: GuestLink | null }>()

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'ignore'): void
  (e: 'go'): void
}>()
</script>

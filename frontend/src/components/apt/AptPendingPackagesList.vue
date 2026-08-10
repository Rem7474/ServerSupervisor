<template>
  <div
    v-if="packages.length > 0"
    class="mb-3"
  >
    <div class="d-flex align-items-center justify-content-between mb-2">
      <span class="small fw-semibold text-secondary">
        Paquets en attente
        <span class="badge bg-warning-lt text-warning ms-1">
          {{ packages.length }}
        </span>
      </span>
      <button
        v-if="packages.length > previewCount"
        type="button"
        class="btn btn-link btn-sm p-0 small text-secondary"
        @click="showAll = !showAll"
      >
        {{ showAll ? 'Réduire' : `Voir tout (${packages.length})` }}
      </button>
    </div>
    <div class="apt-packages-grid">
      <div
        v-for="pkg in visiblePackages"
        :key="pkg"
      >
        <code
          class="small text-body apt-package-item"
          :title="pkg"
        >{{ pkg }}</code>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = withDefaults(defineProps<{
  packages: string[]
  previewCount?: number
}>(), {
  previewCount: 15,
})

const showAll = ref(false)

const visiblePackages = computed(() =>
  showAll.value ? props.packages : props.packages.slice(0, props.previewCount),
)
</script>

<style scoped>
.apt-packages-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.35rem 0.75rem;
}
.apt-package-item {
  display: block;
}
@media (min-width: 768px) {
  .apt-packages-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (min-width: 1200px) {
  .apt-packages-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>

<template>
  <div
    v-if="variant === 'card'"
    class="loading-skeleton loading-skeleton-card"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="loading-skeleton-line"
    />
  </div>

  <div
    v-else-if="variant === 'table'"
    class="loading-skeleton loading-skeleton-table"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="loading-skeleton-row"
    />
  </div>

  <div
    v-else-if="variant === 'badge-list'"
    class="loading-skeleton loading-skeleton-inline d-flex flex-wrap gap-2"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="loading-skeleton-badge"
    />
  </div>

  <div
    v-else-if="variant === 'button-group'"
    class="loading-skeleton loading-skeleton-inline d-flex gap-2"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="loading-skeleton-button"
    />
  </div>

  <div
    v-else-if="variant === 'list'"
    class="loading-skeleton loading-skeleton-list"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="loading-skeleton-list-item"
    />
  </div>
</template>

<script setup lang="ts">
export interface Props {
  variant?: 'card' | 'table' | 'badge-list' | 'button-group' | 'list'
  lines?: number
}

withDefaults(defineProps<Props>(), {
  variant: 'card',
  lines: 3,
})
</script>

<style scoped>
.loading-skeleton {
  width: 100%;
  padding: 1rem;
  border-radius: 0.5rem;
  background: linear-gradient(120deg, rgba(148, 163, 184, 0.12), rgba(148, 163, 184, 0.04));
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.loading-skeleton-card {
  min-height: 120px;
}

.loading-skeleton-table {
  min-height: 180px;
}

.loading-skeleton-inline {
  padding: 0;
  background: transparent;
  border: 0;
}

.loading-skeleton-line,
.loading-skeleton-row,
.loading-skeleton-badge,
.loading-skeleton-button,
.loading-skeleton-list-item {
  background: linear-gradient(90deg, rgba(203, 213, 225, 0.2) 25%, rgba(203, 213, 225, 0.5) 50%, rgba(203, 213, 225, 0.2) 75%);
  background-size: 220% 100%;
  animation: loading-skeleton-wave 1.4s ease infinite;
}

.loading-skeleton-line {
  height: 0.75rem;
  border-radius: 999px;
  margin-bottom: 0.6rem;
}

.loading-skeleton-line:last-child {
  margin-bottom: 0;
  width: 70%;
}

.loading-skeleton-row {
  height: 1rem;
  border-radius: 0.35rem;
  margin-bottom: 0.75rem;
}

.loading-skeleton-row:last-child {
  margin-bottom: 0;
}

.loading-skeleton-badge {
  width: 100px;
  height: 1.5rem;
  border-radius: 999px;
}

.loading-skeleton-button {
  width: 120px;
  height: 2.5rem;
  border-radius: 0.5rem;
}

.loading-skeleton-list-item {
  height: 2.5rem;
  border-radius: 0.5rem;
  margin-bottom: 0.5rem;
}

.loading-skeleton-list-item:last-child {
  margin-bottom: 0;
}

@keyframes loading-skeleton-wave {
  0% {
    background-position: 100% 0;
  }
  100% {
    background-position: -100% 0;
  }
}
</style>

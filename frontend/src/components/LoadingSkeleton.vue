<template>
  <div
    class="loading-skeleton"
    :class="`loading-skeleton-${variant}`"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="loading-skeleton-line"
    />
  </div>
</template>

<script setup>
defineProps({
  lines: {
    type: Number,
    default: 4,
  },
  variant: {
    type: String,
    default: 'card',
  },
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

.loading-skeleton-line {
  height: 0.75rem;
  border-radius: 999px;
  margin-bottom: 0.6rem;
  background: linear-gradient(90deg, rgba(203, 213, 225, 0.2) 25%, rgba(203, 213, 225, 0.5) 50%, rgba(203, 213, 225, 0.2) 75%);
  background-size: 220% 100%;
  animation: loading-skeleton-wave 1.4s ease infinite;
}

.loading-skeleton-line:last-child {
  margin-bottom: 0;
  width: 70%;
}

.loading-skeleton-card {
  min-height: 120px;
}

.loading-skeleton-table {
  min-height: 180px;
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

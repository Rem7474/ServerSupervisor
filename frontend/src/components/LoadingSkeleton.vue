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

  <div
    v-else-if="variant === 'kpi'"
    class="row row-cards mb-4"
    aria-hidden="true"
  >
    <div
      v-for="n in lines"
      :key="n"
      class="col-6 col-lg-3"
    >
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="loading-skeleton-kpi-label" />
          <div class="loading-skeleton-kpi-value" />
          <div class="loading-skeleton-kpi-sub" />
        </div>
      </div>
    </div>
  </div>

  <div
    v-else-if="variant === 'chart'"
    class="loading-skeleton-chart"
    aria-hidden="true"
  >
    <div class="loading-skeleton-chart-inner d-flex align-items-end gap-1 h-100">
      <div
        v-for="n in 10"
        :key="n"
        class="loading-skeleton-chart-bar flex-grow-1"
        :style="`height:${30 + ((n * 37 + n * n * 13) % 55)}%;animation-delay:${(n - 1) * 0.07}s`"
      />
    </div>
  </div>

  <div
    v-else-if="variant === 'donut'"
    class="loading-skeleton loading-skeleton-donut"
    aria-hidden="true"
  >
    <div class="loading-skeleton-donut-chart">
      <div class="loading-skeleton-donut-ring" />
      <div class="loading-skeleton-donut-center" />
    </div>
    <div class="loading-skeleton-donut-legend">
      <div
        v-for="n in 3"
        :key="n"
        class="loading-skeleton-donut-legend-item"
        :style="`width:${72 - n * 10}%;animation-delay:${(n - 1) * 0.08}s`"
      />
    </div>
  </div>

  <div
    v-else-if="variant === 'proxmox-cluster'"
    class="card mb-4"
    aria-hidden="true"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <div class="loading-skeleton-pxmx-title" />
      <div class="loading-skeleton-pxmx-btn" />
    </div>
    <div class="card-body">
      <div class="row g-3 mb-4">
        <div
          v-for="k in 4"
          :key="k"
          class="col-6 col-md-3"
        >
          <div class="loading-skeleton-pxmx-kpi-label mb-2" />
          <div class="loading-skeleton-pxmx-kpi-val mb-2" />
          <div class="loading-skeleton-pxmx-kpi-sub" />
        </div>
      </div>
      <div class="row g-2">
        <div
          v-for="n in lines"
          :key="n"
          class="col-12 col-md-6 col-xl-4"
        >
          <div class="d-flex align-items-center gap-2 p-2">
            <div class="loading-skeleton-pxmx-dot" />
            <div class="flex-grow-1">
              <div class="d-flex justify-content-between mb-1">
                <div class="loading-skeleton-pxmx-name" />
                <div class="loading-skeleton-pxmx-stats" />
              </div>
              <div class="loading-skeleton-pxmx-bar mb-1" />
              <div class="loading-skeleton-pxmx-bar" />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
export interface Props {
  variant?: 'card' | 'table' | 'badge-list' | 'button-group' | 'list' | 'kpi' | 'proxmox-cluster' | 'chart' | 'donut'
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
  border: 1px solid var(--ss-border-default);
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
.loading-skeleton-list-item,
.loading-skeleton-kpi-label,
.loading-skeleton-kpi-value,
.loading-skeleton-kpi-sub,
.loading-skeleton-pxmx-title,
.loading-skeleton-pxmx-btn,
.loading-skeleton-pxmx-kpi-label,
.loading-skeleton-pxmx-kpi-val,
.loading-skeleton-pxmx-kpi-sub,
.loading-skeleton-pxmx-dot,
.loading-skeleton-pxmx-name,
.loading-skeleton-pxmx-stats,
.loading-skeleton-pxmx-bar,
.loading-skeleton-chart-bar {
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

/* kpi variant */
.loading-skeleton-kpi-label {
  height: 0.65rem;
  width: 55%;
  border-radius: 999px;
  margin-bottom: 0.6rem;
}

.loading-skeleton-kpi-value {
  height: 2rem;
  width: 45%;
  border-radius: 0.35rem;
  margin-bottom: 0.5rem;
}

.loading-skeleton-kpi-sub {
  height: 0.6rem;
  width: 75%;
  border-radius: 999px;
}

/* proxmox-cluster variant */
.loading-skeleton-pxmx-title {
  height: 1rem;
  width: 160px;
  border-radius: 999px;
}

.loading-skeleton-pxmx-btn {
  height: 28px;
  width: 72px;
  border-radius: 0.35rem;
}

.loading-skeleton-pxmx-kpi-label {
  height: 0.6rem;
  width: 50%;
  border-radius: 999px;
}

.loading-skeleton-pxmx-kpi-val {
  height: 1.5rem;
  width: 40%;
  border-radius: 0.35rem;
}

.loading-skeleton-pxmx-kpi-sub {
  height: 0.55rem;
  width: 65%;
  border-radius: 999px;
}

.loading-skeleton-pxmx-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.loading-skeleton-pxmx-name {
  height: 0.7rem;
  width: 50%;
  border-radius: 999px;
}

.loading-skeleton-pxmx-stats {
  height: 0.7rem;
  width: 25%;
  border-radius: 999px;
}

.loading-skeleton-pxmx-bar {
  height: 4px;
  border-radius: 999px;
}

/* chart variant */
.loading-skeleton-chart {
  width: 100%;
  height: 100%;
  min-height: 160px;
  padding: 0.5rem 0.25rem 0;
}

.loading-skeleton-chart-bar {
  border-radius: 3px 3px 0 0;
  min-width: 4px;
}

/* donut variant */
.loading-skeleton-donut {
  min-height: 160px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.9rem;
}

.loading-skeleton-donut-chart {
  position: relative;
  width: 122px;
  height: 122px;
  flex-shrink: 0;
}

.loading-skeleton-donut-ring {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: linear-gradient(135deg, rgba(203, 213, 225, 0.18), rgba(203, 213, 225, 0.52), rgba(203, 213, 225, 0.18));
  animation: loading-skeleton-wave 1.4s ease infinite;
}

.loading-skeleton-donut-center {
  position: absolute;
  inset: 28px;
  border-radius: 50%;
  background: var(--ss-panel-medium);
  border: 1px solid var(--ss-border-soft);
}

.loading-skeleton-donut-legend {
  width: min(180px, 100%);
  display: grid;
  gap: 0.45rem;
}

.loading-skeleton-donut-legend-item {
  height: 0.7rem;
  margin: 0 auto;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(203, 213, 225, 0.2) 25%, rgba(203, 213, 225, 0.5) 50%, rgba(203, 213, 225, 0.2) 75%);
  background-size: 220% 100%;
  animation: loading-skeleton-wave 1.4s ease infinite;
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

<template>
  <div class="row row-cards mb-4">
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body text-center">
          <div class="text-secondary small mb-1">
            Requêtes totales
          </div>
          <div class="h2 mb-0">
            {{ numberFormat(traffic.total_requests || 0) }}
          </div>
          <div
            class="small mt-1"
            :class="deltaClass('total_requests')"
          >
            {{ deltaLabel('total_requests') }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body text-center">
          <div class="text-secondary small mb-1">
            Bande passante
          </div>
          <div class="h2 mb-0">
            {{ formatBytes(traffic.total_bytes || 0) }}
          </div>
          <div
            class="small mt-1"
            :class="deltaClass('total_bytes')"
          >
            {{ deltaLabel('total_bytes') }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body text-center">
          <div class="text-secondary small mb-1">
            Taux 5xx
          </div>
          <div class="h2 mb-0 text-red">
            {{ percent(traffic.ratio_5xx) }}
          </div>
          <div
            class="small mt-1"
            :class="deltaClass('ratio_5xx')"
          >
            {{ deltaLabel('ratio_5xx') }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body text-center">
          <div class="text-secondary small mb-1">
            IPs suspectes
          </div>
          <div class="h2 mb-0">
            {{ numberFormat(threats.suspicious_ips || 0) }}
          </div>
          <div
            class="small mt-1"
            :class="deltaClass('suspicious_ips')"
          >
            {{ deltaLabel('suspicious_ips') }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for aggregate web-logs data (no Go model); typed in the Phase 7 split
type AnyRecord = Record<string, any>

const props = defineProps<{
  traffic: AnyRecord
  threats: AnyRecord
  compare: AnyRecord
}>()

function numberFormat(v: number): string {
  return new Intl.NumberFormat('fr-FR').format(Number(v) || 0)
}

function formatBytes(bytes: number): string {
  const value = Number(bytes) || 0
  if (value < 1024) return `${value} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  return `${size.toFixed(1)} ${units[unit]}`
}

function percent(v: number): string {
  const n = Number(v) || 0
  return `${(n * 100).toFixed(2)}%`
}

function kpiDelta(metric: string): number | null {
  const raw = props.compare?.delta_percent?.[metric]
  if (raw === null || raw === undefined) return null
  const n = Number(raw)
  return Number.isFinite(n) ? n : null
}

function deltaClass(metric: string): string {
  const value = kpiDelta(metric)
  if (value === null) return 'text-secondary'

  const increaseIsBad = metric === 'ratio_5xx' || metric === 'suspicious_ips'
  if (!increaseIsBad) {
    if (value > 0) return 'text-green'
    if (value < 0) return 'text-red'
  } else {
    if (value > 0) return 'text-red'
    if (value < 0) return 'text-green'
  }
  return 'text-secondary'
}

function deltaLabel(metric: string): string {
  const v = kpiDelta(metric)
  if (v === null) return 'N/A vs période précédente'
  const sign = v > 0 ? '+' : ''
  return `${sign}${v.toFixed(1)}% vs période précédente`
}
</script>

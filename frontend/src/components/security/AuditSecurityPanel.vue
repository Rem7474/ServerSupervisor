<template>
  <div>
    <!-- Period selector -->
    <div class="d-flex align-items-center justify-content-between mb-3">
      <div class="btn-group btn-group-sm">
        <button
          v-for="p in periodOptions"
          :key="p.hours"
          type="button"
          class="btn"
          :class="period === p.hours ? 'btn-primary' : 'btn-outline-secondary'"
          @click="emit('set-period', p.hours)"
        >
          {{ p.label }}
        </button>
      </div>
    </div>

    <!-- Stats cards -->
    <div class="row row-cards mb-4">
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Connexions ({{ periodLabel }})
            </div>
            <div class="h2 mb-0">
              {{ security.stats?.total ?? '—' }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Échecs ({{ periodLabel }})
            </div>
            <div class="h2 mb-0 text-danger">
              {{ security.stats?.failures ?? '—' }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              IPs uniques ({{ periodLabel }})
            </div>
            <div class="h2 mb-0 text-azure">
              {{ security.stats?.unique_ips ?? '—' }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- IPs bloquées + Top failed IPs -->
    <div class="row row-cards mb-4">
      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">
              IPs bloquées
            </h3>
          </div>
          <div class="card-body p-0">
            <div
              v-if="!security.blocked_ips?.length"
              class="text-center py-4 text-secondary small"
            >
              Aucune IP bloquée
            </div>
            <div v-else>
              <div
                v-for="ip in security.blocked_ips"
                :key="ip"
                class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom"
              >
                <div class="d-flex align-items-center gap-2">
                  <span class="badge bg-red-lt text-red">Bloquée</span>
                  <span class="font-monospace small">{{ ip }}</span>
                </div>
                <button
                  type="button"
                  class="btn btn-sm btn-outline-success"
                  :disabled="unblockingIp === ip"
                  @click="emit('unblock', ip)"
                >
                  {{ unblockingIp === ip ? '…' : 'Débloquer' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="col-lg-7">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">
              Top IPs — échecs de connexion ({{ periodLabel }})
            </h3>
          </div>
          <div class="card-body p-0">
            <div
              v-if="!security.top_failed_ips?.length"
              class="text-center py-4 text-secondary small"
            >
              Aucun échec enregistré sur cette période
            </div>
            <div v-else>
              <div
                v-for="item in security.top_failed_ips"
                :key="item.ip_address"
                class="px-3 py-2 border-bottom"
              >
                <div class="d-flex align-items-center justify-content-between mb-1">
                  <span class="font-monospace small">{{ item.ip_address }}</span>
                  <span class="badge bg-red-lt text-red">{{ item.fail_count }} échecs</span>
                </div>
                <div
                  class="progress"
                  style="height: 4px;"
                >
                  <div
                    class="progress-bar bg-danger"
                    :style="{ width: progressWidth(item.fail_count) + '%' }"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
export interface SecurityFailedIp { ip_address: string; fail_count: number }
export interface SecurityStats { total?: number; failures?: number; unique_ips?: number }
export interface SecurityData {
  stats: SecurityStats | null
  blocked_ips: string[]
  top_failed_ips: SecurityFailedIp[]
}
export interface SecurityPeriodOption { hours: number; label: string }

const props = defineProps<{
  security: SecurityData
  period: number
  periodLabel: string
  periodOptions: SecurityPeriodOption[]
  unblockingIp: string
}>()

const emit = defineEmits<{
  (e: 'set-period', hours: number): void
  (e: 'unblock', ip: string): void
}>()

function progressWidth(failCount: number): number {
  const max = Math.max(...(props.security.top_failed_ips?.map((i) => i.fail_count) || [1]))
  return max > 0 ? Math.round((failCount / max) * 100) : 0
}
</script>

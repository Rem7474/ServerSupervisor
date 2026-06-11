<template>
  <div>
    <div class="d-flex align-items-center justify-content-between mb-4">
      <div>
        <h3 class="mb-1">
          Sécurité de l'application
        </h3>
        <div class="text-secondary small">
          Connexions, échecs d'authentification et IPs bloquées sur ServerSupervisor
        </div>
      </div>
      <div class="d-flex align-items-center gap-2">
        <div class="btn-group btn-group-sm">
          <button
            v-for="p in periodOptions"
            :key="p.hours"
            type="button"
            class="btn"
            :class="period === p.hours ? 'btn-primary' : 'btn-outline-secondary'"
            @click="setPeriod(p.hours)"
          >
            {{ p.label }}
          </button>
        </div>
        <button
          type="button"
          class="btn btn-sm btn-outline-secondary"
          :disabled="loading"
          @click="load"
        >
          <span
            v-if="loading"
            class="spinner-border spinner-border-sm"
          />
          <span v-else>↻</span>
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-4"
    >
      {{ error }}
    </div>

    <div class="row row-cards mb-4">
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Connexions ({{ periodLabel }})
            </div>
            <div class="h2 mb-0">
              {{ data.stats?.total ?? '—' }}
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
              {{ data.stats?.failures ?? '—' }}
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
            <div class="h2 mb-0">
              {{ data.stats?.unique_ips ?? '—' }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards">
      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">
              IPs bloquées
            </h3>
          </div>
          <div class="card-body p-0">
            <div
              v-if="loading && !data.blocked_ips?.length"
              class="p-3"
            >
              <div
                v-for="i in 3"
                :key="i"
                class="placeholder-glow mb-2"
              >
                <span class="placeholder col-8" />
              </div>
            </div>
            <div
              v-else-if="!data.blocked_ips?.length"
              class="text-center py-4 text-secondary small"
            >
              Aucune IP bloquée
            </div>
            <div v-else>
              <div
                v-for="ip in data.blocked_ips"
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
                  :disabled="unblockingIP === ip"
                  @click="unblockIP(ip)"
                >
                  {{ unblockingIP === ip ? '…' : 'Débloquer' }}
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
              Top 10 IPs — échecs de connexion ({{ periodLabel }})
            </h3>
          </div>
          <div class="card-body p-0">
            <div
              v-if="!data.top_failed_ips?.length"
              class="text-center py-4 text-secondary small"
            >
              Aucun échec enregistré sur cette période
            </div>
            <div v-else>
              <div
                v-for="item in data.top_failed_ips"
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
import { ref, computed, onMounted } from 'vue'
import api from '../../api'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

interface FailedIP { ip_address: string; fail_count: number }
interface SecurityData {
  stats: { total?: number; failures?: number; unique_ips?: number } | null
  blocked_ips: string[]
  top_failed_ips: FailedIP[]
}

const periodOptions = [
  { hours: 24, label: '24h' },
  { hours: 168, label: '7j' },
  { hours: 720, label: '30j' },
]

const dialog = useConfirmDialog()
const data = ref<SecurityData>({ stats: null, blocked_ips: [], top_failed_ips: [] })
const loading = ref(false)
const error = ref('')
const unblockingIP = ref('')
const period = ref(24)
const periodLabel = computed(() => periodOptions.find((p) => p.hours === period.value)?.label ?? '24h')

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getSecuritySummary(period.value)
    data.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Impossible de charger les données de sécurité'
  } finally {
    loading.value = false
  }
}

function setPeriod(hours: number): void {
  period.value = hours
  load()
}

async function unblockIP(ip: string): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Débloquer cette IP',
    message: `Retirer l'IP ${ip} de la liste noire ?`,
    variant: 'warning',
  })
  if (!ok) return
  unblockingIP.value = ip
  try {
    await api.unblockIP(ip)
    await load()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Impossible de débloquer cette IP'
  } finally {
    unblockingIP.value = ''
  }
}

function progressWidth(failCount: number): number {
  const max = Math.max(...(data.value.top_failed_ips?.map((i) => i.fail_count) || [1]))
  return max > 0 ? Math.round((failCount / max) * 100) : 0
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>Sécurité hôtes</span>
        </div>
        <h2 class="page-title">
          Sécurité infrastructure
        </h2>
        <div class="text-secondary">
          Supervision des menaces et des activités suspectes sur les hôtes
        </div>
      </div>
    </div>

    <div class="d-flex align-items-center justify-content-between mb-3">
      <div class="btn-group btn-group-sm">
        <button
          v-for="p in periodOptions"
          :key="p.hours"
          class="btn"
          :class="threatsPeriod === p.hours ? 'btn-primary' : 'btn-outline-secondary'"
          @click="setThreatsPeriod(p.hours)"
        >
          {{ p.label }}
        </button>
      </div>
      <button
        class="btn btn-sm btn-outline-secondary"
        :disabled="threatsLoading"
        @click="loadSecurity"
      >
        <span
          v-if="threatsLoading"
          class="spinner-border spinner-border-sm"
        />
        <span v-else>↻</span>
      </button>
    </div>

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
              Echecs ({{ periodLabel }})
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
            <div class="h2 mb-0">
              {{ security.stats?.unique_ips ?? '—' }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards">
      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">
              IPs bloquees
            </h3>
          </div>
          <div class="card-body p-0">
            <div
              v-if="threatsLoading && !security.blocked_ips?.length"
              class="text-center py-4 text-secondary"
            >
              Chargement...
            </div>
            <div
              v-else-if="!security.blocked_ips?.length"
              class="text-center py-4 text-secondary small"
            >
              Aucune IP bloquee actuellement
            </div>
            <div v-else>
              <div
                v-for="ip in security.blocked_ips"
                :key="ip"
                class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom"
              >
                <div class="d-flex align-items-center gap-2">
                  <span class="badge bg-red-lt text-red">Bloquee</span>
                  <span class="font-monospace small">{{ ip }}</span>
                </div>
                <button
                  class="btn btn-sm btn-outline-success"
                  :disabled="unblockingIP === ip"
                  @click="unblockIP(ip)"
                >
                  {{ unblockingIP === ip ? '...' : 'Debloquer' }}
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
              Top 10 IPs - echecs de connexion ({{ periodLabel }})
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
                  <span class="badge bg-red-lt text-red">{{ item.fail_count }} echecs</span>
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

<script setup>
import { ref, computed, onMounted } from 'vue'
import apiClient from '../api'

const periodOptions = [
  { hours: 24, label: '24h' },
  { hours: 168, label: '7j' },
  { hours: 720, label: '30j' },
]

const security = ref({ stats: null, blocked_ips: [], top_failed_ips: [] })
const threatsLoading = ref(false)
const unblockingIP = ref('')
const threatsPeriod = ref(24)
const periodLabel = computed(() => periodOptions.find(p => p.hours === threatsPeriod.value)?.label ?? '24h')

async function loadSecurity() {
  threatsLoading.value = true
  try {
    const res = await apiClient.getSecuritySummary(threatsPeriod.value)
    security.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
  } catch (e) {
    console.error('Failed to load security summary:', e)
  } finally {
    threatsLoading.value = false
  }
}

function setThreatsPeriod(hours) {
  threatsPeriod.value = hours
  loadSecurity()
}

async function unblockIP(ip) {
  unblockingIP.value = ip
  try {
    await apiClient.unblockIP(ip)
    await loadSecurity()
  } catch (e) {
    console.error('Failed to unblock IP:', e)
  } finally {
    unblockingIP.value = ''
  }
}

function progressWidth(failCount) {
  const max = Math.max(...(security.value.top_failed_ips?.map(i => i.fail_count) || [1]))
  return max > 0 ? Math.round((failCount / max) * 100) : 0
}

onMounted(loadSecurity)
</script>


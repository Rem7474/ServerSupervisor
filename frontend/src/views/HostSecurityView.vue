<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link to="/" class="text-decoration-none">Dashboard</router-link>
          <span class="text-muted mx-1">/</span>
          <span>Securite hotes</span>
        </div>
        <h2 class="page-title">Securite infrastructure</h2>
        <div class="text-secondary">Supervision des menaces et des activites suspectes sur les hotes</div>
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
        >{{ p.label }}</button>
      </div>
      <button class="btn btn-sm btn-outline-secondary" @click="loadSecurity" :disabled="threatsLoading">
        <span v-if="threatsLoading" class="spinner-border spinner-border-sm"></span>
        <span v-else>↻</span>
      </button>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Connexions ({{ periodLabel }})</div>
            <div class="h2 mb-0">{{ security.stats?.total ?? '—' }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Echecs ({{ periodLabel }})</div>
            <div class="h2 mb-0 text-danger">{{ security.stats?.failures ?? '—' }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">IPs uniques ({{ periodLabel }})</div>
            <div class="h2 mb-0">{{ security.stats?.unique_ips ?? '—' }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Requetes suspectes (logs web)</div>
            <div class="h2 mb-0 text-orange">{{ security.bot_detection?.total_suspicious_requests ?? 0 }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">IPs suspectes (logs web)</div>
            <div class="h2 mb-0">{{ botTopIPs.length }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Hotes impactes</div>
            <div class="h2 mb-0">{{ botHosts.length }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards">
      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">IPs bloquees</h3>
          </div>
          <div class="card-body p-0">
            <div v-if="threatsLoading && !security.blocked_ips?.length" class="text-center py-4 text-secondary">Chargement...</div>
            <div v-else-if="!security.blocked_ips?.length" class="text-center py-4 text-secondary small">
              Aucune IP bloquee actuellement
            </div>
            <div v-else>
              <div v-for="ip in security.blocked_ips" :key="ip" class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom">
                <div class="d-flex align-items-center gap-2">
                  <span class="badge bg-red-lt text-red">Bloquee</span>
                  <span class="font-monospace small">{{ ip }}</span>
                </div>
                <button class="btn btn-sm btn-outline-success" @click="unblockIP(ip)" :disabled="unblockingIP === ip">
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
            <h3 class="card-title">Top 10 IPs - echecs de connexion ({{ periodLabel }})</h3>
          </div>
          <div class="card-body p-0">
            <div v-if="!security.top_failed_ips?.length" class="text-center py-4 text-secondary small">
              Aucun echec enregistre sur cette periode
            </div>
            <div v-else>
              <div v-for="item in security.top_failed_ips" :key="item.ip_address" class="px-3 py-2 border-bottom">
                <div class="d-flex align-items-center justify-content-between mb-1">
                  <span class="font-monospace small">{{ item.ip_address }}</span>
                  <span class="badge bg-red-lt text-red">{{ item.fail_count }} echecs</span>
                </div>
                <div class="progress" style="height: 4px;">
                  <div class="progress-bar bg-danger" :style="{ width: progressWidth(item.fail_count) + '%' }"></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards mt-4">
      <div class="col-lg-7">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">Top IPs suspectes (logs Nginx/Apache/NPM)</h3>
          </div>
          <div class="card-body p-0">
            <div v-if="!botTopIPs.length" class="text-center py-4 text-secondary small">
              Aucune activite suspecte detectee dans les logs web.
            </div>
            <div v-else>
              <div v-for="item in botTopIPs" :key="item.ip" class="px-3 py-2 border-bottom">
                <div class="d-flex align-items-center justify-content-between mb-1">
                  <span class="font-monospace small">{{ item.ip }}</span>
                  <span class="badge bg-orange-lt text-orange">{{ item.hits }} hits</span>
                </div>
                <div class="d-flex align-items-center justify-content-between text-secondary small mb-1">
                  <span>Hosts: {{ item.host_count || 1 }}</span>
                  <span>Paths: {{ item.unique_paths || 0 }}</span>
                </div>
                <div class="progress" style="height: 4px;">
                  <div class="progress-bar bg-orange" :style="{ width: progressWidthBot(item.hits) + '%' }"></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title">Top chemins scannes</h3>
          </div>
          <div class="card-body p-0">
            <div v-if="!botTopPaths.length" class="text-center py-4 text-secondary small">
              Aucun chemin suspect detecte.
            </div>
            <div v-else>
              <div v-for="item in botTopPaths" :key="item.path" class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom">
                <span class="font-monospace small text-truncate me-2" style="max-width: 70%;">{{ item.path }}</span>
                <span class="badge bg-yellow-lt text-yellow">{{ item.hits }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="card mt-4">
      <div class="card-header">
        <h3 class="card-title">Hotes les plus cibles (logs web)</h3>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Hote</th>
              <th class="text-end">Requetes suspectes</th>
              <th class="text-end">IPs suspectes</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!botHosts.length">
              <td colspan="3" class="text-center text-secondary py-4">Aucune donnee bot-detection remontee par les agents.</td>
            </tr>
            <tr v-for="item in botHosts" :key="item.host_id">
              <td>{{ item.host_name || item.host_id }}</td>
              <td class="text-end">{{ item.suspicious_requests || 0 }}</td>
              <td class="text-end">{{ item.unique_suspicious_ips || 0 }}</td>
            </tr>
          </tbody>
        </table>
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
const botTopIPs = computed(() => security.value.bot_detection?.top_suspicious_ips || [])
const botTopPaths = computed(() => security.value.bot_detection?.top_suspicious_paths || [])
const botHosts = computed(() => security.value.bot_detection?.hosts || [])

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

function progressWidthBot(hits) {
  const max = Math.max(...(botTopIPs.value.map(i => i.hits) || [1]))
  return max > 0 ? Math.round((hits / max) * 100) : 0
}

onMounted(loadSecurity)
</script>

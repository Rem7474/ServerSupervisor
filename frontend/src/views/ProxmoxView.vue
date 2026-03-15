<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>Proxmox VE</span>
      </div>
      <h2 class="page-title">Proxmox VE</h2>
      <div class="text-secondary">Supervision de l'infrastructure de virtualisation</div>
    </div>

    <!-- Summary cards -->
    <div class="row row-cards mb-4">
      <div class="col-6 col-lg-3">
        <div class="card">
          <div class="card-body">
            <div class="d-flex align-items-center">
              <div class="subheader">Connexions</div>
            </div>
            <div class="h1 mt-2 mb-0">{{ summary.connection_count ?? '—' }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card">
          <div class="card-body">
            <div class="subheader">Nœuds</div>
            <div class="h1 mt-2 mb-0">{{ summary.node_count ?? '—' }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card">
          <div class="card-body">
            <div class="subheader">VMs / LXC</div>
            <div class="h1 mt-2 mb-0">
              <span class="text-primary">{{ summary.vm_count ?? '—' }}</span>
              <span class="text-muted fs-5 ms-1">/ {{ summary.lxc_count ?? '—' }}</span>
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card">
          <div class="card-body">
            <div class="subheader">Stockage utilisé</div>
            <div class="h1 mt-2 mb-0">{{ formatBytes(summary.storage_used) }}</div>
            <div class="text-muted small">sur {{ formatBytes(summary.storage_total) }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Cluster health signals (only shown when there are issues) -->
    <div v-if="hasHealthAlerts" class="row row-cards mb-4">
      <div v-if="summary.nodes_down > 0" class="col-6 col-lg-3">
        <div class="card border-danger">
          <div class="card-body">
            <div class="subheader text-danger">Nœuds hors ligne</div>
            <div class="h1 mt-2 mb-0 text-danger">{{ summary.nodes_down }}</div>
          </div>
        </div>
      </div>
      <div v-if="summary.storage_near_full > 0" class="col-6 col-lg-3">
        <div class="card border-warning">
          <div class="card-body">
            <div class="subheader text-warning">Stockages &gt; 80 %</div>
            <div class="h1 mt-2 mb-0 text-warning">{{ summary.storage_near_full }}</div>
          </div>
        </div>
      </div>
      <div v-if="summary.storage_offline > 0" class="col-6 col-lg-3">
        <div class="card border-danger">
          <div class="card-body">
            <div class="subheader text-danger">Stockages inactifs</div>
            <div class="h1 mt-2 mb-0 text-danger">{{ summary.storage_offline }}</div>
          </div>
        </div>
      </div>
      <div v-if="summary.recent_failed_tasks > 0" class="col-6 col-lg-3">
        <div class="card border-warning">
          <div class="card-body">
            <div class="subheader text-warning">Tâches échouées (24 h)</div>
            <div class="h1 mt-2 mb-0 text-warning">{{ summary.recent_failed_tasks }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Error / loading state -->
    <div v-if="loading" class="text-center py-5 text-muted">Chargement...</div>
    <div v-else-if="error" class="alert alert-danger">{{ error }}</div>

    <!-- Nodes table -->
    <div v-else class="card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title mb-0">Nœuds Proxmox</h3>
        <div class="d-flex gap-2">
          <select v-model="filterConnection" class="form-select form-select-sm" style="width:auto">
            <option value="">Toutes les connexions</option>
            <option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }}</option>
          </select>
          <button class="btn btn-sm btn-outline-secondary" @click="load">
            <svg class="icon icon-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 .49-3.86"/>
            </svg>
          </button>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Nœud</th>
              <th>Instance / Cluster</th>
              <th>VMs</th>
              <th>LXC</th>
              <th>CPU</th>
              <th>RAM</th>
              <th>Statut</th>
              <th>Dernier contact</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="filteredNodes.length === 0">
              <td colspan="9" class="text-center text-muted py-4">
                Aucun nœud Proxmox trouvé.
                <router-link v-if="auth.isAdmin" to="/settings" class="ms-1">Configurer une connexion</router-link>
              </td>
            </tr>
            <tr v-for="node in filteredNodes" :key="node.id">
              <td>
                <div class="fw-medium">{{ node.node_name }}</div>
                <div class="text-muted small">{{ node.ip_address }}</div>
              </td>
              <td>
                <span v-if="node.cluster_name" class="text-secondary">{{ node.cluster_name }}</span>
                <span v-else class="text-muted">—</span>
              </td>
              <td>{{ node.vm_count }}</td>
              <td>{{ node.lxc_count }}</td>
              <td>
                <div class="d-flex align-items-center gap-2">
                  <div class="progress progress-xs flex-grow-1" style="min-width:60px">
                    <div class="progress-bar" :class="cpuColor(node.cpu_usage)" :style="`width:${(node.cpu_usage * 100).toFixed(1)}%`"></div>
                  </div>
                  <span class="text-muted small">{{ (node.cpu_usage * 100).toFixed(1) }}%</span>
                </div>
              </td>
              <td>
                <div class="d-flex align-items-center gap-2">
                  <div class="progress progress-xs flex-grow-1" style="min-width:60px">
                    <div class="progress-bar" :class="ramColor(node.mem_used, node.mem_total)" :style="`width:${memPct(node)}%`"></div>
                  </div>
                  <span class="text-muted small">{{ formatBytes(node.mem_used) }} / {{ formatBytes(node.mem_total) }}</span>
                </div>
              </td>
              <td>
                <span v-if="node.status === 'online'" class="badge bg-success-lt text-success">En ligne</span>
                <span v-else class="badge bg-danger-lt text-danger">{{ node.status }}</span>
              </td>
              <td class="text-muted small">{{ formatDate(node.last_seen_at) }}</td>
              <td>
                <router-link :to="`/proxmox/nodes/${node.id}`" class="btn btn-sm btn-outline-primary">
                  Détail
                </router-link>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index.js'

const auth = useAuthStore()

const summary = ref({})
const nodes = ref([])
const instances = ref([])
const filterConnection = ref('')
const loading = ref(true)
const error = ref('')

const filteredNodes = computed(() =>
  filterConnection.value
    ? nodes.value.filter(n => n.connection_id === filterConnection.value)
    : nodes.value
)

const hasHealthAlerts = computed(() =>
  (summary.value.nodes_down ?? 0) > 0 ||
  (summary.value.storage_near_full ?? 0) > 0 ||
  (summary.value.storage_offline ?? 0) > 0 ||
  (summary.value.recent_failed_tasks ?? 0) > 0
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [sumRes, nodesRes, instRes] = await Promise.all([
      api.getProxmoxSummary(),
      api.getProxmoxNodes(),
      api.getProxmoxInstances(),
    ])
    summary.value = sumRes.data
    nodes.value = nodesRes.data
    instances.value = instRes.data
  } catch (e) {
    error.value = e?.response?.data?.error || 'Erreur lors du chargement.'
  } finally {
    loading.value = false
  }
}

function memPct(node) {
  if (!node.mem_total) return 0
  return ((node.mem_used / node.mem_total) * 100).toFixed(1)
}

function cpuColor(usage) {
  if (usage > 0.85) return 'bg-danger'
  if (usage > 0.6) return 'bg-warning'
  return 'bg-success'
}

function ramColor(used, total) {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-success'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

onMounted(load)
</script>

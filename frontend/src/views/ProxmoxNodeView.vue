<template>
  <div>
    <div v-if="loading" class="text-center py-5 text-muted">Chargement...</div>
    <div v-else-if="error" class="alert alert-danger">{{ error }}</div>
    <template v-else-if="node">
      <!-- Header -->
      <div class="page-header mb-4">
        <div class="page-pretitle">
          <router-link to="/" class="text-decoration-none">Dashboard</router-link>
          <span class="text-muted mx-1">/</span>
          <router-link to="/proxmox" class="text-decoration-none">Proxmox VE</router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ node.node_name }}</span>
        </div>
        <div class="d-flex align-items-center gap-3">
          <h2 class="page-title mb-0">{{ node.node_name }}</h2>
          <span v-if="node.status === 'online'" class="badge bg-success-lt text-success">En ligne</span>
          <span v-else class="badge bg-danger-lt text-danger">{{ node.status }}</span>
        </div>
        <div class="text-secondary">{{ node.cluster_name || 'Nœud standalone' }} · PVE {{ node.pve_version || 'N/A' }} · {{ node.ip_address }}</div>
      </div>

      <!-- Stats row -->
      <div class="row row-cards mb-4">
        <div class="col-6 col-lg-3">
          <div class="card">
            <div class="card-body">
              <div class="subheader">CPU</div>
              <div class="h1 mt-2 mb-1">{{ (node.cpu_usage * 100).toFixed(1) }}%</div>
              <div class="text-muted small">{{ node.cpu_count }} cœurs</div>
              <div class="progress progress-xs mt-2">
                <div class="progress-bar" :class="cpuColor(node.cpu_usage)" :style="`width:${(node.cpu_usage*100).toFixed(1)}%`"></div>
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card">
            <div class="card-body">
              <div class="subheader">RAM</div>
              <div class="h1 mt-2 mb-1">{{ formatBytes(node.mem_used) }}</div>
              <div class="text-muted small">sur {{ formatBytes(node.mem_total) }}</div>
              <div class="progress progress-xs mt-2">
                <div class="progress-bar" :class="ramColor(node.mem_used, node.mem_total)" :style="`width:${memPct(node)}%`"></div>
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card">
            <div class="card-body">
              <div class="subheader">Uptime</div>
              <div class="h1 mt-2 mb-0">{{ formatUptime(node.uptime) }}</div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card">
            <div class="card-body">
              <div class="subheader">Guests</div>
              <div class="h1 mt-2 mb-0">
                <span class="text-primary">{{ node.vm_count }}</span>
                <span class="text-muted fs-5 ms-1">VMs</span>
                <span class="ms-2 text-info">{{ node.lxc_count }}</span>
                <span class="text-muted fs-5 ms-1">LXC</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <div class="card">
        <div class="card-header">
          <ul class="nav nav-tabs card-header-tabs">
            <li class="nav-item">
              <button class="nav-link" :class="{ active: tab === 'vms' }" @click="tab = 'vms'">
                VMs ({{ vms.length }})
              </button>
            </li>
            <li class="nav-item">
              <button class="nav-link" :class="{ active: tab === 'lxc' }" @click="tab = 'lxc'">
                LXC ({{ lxcs.length }})
              </button>
            </li>
            <li class="nav-item">
              <button class="nav-link" :class="{ active: tab === 'storage' }" @click="tab = 'storage'">
                Stockage ({{ node.storages?.length ?? 0 }})
              </button>
            </li>
          </ul>
        </div>

        <!-- VMs tab -->
        <div v-if="tab === 'vms'" class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>VMID</th>
                <th>Nom</th>
                <th>Statut</th>
                <th>CPU alloué</th>
                <th>CPU utilisé</th>
                <th>RAM allouée</th>
                <th>RAM utilisée</th>
                <th>Disque</th>
                <th>Uptime</th>
                <th>Tags</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="vms.length === 0">
                <td colspan="10" class="text-center text-muted py-4">Aucune VM sur ce nœud.</td>
              </tr>
              <tr v-for="g in vms" :key="g.id">
                <td class="text-muted">{{ g.vmid }}</td>
                <td class="fw-medium">{{ g.name || '—' }}</td>
                <td><span :class="guestStatusClass(g.status)">{{ g.status }}</span></td>
                <td>{{ g.cpu_alloc }} vCPU</td>
                <td>{{ (g.cpu_usage * 100).toFixed(1) }}%</td>
                <td>{{ formatBytes(g.mem_alloc) }}</td>
                <td>{{ formatBytes(g.mem_usage) }}</td>
                <td>{{ formatBytes(g.disk_alloc) }}</td>
                <td>{{ g.status === 'running' ? formatUptime(g.uptime) : '—' }}</td>
                <td>
                  <template v-if="g.tags">
                    <span v-for="tag in g.tags.split(';').filter(Boolean)" :key="tag" class="badge bg-blue-lt text-blue me-1">{{ tag.trim() }}</span>
                  </template>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- LXC tab -->
        <div v-if="tab === 'lxc'" class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>CT ID</th>
                <th>Nom</th>
                <th>Statut</th>
                <th>CPU alloué</th>
                <th>CPU utilisé</th>
                <th>RAM allouée</th>
                <th>RAM utilisée</th>
                <th>Disque</th>
                <th>Uptime</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="lxcs.length === 0">
                <td colspan="9" class="text-center text-muted py-4">Aucun conteneur LXC sur ce nœud.</td>
              </tr>
              <tr v-for="g in lxcs" :key="g.id">
                <td class="text-muted">{{ g.vmid }}</td>
                <td class="fw-medium">{{ g.name || '—' }}</td>
                <td><span :class="guestStatusClass(g.status)">{{ g.status }}</span></td>
                <td>{{ g.cpu_alloc }}</td>
                <td>{{ (g.cpu_usage * 100).toFixed(1) }}%</td>
                <td>{{ formatBytes(g.mem_alloc) }}</td>
                <td>{{ formatBytes(g.mem_usage) }}</td>
                <td>{{ formatBytes(g.disk_alloc) }}</td>
                <td>{{ g.status === 'running' ? formatUptime(g.uptime) : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Storage tab -->
        <div v-if="tab === 'storage'" class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Stockage</th>
                <th>Type</th>
                <th>Total</th>
                <th>Utilisé</th>
                <th>Disponible</th>
                <th>Utilisation</th>
                <th>Partagé</th>
                <th>Statut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!node.storages?.length">
                <td colspan="8" class="text-center text-muted py-4">Aucun stockage sur ce nœud.</td>
              </tr>
              <tr v-for="s in node.storages" :key="s.id">
                <td class="fw-medium">{{ s.storage_name }}</td>
                <td><span class="badge bg-secondary-lt text-secondary">{{ s.storage_type }}</span></td>
                <td>{{ formatBytes(s.total) }}</td>
                <td>{{ formatBytes(s.used) }}</td>
                <td>{{ formatBytes(s.avail) }}</td>
                <td>
                  <div class="d-flex align-items-center gap-2">
                    <div class="progress progress-xs flex-grow-1" style="min-width:80px">
                      <div class="progress-bar" :class="storageColor(s.used, s.total)" :style="`width:${storagePct(s)}%`"></div>
                    </div>
                    <span class="text-muted small">{{ storagePct(s) }}%</span>
                  </div>
                </td>
                <td>
                  <span v-if="s.shared" class="badge bg-azure-lt text-azure">Oui</span>
                  <span v-else class="text-muted">—</span>
                </td>
                <td>
                  <span v-if="s.active && s.enabled" class="badge bg-success-lt text-success">Actif</span>
                  <span v-else class="badge bg-danger-lt text-danger">Inactif</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api/index.js'

const route = useRoute()
const node = ref(null)
const loading = ref(true)
const error = ref('')
const tab = ref('vms')

const vms = computed(() => node.value?.guests?.filter(g => g.guest_type === 'vm') ?? [])
const lxcs = computed(() => node.value?.guests?.filter(g => g.guest_type === 'lxc') ?? [])

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getProxmoxNode(route.params.id)
    node.value = res.data
  } catch (e) {
    error.value = e?.response?.data?.error || 'Erreur lors du chargement.'
  } finally {
    loading.value = false
  }
}

function memPct(n) {
  if (!n.mem_total) return 0
  return ((n.mem_used / n.mem_total) * 100).toFixed(1)
}

function storagePct(s) {
  if (!s.total) return 0
  return ((s.used / s.total) * 100).toFixed(1)
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

function storageColor(used, total) {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-primary'
}

function guestStatusClass(status) {
  const map = {
    running: 'badge bg-success-lt text-success',
    stopped: 'badge bg-secondary-lt text-secondary',
    paused: 'badge bg-warning-lt text-warning',
  }
  return map[status] ?? 'badge bg-secondary-lt text-secondary'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0, v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatUptime(seconds) {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}j ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

onMounted(load)
</script>

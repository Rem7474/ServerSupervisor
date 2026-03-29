<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <div class="d-flex align-items-center gap-2">
        <!-- Proxmox logo-style icon -->
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
             stroke-linecap="round" stroke-linejoin="round" class="text-orange">
          <rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>
        </svg>
        <h3 class="card-title mb-0">Cluster Proxmox</h3>
        <span v-if="clusterName" class="text-secondary small">— {{ clusterName }}</span>
      </div>
      <router-link to="/proxmox" class="btn btn-sm btn-outline-secondary">Détails</router-link>
    </div>

    <div class="card-body">
      <!-- KPI row -->
      <div class="row g-3 mb-4">
        <div class="col-6 col-md-3">
          <div class="d-flex flex-column">
            <div class="subheader">Nœuds</div>
            <div class="d-flex align-items-baseline gap-1">
              <span class="h2 mb-0" :class="nodesDown > 0 ? 'text-red' : 'text-green'">{{ onlineNodes }}</span>
              <span class="text-secondary small">/ {{ nodes.length }}</span>
            </div>
            <div v-if="nodesDown > 0" class="text-red small">{{ nodesDown }} hors ligne</div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="d-flex flex-column">
            <div class="subheader">VMs / LXC</div>
            <div class="h2 mb-0">{{ totalVMs + totalLXC }}</div>
            <div class="text-secondary small">{{ totalVMs }} VM · {{ totalLXC }} LXC</div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="d-flex flex-column">
            <div class="subheader">CPU cluster</div>
            <div class="h2 mb-0" :class="clusterCpuColor">{{ clusterCpuPct.toFixed(1) }}%</div>
            <div class="text-secondary small">{{ totalCpus }} cœurs</div>
          </div>
        </div>
        <div class="col-6 col-md-3">
          <div class="d-flex flex-column">
            <div class="subheader">RAM cluster</div>
            <div class="h2 mb-0" :class="clusterRamColor">{{ clusterRamPct.toFixed(1) }}%</div>
            <div class="text-secondary small">{{ formatBytes(totalMemUsed) }} / {{ formatBytes(totalMemTotal) }}</div>
          </div>
        </div>
      </div>

      <!-- Per-node bars -->
      <div class="row g-2">
        <div v-for="node in nodes" :key="node.id" class="col-12 col-md-6 col-xl-4">
          <router-link :to="`/proxmox/nodes/${node.id}`" class="text-decoration-none">
            <div class="d-flex align-items-center gap-2 p-2 rounded hover-bg">
              <!-- Status dot -->
              <span :class="node.status === 'online' ? 'badge bg-green node-status-dot' : 'badge bg-red node-status-dot'"></span>
              <div class="flex-grow-1 min-width-0">
                <div class="d-flex justify-content-between align-items-center mb-1">
                  <span class="fw-semibold text-body small text-truncate">{{ node.node_name }}</span>
                  <span class="text-secondary small ms-2 flex-shrink-0">
                    CPU {{ (node.cpu_usage * 100).toFixed(0) }}%
                    · RAM {{ nodRamPct(node).toFixed(0) }}%
                  </span>
                </div>
                <!-- CPU bar -->
                <div class="progress mb-1 progress-thin">
                  <div
                    class="progress-bar"
                    :class="cpuBarColor(node.cpu_usage * 100)"
                    :style="{ width: Math.min(node.cpu_usage * 100, 100) + '%' }"
                  ></div>
                </div>
                <!-- RAM bar -->
                <div class="progress progress-thin">
                  <div
                    class="progress-bar"
                    :class="ramBarColor(nodRamPct(node))"
                    :style="{ width: Math.min(nodRamPct(node), 100) + '%' }"
                  ></div>
                </div>
              </div>
            </div>
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  nodes: { type: Array, default: () => [] },
})

const clusterName = computed(() => {
  const n = props.nodes.find(n => n.cluster_name)
  return n?.cluster_name || ''
})

const onlineNodes = computed(() => props.nodes.filter(n => n.status === 'online').length)
const nodesDown = computed(() => props.nodes.filter(n => n.status !== 'online').length)

const totalVMs = computed(() => props.nodes.reduce((s, n) => s + (n.vm_count || 0), 0))
const totalLXC = computed(() => props.nodes.reduce((s, n) => s + (n.lxc_count || 0), 0))
const totalCpus = computed(() => props.nodes.reduce((s, n) => s + (n.cpu_count || 0), 0))
const totalMemTotal = computed(() => props.nodes.reduce((s, n) => s + (n.mem_total || 0), 0))
const totalMemUsed = computed(() => props.nodes.reduce((s, n) => s + (n.mem_used || 0), 0))

// Weighted-average CPU across all online nodes (each node's cpu_usage * its cpu_count)
const clusterCpuPct = computed(() => {
  const online = props.nodes.filter(n => n.status === 'online')
  const totalW = online.reduce((s, n) => s + (n.cpu_count || 1), 0)
  if (totalW === 0) return 0
  const weighted = online.reduce((s, n) => s + (n.cpu_usage || 0) * (n.cpu_count || 1), 0)
  return (weighted / totalW) * 100
})

const clusterRamPct = computed(() => {
  if (totalMemTotal.value === 0) return 0
  return (totalMemUsed.value / totalMemTotal.value) * 100
})

const clusterCpuColor = computed(() => pctColor(clusterCpuPct.value))
const clusterRamColor = computed(() => pctColor(clusterRamPct.value))

function nodRamPct(node) {
  if (!node.mem_total) return 0
  return (node.mem_used / node.mem_total) * 100
}

function pctColor(pct) {
  if (pct > 90) return 'text-red'
  if (pct > 70) return 'text-yellow'
  return 'text-green'
}

function cpuBarColor(pct) {
  if (pct > 90) return 'bg-red'
  if (pct > 70) return 'bg-yellow'
  return 'bg-blue'
}

function ramBarColor(pct) {
  if (pct > 90) return 'bg-red'
  if (pct > 75) return 'bg-yellow'
  return 'bg-green'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return v.toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}
</script>

<style scoped>
.hover-bg {
  transition: background 0.15s;
}
.hover-bg:hover {
  background: rgba(var(--tblr-body-color-rgb), 0.04);
}
.min-width-0 {
  min-width: 0;
}

.node-status-dot {
  width: 8px;
  height: 8px;
  padding: 0;
  border-radius: 50%;
  flex-shrink: 0;
}

.progress-thin {
  height: 4px;
}
</style>

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

      <!-- Updates banner (only shown when pending updates exist) -->
      <div v-if="node.pending_updates > 0" class="alert mb-4" :class="node.security_updates > 0 ? 'alert-danger' : 'alert-warning'">
        <div class="d-flex align-items-center gap-3">
          <div>
            <strong>Mises à jour disponibles sur ce nœud :</strong>
            {{ node.pending_updates }} paquet(s) en attente
            <span v-if="node.security_updates > 0" class="ms-2 badge bg-danger">
              dont {{ node.security_updates }} de sécurité
            </span>
          </div>
          <div class="ms-auto text-muted small" v-if="node.last_update_check_at">
            Dernière vérification : {{ formatDate(node.last_update_check_at) }}
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
            <li class="nav-item">
              <button class="nav-link" :class="{ active: tab === 'disks' }" @click="tab = 'disks'">
                Disques ({{ node.disks?.length ?? 0 }})
              </button>
            </li>
            <li class="nav-item">
              <button class="nav-link" :class="{ active: tab === 'tasks' }" @click="tab = 'tasks'">
                Tâches ({{ node.tasks?.length ?? 0 }})
                <span v-if="failedTaskCount > 0" class="badge bg-warning ms-1">{{ failedTaskCount }}</span>
              </button>
            </li>
            <li class="nav-item">
              <button class="nav-link" :class="{ active: tab === 'updates' }" @click="tab = 'updates'">
                Mises à jour
                <span v-if="node.pending_updates > 0" class="badge ms-1" :class="node.security_updates > 0 ? 'bg-danger' : 'bg-warning'">
                  {{ node.pending_updates }}
                </span>
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
                <th>Hôte lié</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="vms.length === 0">
                <td colspan="11" class="text-center text-muted py-4">Aucune VM sur ce nœud.</td>
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
                <td>
                  <GuestLinkCell :link="linkForGuest(g)" @confirm="confirmGuestLink(g)" @ignore="ignoreGuestLink(g)" @go="goToHost(linkForGuest(g))" />
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
                <th>Hôte lié</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="lxcs.length === 0">
                <td colspan="10" class="text-center text-muted py-4">Aucun conteneur LXC sur ce nœud.</td>
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
                <td>
                  <GuestLinkCell :link="linkForGuest(g)" @confirm="confirmGuestLink(g)" @ignore="ignoreGuestLink(g)" @go="goToHost(linkForGuest(g))" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Link action feedback -->
        <div v-if="linkMsg" class="card-footer py-2">
          <span :class="['small', linkMsgOk ? 'text-success' : 'text-danger']">{{ linkMsg }}</span>
        </div>

        <!-- Disks tab -->
        <div v-if="tab === 'disks'" class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Périphérique</th>
                <th>Modèle</th>
                <th>Type</th>
                <th>Taille</th>
                <th>Santé SMART</th>
                <th>Usure SSD</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!node.disks?.length">
                <td colspan="6" class="text-center text-muted py-4">Aucun disque détecté sur ce nœud.</td>
              </tr>
              <tr v-for="d in node.disks" :key="d.id">
                <td class="fw-medium font-monospace">{{ d.dev_path }}</td>
                <td>{{ d.model || '—' }}<div class="text-muted small">{{ d.serial }}</div></td>
                <td><span class="badge bg-secondary-lt text-secondary text-uppercase">{{ d.disk_type || '?' }}</span></td>
                <td>{{ formatBytes(d.size_bytes) }}</td>
                <td>
                  <span v-if="d.health === 'PASSED'" class="badge bg-success-lt text-success">PASSED</span>
                  <span v-else-if="d.health === 'FAILED'" class="badge bg-danger-lt text-danger">FAILED</span>
                  <span v-else class="badge bg-secondary-lt text-secondary">{{ d.health }}</span>
                </td>
                <td>
                  <template v-if="d.wearout >= 0">
                    <div class="d-flex align-items-center gap-2">
                      <div class="progress progress-xs flex-grow-1" style="min-width:60px">
                        <div class="progress-bar" :class="wearoutColor(d.wearout)" :style="`width:${d.wearout}%`"></div>
                      </div>
                      <span class="text-muted small">{{ d.wearout }}%</span>
                    </div>
                  </template>
                  <span v-else class="text-muted">—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Tasks tab -->
        <div v-if="tab === 'tasks'" class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Type</th>
                <th>Objet</th>
                <th>Utilisateur</th>
                <th>Début</th>
                <th>Durée</th>
                <th>Statut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!node.tasks?.length">
                <td colspan="6" class="text-center text-muted py-4">Aucune tâche récente pour ce nœud.</td>
              </tr>
              <tr v-for="t in node.tasks" :key="t.id">
                <td><span class="badge bg-azure-lt text-azure font-monospace">{{ t.task_type }}</span></td>
                <td class="text-muted">{{ t.object_id || '—' }}</td>
                <td class="text-muted small">{{ t.user_name }}</td>
                <td class="text-muted small">{{ formatDate(t.start_time) }}</td>
                <td class="text-muted small">{{ taskDuration(t) }}</td>
                <td>
                  <span v-if="t.status === 'running'" class="badge bg-blue-lt text-blue">En cours</span>
                  <span v-else-if="t.exit_status === 'OK'" class="badge bg-success-lt text-success">OK</span>
                  <span v-else-if="t.exit_status" class="badge bg-danger-lt text-danger" :title="t.exit_status">Erreur</span>
                  <span v-else class="badge bg-secondary-lt text-secondary">{{ t.status }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Updates tab -->
        <div v-if="tab === 'updates'" class="card-body">
          <div v-if="node.pending_updates === 0" class="text-center text-muted py-4">
            <div class="mb-1">Aucune mise à jour en attente détectée.</div>
            <div v-if="node.last_update_check_at" class="small">
              Dernière vérification : {{ formatDate(node.last_update_check_at) }}
            </div>
            <div v-else class="small">Données non encore disponibles (prochain cycle de polling).</div>
          </div>
          <div v-else>
            <div class="d-flex align-items-center gap-3 mb-3">
              <div class="h2 mb-0">{{ node.pending_updates }}</div>
              <div>
                <div class="fw-medium">paquet(s) en attente de mise à jour</div>
                <div v-if="node.last_update_check_at" class="text-muted small">
                  Détecté le {{ formatDate(node.last_update_check_at) }}
                </div>
              </div>
              <div v-if="node.security_updates > 0" class="ms-auto">
                <span class="badge bg-danger fs-5 px-3 py-2">
                  {{ node.security_updates }} mise(s) à jour de sécurité
                </span>
              </div>
            </div>
            <div class="alert alert-info mb-0">
              Ces informations proviennent du cache apt du nœud Proxmox (lecture seule).
              Pour appliquer les mises à jour, connectez-vous directement au nœud.
            </div>
          </div>
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
import { ref, computed, onMounted, defineComponent, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../api/index.js'

// Inline component — renders the "Hôte lié" cell without a separate file.
const GuestLinkCell = defineComponent({
  props: { link: { type: Object, default: null } },
  emits: ['confirm', 'ignore', 'go'],
  setup(props, { emit }) {
    return () => {
      const link = props.link
      if (!link) return h('span', { class: 'text-muted small' }, '—')
      if (link.status === 'suggested') {
        return h('div', { class: 'd-flex align-items-center gap-1' }, [
          h('span', { class: 'badge bg-warning-lt text-warning' }, 'Suggéré'),
          h('span', { class: 'text-muted small' }, link.host_hostname || link.host_name),
          h('button', { class: 'btn btn-xs btn-success ms-1', onClick: () => emit('confirm') }, '✓'),
          h('button', { class: 'btn btn-xs btn-outline-secondary', onClick: () => emit('ignore') }, '✗'),
        ])
      }
      if (link.status === 'confirmed') {
        return h('div', { class: 'd-flex align-items-center gap-1' }, [
          h('span', { class: 'badge bg-success-lt text-success' }, 'Lié'),
          h('button', {
            class: 'btn btn-xs btn-outline-primary ms-1',
            onClick: () => emit('go'),
            title: 'Voir la fiche hôte',
          }, link.host_hostname || link.host_name),
        ])
      }
      return h('span', { class: 'text-muted small' }, '—')
    }
  },
})

const route = useRoute()
const router = useRouter()
const node = ref(null)
const loading = ref(true)
const error = ref('')
const tab = ref('vms')

// guest_id → link object (loaded after node data)
const guestLinks = ref({})
const linkMsg = ref('')
const linkMsgOk = ref(false)

const vms = computed(() => node.value?.guests?.filter(g => g.guest_type === 'vm') ?? [])
const lxcs = computed(() => node.value?.guests?.filter(g => g.guest_type === 'lxc') ?? [])
const failedTaskCount = computed(() =>
  (node.value?.tasks ?? []).filter(t => t.status === 'stopped' && t.exit_status && t.exit_status !== 'OK').length
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getProxmoxNode(route.params.id)
    node.value = res.data
    await loadGuestLinks()
  } catch (e) {
    error.value = e?.response?.data?.error || 'Erreur lors du chargement.'
  } finally {
    loading.value = false
  }
}

async function loadGuestLinks() {
  const guests = node.value?.guests ?? []
  if (guests.length === 0) return
  // One request for all links, then index by guest_id — avoids N individual requests.
  try {
    const res = await api.getProxmoxLinks()
    const guestIds = new Set(guests.map(g => g.id))
    const map = {}
    for (const link of res.data ?? []) {
      if (guestIds.has(link.guest_id)) {
        map[link.guest_id] = link
      }
    }
    guestLinks.value = map
  } catch {
    guestLinks.value = {}
  }
}

function linkForGuest(g) {
  return guestLinks.value[g.id] ?? null
}

async function confirmGuestLink(g) {
  const link = linkForGuest(g)
  if (!link) return
  try {
    const res = await api.updateProxmoxLink(link.id, { status: 'confirmed' })
    guestLinks.value = { ...guestLinks.value, [g.id]: res.data }
    showMsg(`[${g.name}] Lien confirmé.`, true)
  } catch (e) {
    showMsg(e?.response?.data?.error || 'Erreur.', false)
  }
}

async function ignoreGuestLink(g) {
  const link = linkForGuest(g)
  if (!link) return
  try {
    await api.deleteProxmoxLink(link.id)
    const m = { ...guestLinks.value }
    delete m[g.id]
    guestLinks.value = m
    showMsg(`[${g.name}] Suggestion ignorée.`, true)
  } catch (e) {
    showMsg(e?.response?.data?.error || 'Erreur.', false)
  }
}

function goToHost(link) {
  if (link?.host_id) router.push(`/hosts/${link.host_id}`)
}

function showMsg(msg, ok) {
  linkMsg.value = msg
  linkMsgOk.value = ok
  setTimeout(() => { linkMsg.value = '' }, 4000)
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

function formatDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

// wearout for SSD: 100=new, lower=more worn → invert to show wear percentage
function wearoutColor(wearout) {
  // wearout is wear level remaining (100=new). Low value = danger.
  if (wearout < 20) return 'bg-danger'
  if (wearout < 50) return 'bg-warning'
  return 'bg-success'
}

function taskDuration(t) {
  if (!t.start_time) return '—'
  const end = t.end_time ? new Date(t.end_time) : (t.status === 'running' ? new Date() : null)
  if (!end) return '—'
  const secs = Math.floor((end - new Date(t.start_time)) / 1000)
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}

onMounted(load)
</script>

<template>
  <!-- Filters -->
  <div class="card mb-4">
    <div class="card-body">
      <div class="row g-3">
        <div class="col-6 col-md-6 col-lg-3">
          <input v-model="searchInput" type="text" class="form-control" placeholder="Rechercher..." />
        </div>
        <div class="col-6 col-md-6 col-lg-3">
          <select v-model="hostFilter" class="form-select">
            <option value="">Tous les hôtes</option>
            <option v-for="h in uniqueHosts" :key="h" :value="h">{{ h }}</option>
          </select>
        </div>
        <div class="col-6 col-md-6 col-lg-3">
          <select v-model="stateFilter" class="form-select">
            <option value="">Tous les états</option>
            <option value="running">En cours</option>
            <option value="restarting">Redémarrage</option>
            <option value="paused">En pause</option>
            <option value="created">Créé</option>
            <option value="exited">Arrêté</option>
            <option value="dead">Mort</option>
          </select>
        </div>
        <div class="col-6 col-md-6 col-lg-3">
          <select v-model="composeFilter" class="form-select">
            <option value="">Tous les conteneurs</option>
            <option value="compose">Docker Compose</option>
            <option value="standalone">Standalone</option>
          </select>
        </div>
      </div>
    </div>
  </div>

  <div v-if="filteredContainers.length > 0" class="card">
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>Hôte</th>
            <th>Compose</th>
            <th>Image</th>
            <th>État</th>
            <th>Ports</th>
            <th>Réseau (Rx / Tx)</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in filteredContainers" :key="c.id">
            <td class="fw-semibold">{{ c.name }}</td>
            <td>
              <router-link :to="`/hosts/${c.host_id}`" class="text-decoration-none">
                {{ c.hostname }}
              </router-link>
            </td>
            <td>
              <div v-if="getComposeInfo(c).project" class="small">
                <div class="text-primary fw-semibold">{{ getComposeInfo(c).project }}</div>
                <div class="text-secondary">{{ getComposeInfo(c).service }}</div>
              </div>
              <span v-else class="text-secondary">-</span>
            </td>
            <td class="small">
              <div>{{ c.image }}</div>
              <div class="mt-1 d-flex align-items-center gap-1 flex-wrap">
                <code>{{ containerVersion(c)?.running_version || c.image_tag }}</code>
                <template v-if="containerVersion(c)">
                  <span v-if="containerVersion(c).is_up_to_date" class="badge bg-green-lt text-green">À jour</span>
                  <span v-else-if="containerVersion(c).running_version || containerVersion(c).update_confirmed" class="badge bg-yellow-lt text-yellow" :title="`Dernière version : ${containerVersion(c).latest_version}`">Mise à jour disponible</span>
                  <span v-else class="badge bg-secondary-lt text-secondary">Version inconnue</span>
                </template>
              </div>
            </td>
            <td>
              <span :class="stateClass(c.state)">{{ c.state }}</span>
            </td>
            <td class="d-none d-sm-table-cell text-secondary small font-monospace">{{ formatContainerPorts(c.ports) }}</td>
            <td class="text-secondary small font-monospace">
              <template v-if="c.state === 'running' && (c.net_rx_bytes > 0 || c.net_tx_bytes > 0)">
                ↓ {{ formatBytes(c.net_rx_bytes) }} / ↑ {{ formatBytes(c.net_tx_bytes) }}
              </template>
              <span v-else class="text-muted">—</span>
            </td>
            <td class="text-end">
              <div class="d-flex align-items-center justify-content-end gap-1">
                <template v-if="canRunDocker">
                  <button
                    v-if="['exited', 'dead', 'created', 'paused'].includes(c.state)"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'start', container: c })"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-sm btn-ghost-success"
                    title="Démarrer"
                    aria-label="Démarrer le conteneur"
                  >
                    <span v-if="actionLoading[c.name] === 'start'" class="spinner-border spinner-border-sm"></span>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M7 4v16l13 -8z" /></svg>
                  </button>
                  <button
                    v-if="c.state === 'running'"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'stop', container: c })"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-sm btn-ghost-danger"
                    title="Arrêter"
                    aria-label="Arrêter le conteneur"
                  >
                    <span v-if="actionLoading[c.name] === 'stop'" class="spinner-border spinner-border-sm"></span>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><rect x="4" y="4" width="16" height="16" rx="2" /></svg>
                  </button>
                  <button
                    v-if="c.state === 'running'"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'restart', container: c })"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-sm btn-ghost-warning"
                    title="Redémarrer"
                    aria-label="Redémarrer le conteneur"
                  >
                    <span v-if="actionLoading[c.name] === 'restart'" class="spinner-border spinner-border-sm"></span>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4" /><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" /></svg>
                  </button>
                  <button
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'logs', container: c })"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-sm btn-ghost-secondary"
                    title="Voir les logs"
                    aria-label="Voir les logs du conteneur"
                  >
                    <span v-if="actionLoading[c.name] === 'logs'" class="spinner-border spinner-border-sm"></span>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                  </button>
                </template>
                <button
                  @click="inspectTarget = c; inspectTab = 'env'"
                  class="btn btn-sm btn-ghost-secondary"
                  title="Inspecter"
                  aria-label="Inspecter le conteneur"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><circle cx="10" cy="10" r="7" /><path d="M21 21l-6 -6" /></svg>
                </button>
                <button
                  v-if="getComposeInfo(c).project || Object.keys(c.labels || {}).length > 0"
                  @click="selectedContainer = c"
                  class="btn btn-sm btn-ghost-secondary"
                  :title="getComposeInfo(c).project ? 'Infos Compose + Labels' : 'Labels'"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none">
                    <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                    <path d="M9 5H7a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2V7a2 2 0 0 0 -2 -2h-2"/>
                    <path d="M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2 -2a2 2 0 0 0 -2 -2h-2a2 2 0 0 0 -2 2z"/>
                    <path d="M9 12l.01 0"/><path d="M13 12l2 0"/><path d="M9 16l.01 0"/><path d="M13 16l2 0"/>
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <div v-if="filteredContainers.length === 0" class="text-center text-secondary py-5">
    <svg xmlns="http://www.w3.org/2000/svg" class="mb-2" width="40" height="40" fill="none" stroke="currentColor" viewBox="0 0 24 24" style="opacity:.35">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/>
    </svg>
    <div class="fw-medium">{{ search || hostFilter ? 'Aucun résultat pour ces filtres' : 'Aucun conteneur trouvé' }}</div>
    <div class="small mt-1 opacity-75">
      <template v-if="search || hostFilter">Modifiez vos critères de recherche</template>
      <template v-else>Connectez un hôte avec l'agent Docker activé pour voir vos conteneurs ici</template>
    </div>
    <router-link v-if="!search && !hostFilter" to="/hosts/new" class="btn btn-sm btn-primary mt-3">Ajouter un hôte</router-link>
  </div>

  <!-- Modal conteneur (labels/compose info) -->
  <div v-if="selectedContainer" class="modal modal-blur fade show" style="display: block;" @click.self="selectedContainer = null">
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">Details Docker Compose</h5>
          <button type="button" class="btn-close" @click="selectedContainer = null" aria-label="Fermer"></button>
        </div>
        <div class="modal-body">
          <div class="mb-3">
            <label class="form-label fw-semibold">Conteneur</label>
            <div>{{ selectedContainer.name }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Projet Compose</label>
            <div>{{ getComposeInfo(selectedContainer).project || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Service</label>
            <div>{{ getComposeInfo(selectedContainer).service || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Repertoire de travail</label>
            <div class="font-monospace small">{{ getComposeInfo(selectedContainer).workingDir || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Fichiers de configuration</label>
            <div class="font-monospace small">{{ getComposeInfo(selectedContainer).configFiles || '-' }}</div>
          </div>
          <div v-if="Object.keys(selectedContainer.labels || {}).length > 0" class="mb-3">
            <label class="form-label fw-semibold">Labels</label>
            <div class="border rounded p-2 bg-dark small font-monospace" style="max-height: 200px; overflow-y: auto;">
              <div v-for="(value, key) in selectedContainer.labels" :key="key" class="mb-1">
                <span class="text-muted">{{ key }}:</span> <span class="text-light">{{ value }}</span>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" class="btn" @click="selectedContainer = null">Fermer</button>
        </div>
      </div>
    </div>
  </div>
  <div v-if="selectedContainer" class="modal-backdrop fade show"></div>

  <!-- Modal Inspection (env vars / volumes / networks) -->
  <div v-if="inspectTarget" class="modal modal-blur fade show" style="display: block;" @click.self="inspectTarget = null">
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <div>
            <h5 class="modal-title">{{ inspectTarget.name }}</h5>
            <div class="text-secondary small">
              {{ inspectTarget.image }}:<code>{{ containerVersion(inspectTarget)?.running_version || inspectTarget.image_tag }}</code>
              <span class="ms-2" :class="stateClass(inspectTarget.state)">{{ inspectTarget.state }}</span>
            </div>
          </div>
          <button type="button" class="btn-close" @click="inspectTarget = null" aria-label="Fermer"></button>
        </div>
        <div class="modal-body p-0">
          <div class="border-bottom px-3">
            <ul class="nav nav-tabs nav-tabs-alt">
              <li class="nav-item">
                <a class="nav-link" :class="{ active: inspectTab === 'env' }" href="#" @click.prevent="inspectTab = 'env'">
                  Env Vars
                  <span class="badge bg-azure-lt text-azure ms-1">{{ Object.keys(inspectTarget.env_vars || {}).length }}</span>
                </a>
              </li>
              <li class="nav-item">
                <a class="nav-link" :class="{ active: inspectTab === 'volumes' }" href="#" @click.prevent="inspectTab = 'volumes'">
                  Volumes
                  <span class="badge bg-azure-lt text-azure ms-1">{{ (inspectTarget.volumes || []).length }}</span>
                </a>
              </li>
              <li class="nav-item">
                <a class="nav-link" :class="{ active: inspectTab === 'networks' }" href="#" @click.prevent="inspectTab = 'networks'">
                  Réseaux
                  <span class="badge bg-azure-lt text-azure ms-1">{{ (inspectTarget.networks || []).length }}</span>
                </a>
              </li>
            </ul>
          </div>
          <div class="p-3" style="min-height: 200px; max-height: 400px; overflow-y: auto;">
            <div v-if="inspectTab === 'env'">
              <div v-if="Object.keys(inspectTarget.env_vars || {}).length === 0" class="text-secondary text-center py-3">
                Aucune variable d'environnement (non sensible) disponible
              </div>
              <table v-else class="table table-sm table-vcenter">
                <thead><tr><th>Variable</th><th>Valeur</th></tr></thead>
                <tbody>
                  <tr v-for="(val, key) in inspectTarget.env_vars" :key="key">
                    <td class="font-monospace small fw-semibold">{{ key }}</td>
                    <td class="font-monospace small text-secondary" style="word-break: break-all;">{{ val }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="inspectTab === 'volumes'">
              <div v-if="!(inspectTarget.volumes || []).length" class="text-secondary text-center py-3">Aucun volume monté</div>
              <ul v-else class="list-unstyled mb-0">
                <li v-for="vol in inspectTarget.volumes" :key="vol" class="py-1 border-bottom font-monospace small">{{ vol }}</li>
              </ul>
            </div>
            <div v-if="inspectTab === 'networks'">
              <div v-if="!(inspectTarget.networks || []).length" class="text-secondary text-center py-3">Aucun réseau connecté</div>
              <div v-else class="d-flex flex-wrap gap-2 pt-1">
                <span v-for="net in inspectTarget.networks" :key="net" class="badge bg-blue-lt text-blue fs-6">{{ net }}</span>
              </div>
              <div v-if="inspectTarget?.net_rx_bytes > 0 || inspectTarget?.net_tx_bytes > 0" class="mt-3 border-top pt-3">
                <div class="text-secondary small fw-semibold mb-1">I/O réseau (cumulatif)</div>
                <div class="row row-sm">
                  <div class="col-6">
                    <div class="text-muted small">↓ Reçu</div>
                    <div class="fw-semibold text-info">{{ formatBytes(inspectTarget.net_rx_bytes) }}</div>
                  </div>
                  <div class="col-6">
                    <div class="text-muted small">↑ Envoyé</div>
                    <div class="fw-semibold text-warning">{{ formatBytes(inspectTarget.net_tx_bytes) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" class="btn" @click="inspectTarget = null">Fermer</button>
        </div>
      </div>
    </div>
  </div>
  <div v-if="inspectTarget" class="modal-backdrop fade show"></div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  containers: { type: Array, default: () => [] },
  versionComparisons: { type: Array, default: () => [] },
  canRunDocker: { type: Boolean, default: false },
  actionLoading: { type: Object, default: () => ({}) },
})

// Map host_id+image → comparison for quick lookup
const versionMap = computed(() => {
  const m = {}
  for (const vc of props.versionComparisons) {
    m[vc.host_id + '|' + vc.docker_image] = vc
  }
  return m
})

function containerVersion(c) {
  return versionMap.value[c.host_id + '|' + c.image] ||
         versionMap.value[c.host_id + '|' + c.image + ':' + c.image_tag] ||
         null
}

defineEmits(['container-action'])

const searchInput = ref('')
const search = ref('')
let searchDebounce = null
watch(searchInput, val => {
  clearTimeout(searchDebounce)
  searchDebounce = setTimeout(() => { search.value = val }, 300)
})
const stateFilter = ref('')
const hostFilter = ref('')
const composeFilter = ref('')
const inspectTarget = ref(null)
const inspectTab = ref('env')
const selectedContainer = ref(null)

function getComposeInfo(container) {
  if (!container.labels) return {}
  return {
    project: container.labels['com.docker.compose.project'] || '',
    service: container.labels['com.docker.compose.service'] || '',
    workingDir: container.labels['com.docker.compose.project.working_dir'] || '',
    configFiles: container.labels['com.docker.compose.project.config_files'] || '',
  }
}

function isComposeContainer(container) {
  return !!container.labels?.['com.docker.compose.project']
}

function stateClass(state) {
  const map = {
    running:    'badge bg-green-lt text-green',
    restarting: 'badge bg-yellow-lt text-yellow',
    paused:     'badge bg-yellow-lt text-yellow',
    created:    'badge bg-blue-lt text-blue',
    exited:     'badge bg-secondary-lt text-secondary',
    dead:       'badge bg-red-lt text-red',
    removing:   'badge bg-orange-lt text-orange',
  }
  return map[state] || 'badge bg-secondary-lt text-secondary'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatContainerPorts(raw) {
  if (!raw) return '-'
  const hostPorts = new Set()
  const matches = raw.matchAll(/(\d+\.\d+\.\d+\.\d+|:::?):(\d+)->/g)
  for (const m of matches) hostPorts.add(m[2])
  return hostPorts.size > 0 ? [...hostPorts].join(', ') : raw.split(',').slice(0, 2).join(', ')
}

const filteredContainers = computed(() => {
  return props.containers.filter(c => {
    const matchSearch = !search.value ||
      c.name?.toLowerCase().includes(search.value.toLowerCase()) ||
      c.image?.toLowerCase().includes(search.value.toLowerCase()) ||
      getComposeInfo(c).project?.toLowerCase().includes(search.value.toLowerCase())
    const matchState = !stateFilter.value || c.state === stateFilter.value
    const matchCompose = !composeFilter.value ||
      (composeFilter.value === 'compose' && isComposeContainer(c)) ||
      (composeFilter.value === 'standalone' && !isComposeContainer(c))
    const matchHost = !hostFilter.value || c.hostname === hostFilter.value
    return matchSearch && matchState && matchCompose && matchHost
  })
})

const uniqueHosts = computed(() => {
  const seen = new Set()
  return props.containers
    .filter(c => { if (seen.has(c.hostname)) return false; seen.add(c.hostname); return true })
    .map(c => c.hostname)
    .sort()
})
</script>

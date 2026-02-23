<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Docker</h2>
      <div class="text-secondary">Vue globale de tous les conteneurs sur l'infrastructure</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'containers' }" href="#" @click.prevent="activeTab = 'containers'">
          Conteneurs
          <span class="badge bg-secondary ms-1">{{ containers?.length || 0 }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'compose' }" href="#" @click.prevent="activeTab = 'compose'">
          Projets Compose
          <span class="badge bg-secondary ms-1">{{ composeProjects?.length || 0 }}</span>
        </a>
      </li>
    </ul>

    <!-- ===== TAB CONTENEURS ===== -->
    <div v-if="activeTab === 'containers'">
      <div class="card mb-4">
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6 col-lg-3">
              <input v-model="search" type="text" class="form-control" placeholder="Rechercher un conteneur..." />
            </div>
            <div class="col-md-6 col-lg-3">
              <select v-model="stateFilter" class="form-select">
                <option value="">Tous les etats</option>
                <option value="running">Running</option>
                <option value="exited">Exited</option>
                <option value="paused">Paused</option>
              </select>
            </div>
            <div class="col-md-6 col-lg-3">
              <select v-model="composeFilter" class="form-select">
                <option value="">Tous les conteneurs</option>
                <option value="compose">Docker Compose uniquement</option>
                <option value="standalone">Standalone uniquement</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Nom</th>
                <th>Hote</th>
                <th>Compose</th>
                <th>Image</th>
                <th>Tag</th>
                <th>Etat</th>
                <th>Status</th>
                <th>Ports</th>
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
                <td>{{ c.image }}</td>
                <td><code>{{ c.image_tag }}</code></td>
                <td>
                  <span :class="c.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                    {{ c.state }}
                  </span>
                </td>
                <td class="text-secondary small">{{ c.status }}</td>
                <td class="text-secondary small font-monospace">{{ c.ports || '-' }}</td>
                <td class="text-end">
                  <button v-if="getComposeInfo(c).project" @click="showComposeDetails(c)" class="btn btn-sm btn-ghost-secondary">
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                      <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                      <path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0" />
                      <path d="M12 10m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0" />
                      <path d="M12 10l0 5" />
                    </svg>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="filteredContainers.length === 0" class="text-center text-secondary py-4">
          Aucun conteneur trouve
        </div>
      </div>
    </div>

    <!-- ===== TAB PROJETS COMPOSE ===== -->
    <div v-if="activeTab === 'compose'">
      <div class="card mb-4">
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6 col-lg-4">
              <input v-model="composeSearch" type="text" class="form-control" placeholder="Rechercher un projet..." />
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Projet</th>
                <th>Hote</th>
                <th>Services</th>
                <th>Fichier de config</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in filteredComposeProjects" :key="p.id">
                <td class="fw-semibold">{{ p.name }}</td>
                <td>
                  <router-link :to="`/hosts/${p.host_id}`" class="text-decoration-none">
                    {{ p.hostname }}
                  </router-link>
                </td>
                <td>
                  <div class="d-flex flex-wrap gap-1">
                    <span v-for="svc in p.services" :key="svc" class="badge bg-blue-lt text-blue">
                      {{ svc }}
                    </span>
                    <span v-if="!p.services || p.services.length === 0" class="text-secondary">-</span>
                  </div>
                </td>
                <td class="font-monospace small text-secondary">{{ p.config_file || p.working_dir || '-' }}</td>
                <td class="text-end">
                  <button @click="selectedProject = p" class="btn btn-sm btn-ghost-secondary">
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                      <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                      <path d="M14 3v4a1 1 0 0 0 1 1h4" />
                      <path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z" />
                      <path d="M9 9l1 0" />
                      <path d="M9 13l6 0" />
                      <path d="M9 17l6 0" />
                    </svg>
                    Config
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="filteredComposeProjects.length === 0" class="text-center text-secondary py-4">
          Aucun projet Compose trouvé
        </div>
      </div>
    </div>

    <!-- Modal conteneur (labels) -->
    <div v-if="selectedContainer" class="modal modal-blur fade show" style="display: block;" @click.self="selectedContainer = null">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Details Docker Compose</h5>
            <button type="button" class="btn-close" @click="selectedContainer = null"></button>
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

    <!-- Modal projet compose (raw config) -->
    <div v-if="selectedProject" class="modal modal-blur fade show" style="display: block;" @click.self="selectedProject = null">
      <div class="modal-dialog modal-xl modal-dialog-centered modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">{{ selectedProject.name }}</h5>
              <div class="text-secondary small font-monospace mt-1">
                {{ selectedProject.config_file || selectedProject.working_dir || '-' }}
              </div>
            </div>
            <button type="button" class="btn-close" @click="selectedProject = null"></button>
          </div>
          <div class="modal-body p-0">
            <div class="row g-0">
              <!-- Infos projet -->
              <div class="col-md-3 border-end p-3">
                <div class="mb-3">
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Hote</div>
                  <div>{{ selectedProject.hostname }}</div>
                </div>
                <div class="mb-3">
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Repertoire</div>
                  <div class="font-monospace small text-break">{{ selectedProject.working_dir || '-' }}</div>
                </div>
                <div class="mb-3">
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Fichier</div>
                  <div class="font-monospace small text-break">{{ selectedProject.config_file || '-' }}</div>
                </div>
                <div>
                  <div class="text-secondary small fw-semibold text-uppercase mb-1">Services ({{ (selectedProject.services || []).length }})</div>
                  <div class="d-flex flex-wrap gap-1">
                    <span v-for="svc in selectedProject.services" :key="svc" class="badge bg-blue-lt text-blue">{{ svc }}</span>
                    <span v-if="!selectedProject.services || selectedProject.services.length === 0" class="text-secondary small">-</span>
                  </div>
                </div>
              </div>
              <!-- Raw config YAML -->
              <div class="col-md-9">
                <div class="d-flex align-items-center justify-content-between px-3 pt-3 pb-2 border-bottom">
                  <span class="text-secondary small fw-semibold">docker compose config (résolu)</span>
                  <button class="btn btn-sm btn-ghost-secondary" @click="copyConfig(selectedProject.raw_config)">
                    {{ copied ? '✓ Copié' : 'Copier' }}
                  </button>
                </div>
                <pre v-if="selectedProject.raw_config" class="m-0 p-3 small" style="max-height: 60vh; overflow-y: auto; background: #1e1e2e; color: #cdd6f4; border-radius: 0 0 4px 0;">{{ selectedProject.raw_config }}</pre>
                <div v-else class="p-4 text-secondary text-center">
                  Config non disponible (agent trop ancien ou docker compose introuvable)
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn" @click="selectedProject = null">Fermer</button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="selectedProject" class="modal-backdrop fade show"></div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'

const containers = ref([])
const composeProjects = ref([])
const search = ref('')
const stateFilter = ref('')
const composeFilter = ref('')
const composeSearch = ref('')
const selectedContainer = ref(null)
const selectedProject = ref(null)
const activeTab = ref('containers')
const copied = ref(false)

const getComposeInfo = (container) => {
  if (!container.labels) return {}
  return {
    project: container.labels['com.docker.compose.project'] || '',
    service: container.labels['com.docker.compose.service'] || '',
    workingDir: container.labels['com.docker.compose.project.working_dir'] || '',
    configFiles: container.labels['com.docker.compose.project.config_files'] || ''
  }
}

const isComposeContainer = (container) => {
  return !!container.labels?.['com.docker.compose.project']
}

const showComposeDetails = (container) => {
  selectedContainer.value = container
}

const copyConfig = async (text) => {
  if (!text) return
  await navigator.clipboard.writeText(text)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

const filteredContainers = computed(() => {
  return containers.value.filter(c => {
    const matchSearch = !search.value ||
      c.name?.toLowerCase().includes(search.value.toLowerCase()) ||
      c.image?.toLowerCase().includes(search.value.toLowerCase()) ||
      getComposeInfo(c).project?.toLowerCase().includes(search.value.toLowerCase())
    const matchState = !stateFilter.value || c.state === stateFilter.value
    const matchCompose = !composeFilter.value ||
      (composeFilter.value === 'compose' && isComposeContainer(c)) ||
      (composeFilter.value === 'standalone' && !isComposeContainer(c))
    return matchSearch && matchState && matchCompose
  })
})

const filteredComposeProjects = computed(() => {
  if (!composeSearch.value) return composeProjects.value
  const q = composeSearch.value.toLowerCase()
  return composeProjects.value.filter(p =>
    p.name?.toLowerCase().includes(q) ||
    p.hostname?.toLowerCase().includes(q) ||
    p.config_file?.toLowerCase().includes(q) ||
    p.working_dir?.toLowerCase().includes(q)
  )
})

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/docker', (payload) => {
  if (payload.type !== 'docker') return
  containers.value = payload.containers || []
  composeProjects.value = payload.compose_projects || []
})
</script>

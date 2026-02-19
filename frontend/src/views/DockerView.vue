<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Docker - Conteneurs</h2>
      <div class="text-secondary">Vue globale de tous les conteneurs sur l'infrastructure</div>
    </div>

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

    <!-- Modal pour les détails Compose -->
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
              <label class="form-label fw-semibold">Tous les labels</label>
              <div class="border rounded p-2 bg-light small font-monospace" style="max-height: 200px; overflow-y: auto;">
                <div v-for="(value, key) in selectedContainer.labels" :key="key" class="mb-1">
                  <span class="text-secondary">{{ key }}:</span> {{ value }}
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import apiClient from '../api'

const containers = ref([])
const search = ref('')
const stateFilter = ref('')
const composeFilter = ref('')
const selectedContainer = ref(null)

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

onMounted(async () => {
  try {
    const res = await apiClient.getAllContainers()
    containers.value = res.data
  } catch (e) {
    console.error('Failed to fetch containers:', e)
  }
})
</script>

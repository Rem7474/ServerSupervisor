<template>
  <!-- Filters -->
  <div class="card mb-4">
    <div class="card-body">
      <div class="row g-3">
        <div class="col-6 col-md-6 col-lg-3">
          <input v-model="composeSearch" type="text" class="form-control" placeholder="Rechercher un projet..." />
        </div>
        <div class="col-6 col-md-6 col-lg-3">
          <select v-model="composeHostFilter" class="form-select">
            <option value="">Tous les hôtes</option>
            <option v-for="h in uniqueHosts" :key="h" :value="h">{{ h }}</option>
          </select>
        </div>
        <div class="col-6 col-md-6 col-lg-3">
          <select v-model="composeStateFilter" class="form-select">
            <option value="">Tous les états</option>
            <option value="running">En cours</option>
            <option value="stopped">Arrêté</option>
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
            <th>Projet</th>
            <th>Hôte</th>
            <th>État</th>
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
              <span :class="getComposeStatus(p) === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                {{ getComposeStatus(p) === 'running' ? 'En cours' : 'Arrêté' }}
              </span>
              <span
                v-if="getComposeUpdates(p).length > 0"
                class="badge bg-yellow-lt text-yellow ms-1"
                :title="getComposeUpdates(p).map(v => `${v.docker_image} : ${v.latest_version} dispo`).join('\n')"
              >
                {{ getComposeUpdates(p).length }} MAJ
              </span>
            </td>
            <td>
              <div class="d-flex flex-wrap gap-1">
                <span v-for="svc in p.services" :key="svc" class="badge bg-blue-lt text-blue">{{ svc }}</span>
                <span v-if="!p.services || p.services.length === 0" class="text-secondary">-</span>
              </div>
            </td>
            <td class="font-monospace small text-secondary">{{ p.config_file || p.working_dir || '-' }}</td>
            <td class="text-end">
              <div class="d-flex align-items-center justify-content-end gap-1">
                <template v-if="canRunDocker">
                  <button
                    v-if="getComposeStatus(p) === 'stopped'"
                    @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_up', workingDir: p.working_dir || '' })"
                    :disabled="!!actionLoading[p.name]"
                    class="btn btn-sm btn-ghost-success"
                    title="Start (up -d)"
                  >
                    <span v-if="actionLoading[p.name] === 'compose_up'" class="spinner-border spinner-border-sm"></span>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M7 4v16l13 -8z" /></svg>
                  </button>
                  <template v-if="getComposeStatus(p) === 'running'">
                    <button
                      @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_down', workingDir: p.working_dir || '' })"
                      :disabled="!!actionLoading[p.name]"
                      class="btn btn-sm btn-ghost-danger"
                      title="Stop (down)"
                    >
                      <span v-if="actionLoading[p.name] === 'compose_down'" class="spinner-border spinner-border-sm"></span>
                      <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><rect x="4" y="4" width="16" height="16" rx="2" /></svg>
                    </button>
                    <button
                      @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_restart', workingDir: p.working_dir || '' })"
                      :disabled="!!actionLoading[p.name]"
                      class="btn btn-sm btn-ghost-warning"
                      title="Redémarrer"
                    >
                      <span v-if="actionLoading[p.name] === 'compose_restart'" class="spinner-border spinner-border-sm"></span>
                      <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4" /><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" /></svg>
                    </button>
                  </template>
                  <button
                    @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_logs', workingDir: p.working_dir || '' })"
                    :disabled="!!actionLoading[p.name]"
                    class="btn btn-sm btn-ghost-secondary"
                    title="Voir les logs"
                  >
                    <span v-if="actionLoading[p.name] === 'compose_logs'" class="spinner-border spinner-border-sm"></span>
                    <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                  </button>
                </template>
                <button @click="selectedProject = p" class="btn btn-sm btn-ghost-secondary" title="Config">
                  <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                    <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                    <path d="M14 3v4a1 1 0 0 0 1 1h4" />
                    <path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z" />
                    <path d="M9 9l1 0" /><path d="M9 13l6 0" /><path d="M9 17l6 0" />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="filteredComposeProjects.length === 0" class="text-center text-secondary py-4">
      Aucun projet Compose trouvé
    </div>
  </div>

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
          <button type="button" class="btn-close" @click="selectedProject = null" aria-label="Fermer"></button>
        </div>
        <div class="modal-body p-0">
          <div class="row g-0">
            <div class="col-md-3 border-end p-3">
              <div class="mb-3">
                <div class="text-secondary small fw-semibold text-uppercase mb-1">Hôte</div>
                <div>{{ selectedProject.hostname }}</div>
              </div>
              <div class="mb-3">
                <div class="text-secondary small fw-semibold text-uppercase mb-1">Répertoire</div>
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
            <div class="col-md-9">
              <div class="d-flex align-items-center justify-content-between px-3 pt-3 pb-2 border-bottom">
                <span class="text-secondary small fw-semibold">docker compose config (résolu)</span>
                <button
                  :class="['btn', 'btn-sm', copied ? 'btn-success' : 'btn-ghost-secondary']"
                  @click="copyConfig(selectedProject.raw_config)">
                  {{ copied ? '✓ Copié' : 'Copier' }}
                </button>
              </div>
              <pre v-if="selectedProject.raw_config" class="m-0 p-3 small" style="max-height: 60vh; overflow-y: auto; background: #0f172a; color: #e2e8f0; border-radius: 0 0 4px 0;">{{ selectedProject.raw_config }}</pre>
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
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  composeProjects: { type: Array, default: () => [] },
  containers: { type: Array, default: () => [] },
  versionComparisons: { type: Array, default: () => [] },
  canRunDocker: { type: Boolean, default: false },
  actionLoading: { type: Object, default: () => ({}) },
})

defineEmits(['compose-action'])

const composeSearch = ref('')
const composeHostFilter = ref('')
const composeStateFilter = ref('')
const selectedProject = ref(null)
const copied = ref(false)

const composeProjectStatus = computed(() => {
  const statusMap = {}
  for (const project of props.composeProjects) {
    const projectContainers = props.containers.filter(
      c => c.labels?.['com.docker.compose.project'] === project.name &&
           c.host_id === project.host_id
    )
    statusMap[`${project.host_id}:${project.name}`] =
      projectContainers.some(c => c.state === 'running') ? 'running' : 'stopped'
  }
  return statusMap
})

function getComposeStatus(project) {
  return composeProjectStatus.value[`${project.host_id}:${project.name}`] || 'stopped'
}

const vcByImage = computed(() => {
  const m = {}
  for (const vc of props.versionComparisons) {
    m[`${vc.host_id}|${vc.docker_image}`] = vc
  }
  return m
})

function getComposeUpdates(project) {
  const projectContainers = props.containers.filter(
    c => c.labels?.['com.docker.compose.project'] === project.name && c.host_id === project.host_id
  )
  const updates = []
  for (const c of projectContainers) {
    const vc = vcByImage.value[`${c.host_id}|${c.image}`] ||
               vcByImage.value[`${c.host_id}|${c.image}:${c.image_tag}`]
    if (vc && !vc.is_up_to_date && vc.running_version) {
      updates.push(vc)
    }
  }
  return updates
}

const uniqueHosts = computed(() => {
  const seen = new Set()
  return props.composeProjects
    .filter(p => { if (seen.has(p.hostname)) return false; seen.add(p.hostname); return true })
    .map(p => p.hostname)
    .sort()
})

const filteredComposeProjects = computed(() => {
  return props.composeProjects.filter(p => {
    if (composeSearch.value) {
      const q = composeSearch.value.toLowerCase()
      const match = p.name?.toLowerCase().includes(q) ||
        p.hostname?.toLowerCase().includes(q) ||
        p.config_file?.toLowerCase().includes(q) ||
        p.working_dir?.toLowerCase().includes(q)
      if (!match) return false
    }
    if (composeHostFilter.value && p.hostname !== composeHostFilter.value) return false
    if (composeStateFilter.value && getComposeStatus(p) !== composeStateFilter.value) return false
    return true
  })
})

async function copyConfig(text) {
  if (!text) return
  await navigator.clipboard.writeText(text)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}
</script>

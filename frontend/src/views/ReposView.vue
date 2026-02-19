<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Versions & Repos suivis</h2>
      <div class="text-secondary">Suivre les releases GitHub et comparer avec les images Docker</div>
    </div>

    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Ajouter un repo GitHub</h3>
      </div>
      <div class="card-body">
        <form @submit.prevent="addRepo" class="row g-3 align-items-end">
          <div class="col-md-3">
            <label class="form-label">Owner (ex: home-assistant)</label>
            <input v-model="newRepo.owner" type="text" class="form-control" required placeholder="home-assistant" />
          </div>
          <div class="col-md-3">
            <label class="form-label">Repo (ex: core)</label>
            <input v-model="newRepo.repo" type="text" class="form-control" required placeholder="core" />
          </div>
          <div class="col-md-3">
            <label class="form-label">Nom affiche (optionnel)</label>
            <input v-model="newRepo.display_name" type="text" class="form-control" placeholder="Home Assistant" />
          </div>
          <div class="col-md-3">
            <label class="form-label">Image Docker associee (optionnel)</label>
            <input v-model="newRepo.docker_image" type="text" class="form-control" placeholder="homeassistant/home-assistant" />
          </div>
          <div class="col-12">
            <button type="submit" class="btn btn-primary">Ajouter</button>
          </div>
        </form>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Repos suivis ({{ repos.length }})</h3>
      </div>
      <div v-if="repos.length === 0" class="text-center text-secondary py-4">
        Aucun repo suivi. Ajoutez un repo GitHub ci-dessus.
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Repo</th>
              <th>Docker</th>
              <th>Derniere version</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="repo in repos" :key="repo.id">
              <td>
                <div class="fw-semibold">{{ repo.display_name || `${repo.owner}/${repo.repo}` }}</div>
                <div class="text-secondary small">
                  <a :href="`https://github.com/${repo.owner}/${repo.repo}`" target="_blank" class="link-primary">
                    {{ repo.owner }}/{{ repo.repo }}
                  </a>
                </div>
              </td>
              <td class="text-secondary small">
                {{ repo.docker_image || '-' }}
              </td>
              <td>
                <div v-if="repo.latest_version" class="fw-semibold text-green">{{ repo.latest_version }}</div>
                <div v-else class="text-secondary">En attente...</div>
                <div class="text-secondary small">{{ repo.latest_version ? formatDate(repo.latest_date) : '' }}</div>
              </td>
              <td class="text-end">
                <a v-if="repo.release_url" :href="repo.release_url" target="_blank" class="btn btn-outline-secondary btn-sm">
                  Voir
                </a>
                <button @click="deleteRepo(repo.id)" class="btn btn-outline-danger btn-sm ms-2">
                  Supprimer
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <h3 class="card-title">Comparaison des versions</h3>
      </div>
      <div class="card-body text-secondary">
        Compare les images Docker en cours d'execution avec les dernieres releases GitHub.
      </div>

      <div v-if="comparisons.length === 0" class="text-center text-secondary py-4">
        Aucune comparaison disponible. Associez un nom d'image Docker a un repo suivi.
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Image Docker</th>
              <th>Hote</th>
              <th>Version en cours</th>
              <th>Derniere version</th>
              <th>Statut</th>
              <th>Lien</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in comparisons" :key="v.docker_image + v.host_id">
              <td class="fw-semibold">{{ v.docker_image }}</td>
              <td class="text-secondary">{{ v.hostname }}</td>
              <td><code>{{ v.running_version }}</code></td>
              <td>
                <span :class="v.is_up_to_date ? 'badge bg-green-lt text-green badge-soft' : 'badge bg-yellow-lt text-yellow badge-soft'">
                  {{ v.latest_version }}
                </span>
              </td>
              <td>
                <span v-if="v.is_up_to_date" class="badge bg-green-lt text-green">A jour</span>
                <span v-else class="badge bg-yellow-lt text-yellow">Mise a jour dispo</span>
              </td>
              <td>
                <a v-if="v.release_url" :href="v.release_url" target="_blank" class="link-primary">
                  GitHub
                </a>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import apiClient from '../api'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.locale('fr')

const repos = ref([])
const comparisons = ref([])
const newRepo = ref({ owner: '', repo: '', display_name: '', docker_image: '' })

async function fetchData() {
  try {
    const [reposRes, compRes] = await Promise.all([
      apiClient.getTrackedRepos(),
      apiClient.getVersionComparisons().catch(() => ({ data: [] })),
    ])
    repos.value = reposRes.data
    comparisons.value = compRes.data || []
  } catch (e) {
    console.error('Failed to fetch repos:', e)
  }
}

async function addRepo() {
  try {
    await apiClient.addTrackedRepo(newRepo.value)
    newRepo.value = { owner: '', repo: '', display_name: '', docker_image: '' }
    await fetchData()
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

async function deleteRepo(id) {
  if (!confirm('Supprimer ce repo suivi ?')) return
  try {
    await apiClient.deleteTrackedRepo(id)
    await fetchData()
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return ''
  return dayjs(date).fromNow()
}

onMounted(fetchData)
</script>

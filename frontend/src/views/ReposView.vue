<template>
  <div>
    <h1 class="text-2xl font-bold mb-2">Versions & Repos suivis</h1>
    <p class="text-gray-400 mb-8">Suivre les dernières releases GitHub et comparer avec les images Docker en cours</p>

    <!-- Add Repo Form -->
    <div class="card mb-8">
      <h3 class="text-lg font-semibold mb-4">Ajouter un repo GitHub</h3>
      <form @submit.prevent="addRepo" class="flex gap-4 flex-wrap items-end">
        <div class="flex-1 min-w-[200px]">
          <label class="block text-sm text-gray-400 mb-1">Owner (ex: home-assistant)</label>
          <input v-model="newRepo.owner" type="text" class="input-field" required placeholder="home-assistant" />
        </div>
        <div class="flex-1 min-w-[200px]">
          <label class="block text-sm text-gray-400 mb-1">Repo (ex: core)</label>
          <input v-model="newRepo.repo" type="text" class="input-field" required placeholder="core" />
        </div>
        <div class="flex-1 min-w-[200px]">
          <label class="block text-sm text-gray-400 mb-1">Nom affiché (optionnel)</label>
          <input v-model="newRepo.display_name" type="text" class="input-field" placeholder="Home Assistant" />
        </div>
        <div class="flex-1 min-w-[250px]">
          <label class="block text-sm text-gray-400 mb-1">Image Docker associée (optionnel)</label>
          <input v-model="newRepo.docker_image" type="text" class="input-field" placeholder="homeassistant/home-assistant" />
        </div>
        <button type="submit" class="btn-primary">Ajouter</button>
      </form>
    </div>

    <!-- Tracked Repos -->
    <div class="card mb-8">
      <h3 class="text-lg font-semibold mb-4">Repos suivis ({{ repos.length }})</h3>
      <div v-if="repos.length === 0" class="text-gray-400 text-center py-8">
        Aucun repo suivi. Ajoutez un repo GitHub ci-dessus.
      </div>
      <div class="space-y-3">
        <div v-for="repo in repos" :key="repo.id" class="bg-dark-900 rounded-lg p-4 flex items-center gap-4">
          <div class="flex-1">
            <div class="font-semibold">{{ repo.display_name || `${repo.owner}/${repo.repo}` }}</div>
            <div class="text-sm text-gray-400">
              <a :href="`https://github.com/${repo.owner}/${repo.repo}`" target="_blank" class="text-primary-400 hover:underline">
                {{ repo.owner }}/{{ repo.repo }}
              </a>
              <span v-if="repo.docker_image" class="ml-3">
                🐳 {{ repo.docker_image }}
              </span>
            </div>
          </div>
          <div class="text-center">
            <div v-if="repo.latest_version" class="font-medium text-emerald-400">{{ repo.latest_version }}</div>
            <div v-else class="text-gray-500">En attente...</div>
            <div class="text-xs text-gray-500">{{ repo.latest_version ? formatDate(repo.latest_date) : '' }}</div>
          </div>
          <a v-if="repo.release_url" :href="repo.release_url" target="_blank" class="btn-secondary text-sm">
            Voir
          </a>
          <button @click="deleteRepo(repo.id)" class="text-red-400 hover:text-red-300 p-2">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Version Comparisons -->
    <div class="card">
      <h3 class="text-lg font-semibold mb-4">Comparaison des versions</h3>
      <p class="text-gray-400 text-sm mb-4">Compare les images Docker en cours d'exécution avec les dernières releases GitHub</p>

      <div v-if="comparisons.length === 0" class="text-gray-400 text-center py-8">
        Aucune comparaison disponible. Associez un nom d'image Docker à un repo suivi.
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-dark-700">
              <th class="text-left py-3 px-4">Image Docker</th>
              <th class="text-left py-3 px-4">Hôte</th>
              <th class="text-left py-3 px-4">Version en cours</th>
              <th class="text-left py-3 px-4">Dernière version</th>
              <th class="text-left py-3 px-4">Statut</th>
              <th class="text-left py-3 px-4">Lien</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in comparisons" :key="v.docker_image + v.host_id" class="border-b border-dark-700/50">
              <td class="py-3 px-4 font-medium">{{ v.docker_image }}</td>
              <td class="py-3 px-4 text-gray-400">{{ v.hostname }}</td>
              <td class="py-3 px-4">
                <code class="text-xs bg-dark-700 px-2 py-1 rounded">{{ v.running_version }}</code>
              </td>
              <td class="py-3 px-4">
                <code class="text-xs px-2 py-1 rounded" :class="v.is_up_to_date ? 'bg-emerald-500/20 text-emerald-400' : 'bg-yellow-500/20 text-yellow-400'">
                  {{ v.latest_version }}
                </code>
              </td>
              <td class="py-3 px-4">
                <span v-if="v.is_up_to_date" class="badge-online">À jour</span>
                <span v-else class="badge-warning">⬆ Mise à jour dispo</span>
              </td>
              <td class="py-3 px-4">
                <a v-if="v.release_url" :href="v.release_url" target="_blank" class="text-primary-400 hover:underline text-xs">
                  GitHub →
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

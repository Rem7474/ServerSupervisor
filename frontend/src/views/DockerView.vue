<template>
  <div>
    <h1 class="text-2xl font-bold mb-2">Docker - Conteneurs</h1>
    <p class="text-gray-400 mb-8">Vue globale de tous les conteneurs sur l'infrastructure</p>

    <!-- Filters -->
    <div class="flex gap-4 mb-6">
      <input v-model="search" type="text" class="input-field max-w-xs" placeholder="Rechercher un conteneur..." />
      <select v-model="stateFilter" class="input-field max-w-[200px]">
        <option value="">Tous les états</option>
        <option value="running">Running</option>
        <option value="exited">Exited</option>
        <option value="paused">Paused</option>
      </select>
    </div>

    <!-- Containers Table -->
    <div class="card">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-dark-700">
              <th class="text-left py-3 px-4">Nom</th>
              <th class="text-left py-3 px-4">Hôte</th>
              <th class="text-left py-3 px-4">Image</th>
              <th class="text-left py-3 px-4">Tag</th>
              <th class="text-left py-3 px-4">État</th>
              <th class="text-left py-3 px-4">Status</th>
              <th class="text-left py-3 px-4">Ports</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in filteredContainers" :key="c.id" class="border-b border-dark-700/50 hover:bg-dark-800/50">
              <td class="py-3 px-4 font-medium">{{ c.name }}</td>
              <td class="py-3 px-4 text-gray-400">{{ c.host_id?.substring(0, 8) }}</td>
              <td class="py-3 px-4 text-gray-300">{{ c.image }}</td>
              <td class="py-3 px-4"><code class="text-xs bg-dark-700 px-2 py-1 rounded">{{ c.image_tag }}</code></td>
              <td class="py-3 px-4">
                <span :class="c.state === 'running' ? 'badge-running' : 'badge-stopped'">{{ c.state }}</span>
              </td>
              <td class="py-3 px-4 text-gray-400 text-xs">{{ c.status }}</td>
              <td class="py-3 px-4 text-gray-400 text-xs font-mono">{{ c.ports || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="filteredContainers.length === 0" class="text-center py-8 text-gray-400">
        Aucun conteneur trouvé
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import apiClient from '../api'

const containers = ref([])
const search = ref('')
const stateFilter = ref('')

const filteredContainers = computed(() => {
  return containers.value.filter(c => {
    const matchSearch = !search.value ||
      c.name?.toLowerCase().includes(search.value.toLowerCase()) ||
      c.image?.toLowerCase().includes(search.value.toLowerCase())
    const matchState = !stateFilter.value || c.state === stateFilter.value
    return matchSearch && matchState
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

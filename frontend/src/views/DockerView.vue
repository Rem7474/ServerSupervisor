<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Docker - Conteneurs</h2>
      <div class="text-secondary">Vue globale de tous les conteneurs sur l'infrastructure</div>
    </div>

    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3">
          <div class="col-md-6 col-lg-4">
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
              <th>Image</th>
              <th>Tag</th>
              <th>Etat</th>
              <th>Status</th>
              <th>Ports</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in filteredContainers" :key="c.id">
              <td class="fw-semibold">{{ c.name }}</td>
              <td class="text-secondary">{{ c.host_id?.substring(0, 8) }}</td>
              <td>{{ c.image }}</td>
              <td><code>{{ c.image_tag }}</code></td>
              <td>
                <span :class="c.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                  {{ c.state }}
                </span>
              </td>
              <td class="text-secondary small">{{ c.status }}</td>
              <td class="text-secondary small font-monospace">{{ c.ports || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="filteredContainers.length === 0" class="text-center text-secondary py-4">
        Aucun conteneur trouve
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

<template>
  <div class="card mb-4">
    <div class="card-header">
      <h3 class="card-title">Conteneurs Docker <span v-if="containers.length">({{ containers.length }})</span></h3>
    </div>
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>Image</th>
            <th>Tag</th>
            <th>Etat</th>
            <th>Status</th>
            <th>Ports</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in containers" :key="c.id">
            <td class="fw-semibold">{{ c.name }}</td>
            <td class="text-secondary">{{ c.image }}</td>
            <td>
              <code>{{ c.image_tag }}</code>
              <template v-if="containerVersion(c)">
                <br>
                <span v-if="containerVersion(c).is_up_to_date" class="badge bg-green-lt text-green mt-1">A jour</span>
                <span v-else-if="!containerVersion(c).running_version" class="badge bg-secondary-lt text-secondary mt-1">Version inconnue</span>
                <span v-else class="badge bg-yellow-lt text-yellow mt-1" :title="`Derniere : ${containerVersion(c).latest_version}`">MAJ dispo</span>
              </template>
            </td>
            <td>
              <span :class="c.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                {{ { running: 'En cours', exited: 'Arrete', paused: 'En pause', created: 'Cree', restarting: 'Redemarrage', dead: 'Mort' }[c.state] || c.state }}
              </span>
            </td>
            <td class="text-secondary small">{{ c.status }}</td>
            <td class="text-secondary small font-monospace">{{ c.ports || '-' }}</td>
          </tr>
          <tr v-if="!containers.length">
            <td colspan="6" class="text-center text-secondary py-4">Aucun conteneur Docker actif sur cet hote.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  containers: {
    type: Array,
    default: () => [],
  },
  versionComparisons: {
    type: Array,
    default: () => [],
  },
})

const versionMap = computed(() => {
  const map = {}
  for (const vc of props.versionComparisons) {
    map[vc.docker_image] = vc
  }
  return map
})

function containerVersion(container) {
  return versionMap.value[container.image] || versionMap.value[`${container.image}:${container.image_tag}`] || null
}
</script>

<template>
  <div class="card mb-4">
    <div class="card-header">
      <h3 class="card-title">
        Conteneurs Docker <span v-if="containers.length">({{ containers.length }})</span>
      </h3>
    </div>
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>Image</th>
            <th>Tag</th>
            <th>Version réelle</th>
            <th>État</th>
            <th>Status</th>
            <th>Port interne</th>
            <th>Port hôte exposé</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="c in containers"
            :key="c.id"
          >
            <td class="fw-semibold">
              {{ c.name }}
            </td>
            <td class="text-secondary">
              {{ c.image }}
            </td>
            <td>
              <code>{{ c.image_tag }}</code>
              <template v-if="containerVersion(c)">
                <br>
                <span
                  v-if="containerVersion(c)?.tracker_id && containerVersion(c)?.custom_task_id && containerVersion(c)?.is_up_to_date"
                  class="badge bg-green-lt text-green mt-1"
                >A jour</span>
                <span
                  v-else-if="containerVersion(c)?.tracker_id && containerVersion(c)?.custom_task_id && !containerVersion(c)?.is_up_to_date && containerVersion(c)?.running_version"
                  class="badge bg-yellow-lt text-yellow mt-1"
                  :title="`Dernière : ${containerVersion(c)?.latest_version}`"
                >MAJ dispo</span>
                <span
                  v-else-if="containerVersion(c)?.tracker_id && !containerVersion(c)?.custom_task_id"
                  class="badge bg-secondary-lt text-secondary mt-1"
                  title="Tracker est configuré mais aucune task n'a été associée"
                >Surveillance seule</span>
                <span
                  v-else-if="!containerVersion(c)?.tracker_id"
                  class="badge bg-secondary-lt text-secondary mt-1"
                >Pas de tracker</span>
                <span
                  v-else
                  class="badge bg-secondary-lt text-secondary mt-1"
                >Version inconnue</span>
              </template>
            </td>
            <td>
              <code v-if="containerVersion(c)?.running_version">
                {{ c.image_tag }} → <strong>{{ containerVersion(c)?.running_version }}</strong>
              </code>
              <code v-else>{{ c.image_tag }}</code>
            </td>
            <td>
              <span :class="c.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                {{ ({ running: 'En cours', exited: 'Arrêté', paused: 'En pause', created: 'Créé', restarting: 'Redémarrage', dead: 'Mort' } as Record<string, string>)[c.state || ''] || c.state }}
              </span>
            </td>
            <td class="text-secondary small">
              {{ c.status }}
            </td>
            <td>
              <DockerPortBadges
                :ports="normalizedPortsForContainer(c)"
                kind="internal"
              />
            </td>
            <td>
              <DockerPortBadges
                :ports="normalizedPortsForContainer(c)"
                kind="exposed"
              />
            </td>
          </tr>
          <tr v-if="!containers.length">
            <td
              colspan="8"
              class="text-center text-secondary py-4"
            >
              Aucun conteneur Docker actif sur cet hôte.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, toRef } from 'vue'
import DockerPortBadges from '../common/DockerPortBadges.vue'
import { useDockerContainerPorts } from '../../composables/useDockerContainerPorts'

interface Container {
  id: string
  name?: string
  image: string
  image_tag?: string
  state?: string
  status?: string
  [key: string]: any
}

interface VersionComparison {
  docker_image: string
  tracker_id?: string
  custom_task_id?: string
  is_up_to_date?: boolean
  running_version?: string
  latest_version?: string
}

const props = withDefaults(defineProps<{
  containers?: Container[]
  versionComparisons?: VersionComparison[]
}>(), {
  containers: () => [],
  versionComparisons: () => [],
})

const { normalizedPortsForContainer } = useDockerContainerPorts(toRef(props, 'containers') as any)

const versionMap = computed<Record<string, VersionComparison>>(() => {
  const map: Record<string, VersionComparison> = {}
  for (const vc of props.versionComparisons) {
    map[vc.docker_image] = vc
  }
  return map
})

function containerVersion(container: Container): VersionComparison | null {
  return versionMap.value[container.image] || versionMap.value[`${container.image}:${container.image_tag}`] || null
}
</script>

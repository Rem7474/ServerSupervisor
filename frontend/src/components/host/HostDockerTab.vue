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
            <th v-if="canRun" />
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
            <td
              v-if="canRun"
              class="text-end text-nowrap"
            >
              <div class="d-flex align-items-center justify-content-end gap-1">
                <button
                  v-if="['exited', 'dead', 'created', 'paused'].includes(c.state || '')"
                  type="button"
                  :disabled="!!actionLoading[containerKey(c)]"
                  class="btn btn-sm btn-success"
                  title="Démarrer"
                  aria-label="Démarrer le conteneur"
                  @click="runAction(c, 'start')"
                >
                  <span
                    v-if="actionLoading[containerKey(c)] === 'start'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconPlayerPlay
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  v-if="c.state === 'running'"
                  type="button"
                  :disabled="!!actionLoading[containerKey(c)]"
                  class="btn btn-sm btn-outline-danger"
                  title="Arrêter"
                  aria-label="Arrêter le conteneur"
                  @click="runAction(c, 'stop')"
                >
                  <span
                    v-if="actionLoading[containerKey(c)] === 'stop'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconPlayerStop
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  v-if="c.state === 'running'"
                  type="button"
                  :disabled="!!actionLoading[containerKey(c)]"
                  class="btn btn-sm btn-outline-warning"
                  title="Redémarrer"
                  aria-label="Redémarrer le conteneur"
                  @click="runAction(c, 'restart')"
                >
                  <span
                    v-if="actionLoading[containerKey(c)] === 'restart'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconRefresh
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  :disabled="!!actionLoading[containerKey(c)]"
                  class="btn btn-sm btn-ghost-secondary"
                  title="Voir les logs"
                  aria-label="Voir les logs du conteneur"
                  @click="runAction(c, 'logs')"
                >
                  <span
                    v-if="actionLoading[containerKey(c)] === 'logs'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconList
                    v-else
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!containers.length">
            <td
              :colspan="canRun ? 9 : 8"
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
import { computed, ref, toRef } from 'vue'
import { IconPlayerPlay, IconPlayerStop, IconRefresh, IconList } from '@tabler/icons-vue'
import DockerPortBadges from '../common/DockerPortBadges.vue'
import { useDockerContainerPorts } from '../../composables/useDockerContainerPorts'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { addToast } from '../../composables/useGlobalToast'
import apiClient, { getApiErrorMessage } from '../../api'

interface Container {
  id: string
  name?: string
  image: string
  image_tag?: string
  state?: string
  status?: string
  [key: string]: unknown
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
  hostId: string | number
  containers?: Container[]
  versionComparisons?: VersionComparison[]
  canRun?: boolean
}>(), {
  containers: () => [],
  versionComparisons: () => [],
  canRun: false,
})

const emit = defineEmits<{
  (e: 'open-command', payload: Record<string, unknown>): void
  (e: 'history-changed'): void
}>()

const dialog = useConfirmDialog()
const actionLoading = ref<Record<string, string | null>>({})

const { normalizedPortsForContainer } = useDockerContainerPorts(toRef(props, 'containers'))

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

function containerKey(container: Container): string {
  return container.name || container.id
}

async function runAction(container: Container, action: string): Promise<void> {
  const name = containerKey(container)
  if (actionLoading.value[name]) return

  if (action === 'stop' || action === 'restart') {
    const ok = await dialog.confirm({
      title: `${action === 'stop' ? 'Arrêter' : 'Redémarrer'} le conteneur`,
      message: `Confirmer : ${action} du conteneur « ${name} » ?`,
      variant: 'warning',
    })
    if (!ok) return
  }

  actionLoading.value = { ...actionLoading.value, [name]: action }
  try {
    const res = await apiClient.sendDockerCommand(String(props.hostId), name, action)
    emit('open-command', {
      id: res.data.command_id,
      module: 'docker',
      action,
      target: name,
      status: 'pending',
      output: '',
    })
    emit('history-changed')
  } catch (e: unknown) {
    addToast(getApiErrorMessage(e, 'Erreur Docker'), 'error', 6000)
  } finally {
    actionLoading.value = { ...actionLoading.value, [name]: null }
  }
}
</script>

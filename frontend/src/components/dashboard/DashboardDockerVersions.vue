<template>
  <div class="card">
    <div
      class="card-header dashboard-docker-header clickable-row"
      role="button"
      tabindex="0"
      :aria-expanded="isOpen"
      :aria-controls="panelId"
      @click="toggle"
      @keydown.enter.prevent="toggle"
      @keydown.space.prevent="toggle"
    >
      <h3 class="card-title d-flex align-items-center gap-2">
        Versions &amp; Mises à jour Docker
        <span
          v-if="outdatedCount > 0"
          class="badge bg-warning-lt text-warning"
        >{{ outdatedCount }} en retard</span>
        <IconChevronDown
          :size="16"
          class="ms-auto docker-chevron"
          :class="{ 'is-open': isOpen }"
        />
      </h3>
      <div class="card-options text-secondary small">
        Suivi via <router-link
          to="/git-webhooks"
          @click.stop
        >
          Git / Automatisation
        </router-link>
      </div>
    </div>
    <div
      v-show="isOpen"
      :id="panelId"
      class="table-responsive scroll-table"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Image</th>
            <th>Hôte</th>
            <th>Conteneurs</th>
            <th>En cours</th>
            <th>Dernière version</th>
            <th>Statut</th>
            <th>Task</th>
            <th class="text-end">
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="v in versions"
            :key="v.docker_image + v.host_id"
          >
            <td class="fw-semibold">
              {{ v.docker_image }}
            </td>
            <td class="text-secondary">
              {{ v.hostname }}
            </td>
            <td>
              <span
                v-if="(v.container_count ?? 0) > 0"
                class="badge bg-azure-lt text-azure"
                :title="`${v.container_count} conteneur${(v.container_count ?? 0) > 1 ? 's' : ''} utilisent cette image`"
              >{{ v.container_count }}</span>
              <span
                v-else
                class="text-secondary small"
              >—</span>
            </td>
            <td>
              <code v-if="v.running_version">{{ v.running_version }}</code><span
                v-else
                class="text-secondary small"
              >inconnue</span>
            </td>
            <td>
              <a
                v-if="v.release_url"
                :href="v.release_url"
                target="_blank"
                rel="noopener noreferrer"
                class="link-primary"
              >{{ v.latest_version }}</a>
              <span v-else>{{ v.latest_version }}</span>
            </td>
            <td>
              <span
                v-if="v.is_up_to_date"
                class="badge bg-success-lt text-success"
              >À jour</span>
              <span
                v-else-if="v.running_version || v.update_confirmed"
                class="badge bg-warning-lt text-warning"
              >Mise à jour disponible</span>
              <span
                v-else
                class="badge bg-secondary-lt text-secondary"
              >Version inconnue</span>
            </td>
            <td>
              <span
                v-if="v.custom_task_id"
                class="badge bg-success-lt text-success"
                title="Task de déploiement configurée"
              >✅ Déploiement</span>
              <span
                v-else-if="v.tracker_id"
                class="badge bg-warning-lt text-warning"
                title="Surveillance active mais aucune task configurée"
              >⏸️ Surveillance</span>
              <span
                v-else
                class="badge bg-secondary-lt text-secondary"
                title="Aucun tracker configuré"
              >❌ Non suivi</span>
            </td>
            <td class="text-end">
              <div class="btn-list justify-content-end">
                <router-link
                  v-if="v.tracker_id"
                  :to="`/release-trackers/${v.tracker_id}`"
                  class="btn btn-sm btn-outline-secondary"
                  title="Ouvrir le suivi de version"
                >
                  Voir suivi
                </router-link>
                <button
                  v-if="v.tracker_id"
                  type="button"
                  class="btn btn-sm btn-primary"
                  :disabled="isRunDisabled(v)"
                  :title="runTooltip(v)"
                  @click="runTracker(v)"
                >
                  {{ runningIds[v.tracker_id] ? 'Déclenchement...' : 'Déclencher' }}
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="versions.length === 0">
            <td colspan="8">
              <EmptyState
                title="Aucun suivi de version configuré."
                subtitle="Ajoutez des release trackers pour surveiller vos images Docker."
                cta-label="Git / Automatisation"
                cta-to="/git-webhooks"
              />
            </td>
          </tr>
        </tbody>
      </table>
      <div
        v-if="feedback"
        class="alert alert-dismissible m-3 mb-0 py-2"
        :class="feedbackIsError ? 'alert-danger' : 'alert-success'"
        role="status"
      >
        {{ feedback }}
        <button
          type="button"
          class="btn-close"
          aria-label="Fermer"
          @click="dismissFeedback"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { IconChevronDown } from '@tabler/icons-vue'
import apiClient from '../../api'
import { useAuthStore } from '../../stores/auth'
import EmptyState from '../EmptyState.vue'
import { getApiErrorMessage } from '../../api/client'

interface DockerVersion {
  docker_image: string
  host_id: string
  hostname?: string
  container_count?: number
  running_version?: string
  release_url?: string
  latest_version?: string
  is_up_to_date?: boolean
  update_confirmed?: boolean
  custom_task_id?: string
  tracker_id?: string
}

const props = withDefaults(defineProps<{
  versions?: DockerVersion[]
}>(), {
  versions: () => [],
})

const auth = useAuthStore()
const isOpen = ref(false)
const panelId = 'dashboard-docker-versions-panel'
const runningIds = ref<Record<string, boolean>>({})
const feedback = ref('')
const feedbackIsError = ref(false)
const FEEDBACK_TIMEOUT_MS = 6000
let feedbackTimer: ReturnType<typeof setTimeout> | null = null

function showFeedback(message: string, isError = false): void {
  if (feedbackTimer) clearTimeout(feedbackTimer)
  feedback.value = message
  feedbackIsError.value = isError
  feedbackTimer = setTimeout(() => {
    feedback.value = ''
    feedbackTimer = null
  }, FEEDBACK_TIMEOUT_MS)
}

function dismissFeedback(): void {
  if (feedbackTimer) {
    clearTimeout(feedbackTimer)
    feedbackTimer = null
  }
  feedback.value = ''
}

onUnmounted(() => {
  if (feedbackTimer) clearTimeout(feedbackTimer)
})

const outdatedCount = computed(() =>
  props.versions.filter(v => !v.is_up_to_date && (v.running_version || v.update_confirmed)).length
)

const canRunTracker = computed(() => auth.role === 'admin' || auth.role === 'operator')

function toggle() {
  isOpen.value = !isOpen.value
}

function hasManualData(v: DockerVersion): boolean {
  return !!(v.latest_version && String(v.latest_version).trim())
}

function isRunDisabled(v: DockerVersion): boolean {
  if (!canRunTracker.value) return true
  if (!v?.tracker_id) return true
  if (!hasManualData(v)) return true
  return !!runningIds.value[v.tracker_id]
}

function runTooltip(v: DockerVersion): string {
  if (!canRunTracker.value) return 'Action réservée admin/opérateur'
  if (!hasManualData(v)) return 'Attendez la première vérification automatique'
  return 'Déclencher la tâche du tracker maintenant'
}

async function runTracker(v: DockerVersion): Promise<void> {
  if (isRunDisabled(v)) return
  const id = v.tracker_id!
  runningIds.value = { ...runningIds.value, [id]: true }
  dismissFeedback()
  try {
    await apiClient.runReleaseTracker(id)
    showFeedback(`Déclenchement lancé pour ${v.docker_image}.`)
  } catch (e: unknown) {
    showFeedback(getApiErrorMessage(e, 'Échec du déclenchement manuel.'), true)
  } finally {
    const next = { ...runningIds.value }
    delete next[id]
    runningIds.value = next
  }
}
</script>

<style scoped>
.docker-chevron {
  flex-shrink: 0;
  transition: transform 0.2s;
}

.docker-chevron.is-open {
  transform: rotate(180deg);
}
</style>

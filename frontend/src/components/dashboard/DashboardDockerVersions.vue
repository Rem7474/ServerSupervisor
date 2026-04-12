<template>
  <div class="card">
    <div
      class="card-header dashboard-docker-header"
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
          class="badge bg-yellow-lt text-yellow"
        >{{ outdatedCount }} en retard</span>
        <svg
          class="ms-auto docker-chevron"
          :class="{ 'is-open': isOpen }"
          width="16"
          height="16"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
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
      class="table-responsive"
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
            <th class="text-end">Actions</th>
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
                v-if="v.container_count > 0"
                class="badge bg-azure-lt text-azure"
                :title="`${v.container_count} conteneur${v.container_count > 1 ? 's' : ''} utilisent cette image`"
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
                class="badge bg-green-lt text-green"
              >À jour</span>
              <span
                v-else-if="v.running_version || v.update_confirmed"
                class="badge bg-yellow-lt text-yellow"
              >Mise à jour disponible</span>
              <span
                v-else
                class="badge bg-secondary-lt text-secondary"
              >Version inconnue</span>
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
            <td
              colspan="7"
              class="text-center text-secondary py-4"
            >
              Aucun suivi de version configuré. Ajoutez des release trackers dans
              <router-link to="/git-webhooks">
                Git / Automatisation
              </router-link>.
            </td>
          </tr>
        </tbody>
      </table>
      <div
        v-if="feedback"
        class="alert alert-info m-3 mb-0 py-2"
        role="status"
      >
        {{ feedback }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { defineProps, ref, computed } from 'vue'
import apiClient from '../../api'
import { useAuthStore } from '../../stores/auth'

const props = defineProps({
  versions: { type: Array, default: () => [] },
})

const auth = useAuthStore()
const isOpen = ref(false)
const panelId = 'dashboard-docker-versions-panel'
const runningIds = ref({})
const feedback = ref('')

const outdatedCount = computed(() =>
  props.versions.filter(v => !v.is_up_to_date && (v.running_version || v.update_confirmed)).length
)

const canRunTracker = computed(() => auth.role === 'admin' || auth.role === 'operator')

function toggle() {
  isOpen.value = !isOpen.value
}

function hasManualData(v) {
  return !!(v.latest_version && String(v.latest_version).trim())
}

function isRunDisabled(v) {
  if (!canRunTracker.value) return true
  if (!v?.tracker_id) return true
  if (!hasManualData(v)) return true
  return !!runningIds.value[v.tracker_id]
}

function runTooltip(v) {
  if (!canRunTracker.value) return 'Action réservée admin/opérateur'
  if (!hasManualData(v)) return 'Attendez la première vérification automatique'
  return 'Déclencher la tâche du tracker maintenant'
}

async function runTracker(v) {
  if (isRunDisabled(v)) return
  const id = v.tracker_id
  runningIds.value = { ...runningIds.value, [id]: true }
  feedback.value = ''
  try {
    await apiClient.runReleaseTracker(id)
    feedback.value = `Déclenchement lancé pour ${v.docker_image}.`
  } catch (e) {
    feedback.value = e.response?.data?.error || 'Échec du déclenchement manuel.'
  } finally {
    const next = { ...runningIds.value }
    delete next[id]
    runningIds.value = next
  }
}
</script>

<style scoped>
.dashboard-docker-header {
  cursor: pointer;
}

.docker-chevron {
  flex-shrink: 0;
  transition: transform 0.2s;
}

.docker-chevron.is-open {
  transform: rotate(180deg);
}
</style>

<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        Suivi de versions
      </h3>
      <router-link
        to="/git-webhooks"
        class="btn btn-sm btn-ghost-secondary"
      >
        Gérer
        <svg
          class="icon icon-sm ms-1"
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
          />
        </svg>
      </router-link>
    </div>

    <div
      v-if="loading"
      class="card-body text-center py-5"
    >
      <div
        class="spinner-border text-primary"
        role="status"
      />
      <div class="mt-2 text-muted">
        Chargement...
      </div>
    </div>

    <div
      v-else-if="error"
      class="card-body text-center py-5 text-danger"
    >
      {{ error }}
    </div>

    <div
      v-else-if="trackers.length === 0"
      class="card-body text-center py-5 text-muted"
    >
      <svg
        class="icon icon-lg mb-3 d-block mx-auto opacity-50"
        width="48"
        height="48"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M22 12h-4l-3 9L9 3l-3 9H2"
        />
      </svg>
      <div>Aucun tracker configuré</div>
      <router-link
        to="/git-webhooks"
        class="btn btn-sm btn-primary mt-3"
      >
        Créer un tracker
      </router-link>
    </div>

    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Tracker</th>
            <th>Type</th>
            <th>Dernière version</th>
            <th>Dernière exécution</th>
            <th>Vérifié</th>
            <th class="w-1" />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="tracker in trackers"
            :key="tracker.id"
          >
            <td>
              <div class="d-flex align-items-center gap-2">
                <span
                  v-if="!tracker.enabled"
                  class="badge bg-secondary-lt text-secondary"
                >Désactivé</span>
                <span class="fw-bold">{{ tracker.name }}</span>
              </div>
              <div
                v-if="tracker.host_name"
                class="text-muted small"
              >
                {{ tracker.host_name }}
              </div>
              <div
                v-if="tracker.last_error"
                class="text-danger small mt-1"
                :title="tracker.last_error"
              >
                <svg
                  class="icon icon-sm me-1"
                  width="14"
                  height="14"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
                  />
                </svg>
                Erreur lors de la vérification
              </div>
            </td>
            <td>
              <span
                class="badge"
                :class="tracker.tracker_type === 'docker' ? 'bg-cyan-lt text-cyan' : 'bg-purple-lt text-purple'"
              >
                {{ tracker.tracker_type === 'docker' ? 'Docker' : 'Git' }}
              </span>
            </td>
            <td>
              <div v-if="tracker.last_release_tag">
                <code class="text-body">{{ tracker.last_release_tag }}</code>
                <div
                  v-if="tracker.last_release_detected_at"
                  class="text-muted small"
                >
                  {{ formatDate(tracker.last_release_detected_at) }}
                </div>
              </div>
              <span
                v-else
                class="text-muted"
              >—</span>
            </td>
            <td>
              <div v-if="tracker.last_execution">
                <span
                  class="badge"
                  :class="executionBadgeClass(tracker.last_execution.status)"
                >{{ executionLabel(tracker.last_execution.status) }}</span>
                <div class="text-muted small">
                  {{ tracker.last_execution.tag_name }}
                </div>
              </div>
              <span
                v-else-if="!tracker.host_id"
                class="text-muted small"
              >Surveillance seule</span>
              <span
                v-else
                class="text-muted"
              >—</span>
            </td>
            <td>
              <span
                v-if="tracker.last_checked_at"
                class="text-muted small"
              >{{ formatDate(tracker.last_checked_at) }}</span>
              <span
                v-else
                class="text-muted"
              >Jamais</span>
            </td>
            <td>
              <router-link
                :to="`/release-trackers/${tracker.id}`"
                class="btn btn-sm btn-ghost-secondary"
                title="Voir le détail"
              >
                <svg
                  class="icon"
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
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </router-link>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useDateFormatter } from '../../composables/useDateFormatter'

interface TrackerExecution {
  status: string
  tag_name?: string
}

interface Tracker {
  id: string | number
  name: string
  enabled?: boolean
  host_id?: string
  host_name?: string
  tracker_type?: string
  last_release_tag?: string
  last_release_detected_at?: string
  last_checked_at?: string
  last_error?: string
  last_execution?: TrackerExecution | null
}

withDefaults(defineProps<{
  trackers?: Tracker[]
  loading?: boolean
  error?: string
}>(), {
  trackers: () => [],
  loading: false,
  error: '',
})

const { formatLocaleDateTime } = useDateFormatter()

function formatDate(dateStr: string | undefined): string {
  return formatLocaleDateTime(dateStr)
}

const EXECUTION_BADGE: Record<string, string> = {
  succeeded: 'bg-green-lt text-green',
  failed: 'bg-red-lt text-red',
  running: 'bg-blue-lt text-blue',
  pending: 'bg-yellow-lt text-yellow',
}

const EXECUTION_LABEL: Record<string, string> = {
  succeeded: 'Succès',
  failed: 'Échec',
  running: 'En cours',
  pending: 'En attente',
}

function executionBadgeClass(status: string): string {
  return EXECUTION_BADGE[status] || 'bg-secondary-lt text-secondary'
}

function executionLabel(status: string): string {
  return EXECUTION_LABEL[status] || status
}
</script>

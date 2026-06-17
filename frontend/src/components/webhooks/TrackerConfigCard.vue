<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        Configuration
      </h3>
      <div class="d-flex gap-2">
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :disabled="checking"
          @click="$emit('check')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
            class="me-1"
          >
            <polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10" />
          </svg>
          {{ checking ? 'Vérification...' : 'Vérifier maintenant' }}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-primary"
          :disabled="running || !canRunManually"
          :title="runDisabledReason"
          @click="$emit('run')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
            class="me-1"
          >
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
          {{ running ? 'Déclenchement...' : 'Exécuter' }}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          @click="$emit('edit')"
        >
          Modifier
        </button>
      </div>
    </div>
    <div class="card-body">
      <dl class="row mb-0 small">
        <dt class="col-5 text-muted">
          Type
        </dt>
        <dd class="col-7">
          <span
            v-if="tracker.tracker_type === 'docker'"
            class="badge bg-cyan-lt text-cyan"
          >Image Docker</span>
          <span
            v-else
            class="badge bg-blue-lt text-blue"
          >Release Git</span>
        </dd>

        <!-- Git-specific -->
        <template v-if="tracker.tracker_type !== 'docker'">
          <dt class="col-5 text-muted">
            Provider
          </dt>
          <dd class="col-7">
            {{ tracker.provider }}
          </dd>
          <dt class="col-5 text-muted">
            Dépôt
          </dt>
          <dd class="col-7">
            <a
              :href="repoURL"
              target="_blank"
              class="link-primary"
            >
              {{ tracker.repo_owner }}/{{ tracker.repo_name }}
            </a>
          </dd>
          <dt class="col-5 text-muted">
            Dernière release
          </dt>
          <dd class="col-7">
            <span
              v-if="tracker.last_release_tag"
              class="badge bg-green-lt text-green"
            >{{ tracker.last_release_tag }}</span>
            <span
              v-else
              class="text-muted"
            >En attente...</span>
          </dd>
        </template>

        <!-- Docker-specific -->
        <template v-else>
          <dt class="col-5 text-muted">
            Image
          </dt>
          <dd class="col-7">
            <code>{{ tracker.docker_image }}</code>
          </dd>
          <dt class="col-5 text-muted">
            Tag surveillé
          </dt>
          <dd class="col-7">
            <code>{{ tracker.docker_tag || 'latest' }}</code>
          </dd>
          <template v-if="tracker.latest_image_digest">
            <dt class="col-5 text-muted">
              Dernier digest
            </dt>
            <dd class="col-7">
              <code
                class="small text-muted"
                :title="tracker.latest_image_digest"
              >
                {{ tracker.latest_image_digest.slice(0, 19) }}…
              </code>
            </dd>
          </template>
          <dt class="col-5 text-muted">
            Dernier check
          </dt>
          <dd class="col-7">
            <span v-if="tracker.last_checked_at"><RelativeTime :date="tracker.last_checked_at" /></span>
            <span
              v-else
              class="text-muted"
            >Jamais</span>
          </dd>

          <template v-if="tracker.repo_owner && tracker.repo_name">
            <dt class="col-5 text-muted">
              Repo lié
            </dt>
            <dd class="col-7">
              <a
                :href="repoURL"
                target="_blank"
                class="link-primary"
              >
                {{ tracker.repo_owner }}/{{ tracker.repo_name }}
              </a>
              <div class="small mt-1">
                <a
                  :href="releaseNotesURL"
                  target="_blank"
                  class="link-secondary"
                >Voir les release notes</a>
              </div>
            </dd>
          </template>
        </template>

        <!-- Common fields -->
        <template v-if="tracker.host_id && tracker.custom_task_id">
          <dt class="col-5 text-muted">
            VM cible
          </dt>
          <dd class="col-7">
            {{ tracker.host_name || tracker.host_id }}
          </dd>
          <dt class="col-5 text-muted">
            Tâche
          </dt>
          <dd class="col-7">
            <code>{{ tracker.custom_task_id }}</code>
          </dd>
        </template>
        <template v-else-if="!tracker.host_id || !tracker.custom_task_id">
          <dt class="col-5 text-muted">
            Mode
          </dt>
          <dd class="col-7">
            <span class="badge bg-blue-lt text-blue">Surveillance seule</span>
          </dd>
        </template>
        <dt
          v-if="tracker.tracker_type !== 'docker' && tracker.last_checked_at"
          class="col-5 text-muted"
        >
          Dernier check
        </dt>
        <dd
          v-if="tracker.tracker_type !== 'docker' && tracker.last_checked_at"
          class="col-7"
        >
          <RelativeTime :date="tracker.last_checked_at" />
        </dd>
        <template v-if="tracker.last_error">
          <dt class="col-5 text-muted">
            Erreur
          </dt>
          <dd class="col-7 text-danger small">
            {{ tracker.last_error }}
          </dd>
        </template>
        <dt
          v-if="tracker.last_triggered_at"
          class="col-5 text-muted"
        >
          Dernier déclench.
        </dt>
        <dd
          v-if="tracker.last_triggered_at"
          class="col-7"
        >
          <RelativeTime :date="tracker.last_triggered_at" />
        </dd>
        <dt
          v-if="tracker.notify_channels?.length"
          class="col-5 text-muted"
        >
          Notifications
        </dt>
        <dd
          v-if="tracker.notify_channels?.length"
          class="col-7"
        >
          <span
            v-for="ch in tracker.notify_channels"
            :key="ch"
            class="badge me-1"
            :class="channelBadge(ch)"
          >{{ ch }}</span>
        </dd>
        <dt class="col-5 text-muted">
          Créé le
        </dt>
        <dd class="col-7">
          {{ formatDateTime(tracker.created_at) }}
        </dd>
        <template v-if="Number(tracker.cooldown_hours || 0) > 0">
          <dt class="col-5 text-muted">
            Cooldown
          </dt>
          <dd class="col-7">
            {{ `${tracker.cooldown_hours}h` }}
          </dd>
        </template>
        <template v-if="cooldownActive">
          <dt class="col-5 text-muted">
            Déploiement prévu
          </dt>
          <dd class="col-7">
            {{ cooldownEtaText }}
          </dd>
        </template>
      </dl>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import RelativeTime from '../RelativeTime.vue'
import { formatDateTime } from '../../utils/formatters'
import type { ReleaseTracker } from '../../types/tracker'

const props = defineProps<{
  tracker: ReleaseTracker
  checking: boolean
  running: boolean
  canRunManually: boolean
  runDisabledReason: string
  cooldownActive: boolean
  cooldownEtaText: string
}>()

defineEmits<{
  (e: 'check'): void
  (e: 'run'): void
  (e: 'edit'): void
}>()

const repoURL = computed(() => {
  const t = props.tracker
  if (!t || !t.repo_owner || !t.repo_name) return '#'
  switch (t.provider) {
    case 'gitlab': return `https://gitlab.com/${t.repo_owner}/${t.repo_name}`
    case 'gitea': return `https://codeberg.org/${t.repo_owner}/${t.repo_name}`
    default: return `https://github.com/${t.repo_owner}/${t.repo_name}`
  }
})

const releaseNotesURL = computed(() => {
  const t = props.tracker
  if (!t || !t.repo_owner || !t.repo_name) return '#'
  switch (t.provider) {
    case 'gitlab': return `https://gitlab.com/${t.repo_owner}/${t.repo_name}/-/releases`
    case 'gitea':
    case 'forgejo':
      return `https://codeberg.org/${t.repo_owner}/${t.repo_name}/releases`
    default:
      return `https://github.com/${t.repo_owner}/${t.repo_name}/releases`
  }
})

function channelBadge(ch: string): string {
  const map: Record<string, string> = {
    smtp: 'bg-blue-lt text-blue',
    ntfy: 'bg-orange-lt text-orange',
    browser: 'bg-purple-lt text-purple',
  }
  return map[ch] || 'bg-secondary-lt text-secondary'
}
</script>

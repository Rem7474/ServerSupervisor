<template>
  <div
    v-if="visible"
    ref="modalRef"
    class="modal modal-blur show d-block"
    style="background:rgba(0,0,0,.5)"
    role="dialog"
    aria-modal="true"
  >
    <div class="modal-dialog modal-lg">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            {{ title }}
          </h5>
          <button
            type="button"
            class="btn-close"
            @click="close"
          />
        </div>
        <div class="modal-body">
          <div
            v-if="errorMessage"
            class="alert alert-danger"
          >
            {{ errorMessage }}
          </div>

          <div class="row g-3">
            <div class="col-12">
              <label class="form-label required">Nom</label>
              <input
                v-model="form.name"
                type="text"
                class="form-control"
                :placeholder="mode === 'webhook' ? 'ex: Deploy mon-app' : 'ex: Mise a jour Home Assistant'"
              >
            </div>

            <!-- ===== WEBHOOK FIELDS ===== -->
            <template v-if="mode === 'webhook'">
              <div class="col-md-6">
                <label class="form-label required">Provider</label>
                <select
                  v-model="form.provider"
                  class="form-select"
                >
                  <option value="github">
                    GitHub
                  </option>
                  <option value="gitlab">
                    GitLab
                  </option>
                  <option value="gitea">
                    Gitea
                  </option>
                  <option value="forgejo">
                    Forgejo
                  </option>
                  <option value="custom">
                    Custom
                  </option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Evenement</label>
                <select
                  v-model="form.event_filter"
                  class="form-select"
                >
                  <option value="push">
                    push
                  </option>
                  <option value="tag">
                    tag / create
                  </option>
                  <option value="release">
                    release
                  </option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre repo <span class="text-muted">(optionnel)</span></label>
                <input
                  v-model="form.repo_filter"
                  type="text"
                  class="form-control"
                  placeholder="ex: monorg/mon-app"
                >
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre branche <span class="text-muted">(optionnel)</span></label>
                <input
                  v-model="form.branch_filter"
                  type="text"
                  class="form-control"
                  placeholder="ex: main"
                >
              </div>
            </template>

            <!-- ===== TRACKER FIELDS ===== -->
            <template v-else>
              <WebhookTrackerFields
                :form="form"
                :registry-credentials="registryCredentials"
              />
            </template>

            <!-- VM + Task -->
            <!-- Trackers: optional dispatch toggle -->
            <div
              v-if="mode === 'tracker'"
              class="col-12"
            >
              <label class="form-check form-switch">
                <input
                  v-model="form.dispatch_task"
                  class="form-check-input"
                  type="checkbox"
                >
                <span class="form-check-label fw-medium">Déclencher une tâche lors d'une mise à jour</span>
              </label>
              <div
                id="dispatch-task-hint"
                class="form-hint text-muted"
              >
                Si désactivé, le tracker surveille uniquement et enregistre la version sans exécuter de script.
              </div>
            </div>

            <div
              v-if="mode === 'tracker' && form.dispatch_task"
              class="col-md-4"
            >
              <label class="form-label">Cooldown (heures)</label>
              <input
                v-model.number="form.cooldown_hours"
                type="number"
                min="0"
                max="168"
                class="form-control"
                placeholder="0"
              >
              <div class="form-hint">
                Délai avant déclenchement après détection d'une nouvelle version (0 = immédiat).
              </div>
            </div>

            <!-- Docker tracker: choose deployment mode -->
            <div
              v-if="mode === 'tracker' && form.dispatch_task && form.tracker_type === 'docker'"
              class="col-12"
            >
              <label class="form-label">Mode de mise a jour</label>
              <div class="row g-2">
                <div class="col-6">
                  <label
                    class="tracker-type-card"
                    :class="form.update_action === 'compose' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
                  >
                    <input
                      v-model="form.update_action"
                      class="tracker-type-input"
                      type="radio"
                      value="compose"
                    >
                    <span>
                      <span class="fw-semibold d-block">Compose (natif)</span>
                      <span class="text-muted small">pull + up -d automatique sur un projet compose, sans script</span>
                    </span>
                  </label>
                </div>
                <div class="col-6">
                  <label
                    class="tracker-type-card"
                    :class="form.update_action !== 'compose' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
                  >
                    <input
                      v-model="form.update_action"
                      class="tracker-type-input"
                      type="radio"
                      value="custom"
                    >
                    <span>
                      <span class="fw-semibold d-block">Tache (tasks.yaml)</span>
                      <span class="text-muted small">Execute un script declare cote agent</span>
                    </span>
                  </label>
                </div>
              </div>
            </div>

            <template v-if="mode === 'webhook' || (mode === 'tracker' && form.dispatch_task)">
              <div class="col-md-6">
                <label
                  class="form-label"
                  :class="(mode === 'webhook' || (mode === 'tracker' && form.dispatch_task)) ? 'required' : ''"
                >VM cible</label>
                <select
                  v-model="form.host_id"
                  class="form-select"
                >
                  <option value="">
                    -- Sélectionner un hôte --
                  </option>
                  <option
                    v-for="host in hosts"
                    :key="host.id"
                    :value="host.id"
                  >
                    {{ host.name }}
                  </option>
                </select>
              </div>

              <!-- Compose mode: project + service -->
              <template v-if="isComposeMode">
                <div class="col-md-6">
                  <label class="form-label required">Projet compose</label>
                  <input
                    v-model="form.compose_project"
                    type="text"
                    class="form-control"
                    placeholder="ex: mon-app"
                    aria-describedby="compose-project-hint"
                  >
                  <div
                    id="compose-project-hint"
                    class="form-hint"
                  >
                    Nom du projet compose tel que decouvert sur l'hote (label <code>com.docker.compose.project</code>).
                  </div>
                </div>
                <div class="col-md-6">
                  <label class="form-label">Service <span class="text-muted">(optionnel)</span></label>
                  <input
                    v-model="form.compose_service"
                    type="text"
                    class="form-control"
                    placeholder="laisser vide = tout le projet"
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label">Healthcheck (s)</label>
                  <input
                    v-model.number="form.healthcheck_timeout_sec"
                    type="number"
                    min="0"
                    max="3600"
                    class="form-control"
                    placeholder="0"
                  >
                  <div class="form-hint">
                    Attente max de l'etat healthy apres up -d (0 = desactive).
                  </div>
                </div>
                <div class="col-md-8 d-flex align-items-end">
                  <div class="d-flex flex-wrap gap-3 pb-2">
                    <label class="form-check">
                      <input
                        v-model="form.rollback_on_failure"
                        class="form-check-input"
                        type="checkbox"
                      >
                      <span class="form-check-label">Rollback si echec / unhealthy</span>
                    </label>
                    <label class="form-check">
                      <input
                        v-model="form.cleanup_after_update"
                        class="form-check-input"
                        type="checkbox"
                      >
                      <span class="form-check-label">Nettoyer les images orphelines</span>
                    </label>
                  </div>
                </div>
                <div class="col-md-6">
                  <label class="form-label">Hook avant MAJ <span class="text-muted">(optionnel)</span></label>
                  <select
                    v-if="customTasks.length"
                    v-model="form.pre_update_task_id"
                    class="form-select"
                  >
                    <option value="">
                      -- Aucun --
                    </option>
                    <option
                      v-for="task in customTasks"
                      :key="task.id"
                      :value="task.id"
                    >
                      {{ task.name }} ({{ task.id }})
                    </option>
                  </select>
                  <input
                    v-else
                    v-model="form.pre_update_task_id"
                    type="text"
                    class="form-control"
                    placeholder="ex: backup-postgres"
                  >
                  <div class="form-hint">
                    Tache <code>tasks.yaml</code> executee avant le pull (ex: sauvegarde).
                  </div>
                </div>
                <div class="col-md-6">
                  <label class="form-label">Hook apres MAJ <span class="text-muted">(optionnel)</span></label>
                  <select
                    v-if="customTasks.length"
                    v-model="form.post_update_task_id"
                    class="form-select"
                  >
                    <option value="">
                      -- Aucun --
                    </option>
                    <option
                      v-for="task in customTasks"
                      :key="task.id"
                      :value="task.id"
                    >
                      {{ task.name }} ({{ task.id }})
                    </option>
                  </select>
                  <input
                    v-else
                    v-model="form.post_update_task_id"
                    type="text"
                    class="form-control"
                    placeholder="ex: verify-health"
                  >
                </div>
              </template>

              <!-- Custom / webhook mode: tasks.yaml task -->
              <div
                v-else
                class="col-md-6"
              >
                <label
                  class="form-label"
                  :class="(mode === 'webhook' || (mode === 'tracker' && form.dispatch_task)) ? 'required' : ''"
                >Tache (tasks.yaml)</label>
                <select
                  v-if="customTasks.length"
                  v-model="form.custom_task_id"
                  class="form-select"
                >
                  <option
                    value=""
                    disabled
                  >
                    -- Sélectionner une tâche --
                  </option>
                  <option
                    v-for="task in customTasks"
                    :key="task.id"
                    :value="task.id"
                  >
                    {{ task.name }} ({{ task.id }})
                  </option>
                </select>
                <input
                  v-else
                  v-model="form.custom_task_id"
                  type="text"
                  class="form-control"
                  :placeholder="mode === 'webhook' ? 'ex: deploy-mon-app' : 'ex: update-home-assistant'"
                  aria-describedby="task-id-hint"
                >
                <div
                  id="task-id-hint"
                  class="form-hint"
                >
                  Correspond a l'<code>id</code> dans <code>tasks.yaml</code> de l'agent.
                </div>
              </div>
            </template>

            <!-- Notifications -->
            <div class="col-12">
              <label class="form-label">Notifications</label>
              <div class="d-flex flex-wrap gap-3 mt-1">
                <label
                  v-if="mode === 'webhook'"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_on_success"
                    class="form-check-input"
                    type="checkbox"
                  >
                  <span class="form-check-label">En cas de succes</span>
                </label>
                <label
                  v-if="mode === 'webhook'"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_on_failure"
                    class="form-check-input"
                    type="checkbox"
                  >
                  <span class="form-check-label">En cas d'echec</span>
                </label>
                <label
                  v-if="mode === 'tracker'"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_on_release"
                    class="form-check-input"
                    type="checkbox"
                    :disabled="!form.notify_channels.length"
                  >
                  <span class="form-check-label">Notifier a chaque mise a jour detectee</span>
                </label>
              </div>
              <div class="d-flex flex-wrap gap-3 mt-2">
                <label
                  v-for="channel in ['smtp', 'ntfy', 'browser']"
                  :key="channel"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_channels"
                    class="form-check-input"
                    type="checkbox"
                    :value="channel"
                  >
                  <span class="form-check-label">{{ channel }}</span>
                </label>
              </div>
              <div
                v-if="mode === 'tracker'"
                class="form-hint mt-2"
              >
                Activez au moins un canal pour pouvoir notifier les nouvelles versions.
              </div>
            </div>

            <div class="col-12 border-top pt-3">
              <label class="form-check form-switch mb-0">
                <input
                  v-model="form.enabled"
                  class="form-check-input"
                  type="checkbox"
                >
                <span class="form-check-label fw-medium">Activer ce {{ mode === 'tracker' ? 'tracker' : 'webhook' }}</span>
              </label>
            </div>
          </div>

          <!-- Env vars table for trackers -->
          <WebhookEnvVarsCard
            v-if="mode === 'tracker'"
            :env-vars="currentEnvVars"
          />
        </div>
        <div class="modal-footer">
          <button
            class="btn btn-secondary"
            @click="close"
          >
            Annuler
          </button>
          <button
            class="btn btn-primary"
            :disabled="saving"
            @click="submit"
          >
            {{ saving ? 'Enregistrement...' : submitLabel }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useModalFocusTrap } from '../../composables/useModalFocusTrap'
import { useWebhookForm, type WebhookItem, type Host, type WebhookFormData } from '../../composables/useWebhookForm'
import WebhookTrackerFields from './WebhookTrackerFields.vue'
import WebhookEnvVarsCard from './WebhookEnvVarsCard.vue'

const props = withDefaults(defineProps<{
  visible?: boolean
  mode?: string
  item?: WebhookItem | null
  hosts?: Host[]
  saving?: boolean
  error?: string
  prefillDockerImage?: string
  prefillDockerTag?: string
  prefillComposeProject?: string
}>(), {
  visible: false,
  mode: 'webhook',
  item: null,
  hosts: () => [],
  saving: false,
  error: '',
  prefillDockerImage: '',
  prefillDockerTag: '',
  prefillComposeProject: '',
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: WebhookFormData): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalFocusTrap(modalRef)

const {
  form,
  customTasks,
  registryCredentials,
  currentEnvVars,
  isComposeMode,
  title,
  submitLabel,
  errorMessage,
  submit,
  clearError,
} = useWebhookForm(props, (payload) => emit('submit', payload))

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      document.addEventListener('keydown', onKeyDown)
      return
    }
    document.removeEventListener('keydown', onKeyDown)
  },
  { immediate: true },
)

onUnmounted(() => {
  document.removeEventListener('keydown', onKeyDown)
})

function close(): void {
  clearError()
  emit('close')
}

function onKeyDown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && props.visible) close()
}
</script>

<style scoped>
.tracker-type-card {
  display: block;
  width: 100%;
  padding: 1rem;
  border-radius: 0.5rem;
  border: 1px solid var(--tblr-border-color);
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease;
}

.tracker-type-card--active {
  border-color: var(--tblr-primary);
  background: var(--tblr-primary-lt);
}

.tracker-type-card--idle {
  border-color: var(--tblr-border-color);
  background: transparent;
}

.tracker-type-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  pointer-events: none;
}
</style>

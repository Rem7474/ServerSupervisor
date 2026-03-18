<template>
  <div v-if="visible" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.5)">
    <div class="modal-dialog modal-lg">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">{{ title }}</h5>
          <button type="button" class="btn-close" @click="close"></button>
        </div>
        <div class="modal-body">
          <div v-if="errorMessage" class="alert alert-danger">{{ errorMessage }}</div>

          <div class="row g-3">
            <div class="col-12">
              <label class="form-label required">Nom</label>
              <input v-model="form.name" type="text" class="form-control" :placeholder="mode === 'webhook' ? 'ex: Deploy mon-app' : 'ex: Mise a jour Home Assistant'" />
            </div>

            <!-- ===== WEBHOOK FIELDS ===== -->
            <template v-if="mode === 'webhook'">
              <div class="col-md-6">
                <label class="form-label required">Provider</label>
                <select class="form-select" v-model="form.provider">
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                  <option value="gitea">Gitea</option>
                  <option value="forgejo">Forgejo</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Evenement</label>
                <select class="form-select" v-model="form.event_filter">
                  <option value="push">push</option>
                  <option value="tag">tag / create</option>
                  <option value="release">release</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre repo <span class="text-muted">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="form.repo_filter" placeholder="ex: monorg/mon-app" />
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre branche <span class="text-muted">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="form.branch_filter" placeholder="ex: main" />
              </div>
            </template>

            <!-- ===== TRACKER FIELDS ===== -->
            <template v-else>
              <!-- Type selector (hidden when pre-set from Docker page) -->
              <div class="col-12">
                <label class="form-label required">Type de suivi</label>
                <div class="row g-2">
                  <div class="col-6">
                    <label class="form-check form-check-inline w-100 p-3 border rounded cursor-pointer" :class="form.tracker_type === 'git' ? 'border-primary bg-primary-lt' : 'border-muted'">
                      <input class="form-check-input" type="radio" v-model="form.tracker_type" value="git" />
                      <span class="ms-2">
                        <span class="fw-semibold d-block">Release Git</span>
                        <span class="text-muted small">Surveille les nouvelles releases/tags sur GitHub, GitLab ou Gitea</span>
                      </span>
                    </label>
                  </div>
                  <div class="col-6">
                    <label class="form-check form-check-inline w-100 p-3 border rounded cursor-pointer" :class="form.tracker_type === 'docker' ? 'border-primary bg-primary-lt' : 'border-muted'">
                      <input class="form-check-input" type="radio" v-model="form.tracker_type" value="docker" />
                      <span class="ms-2">
                        <span class="fw-semibold d-block">Image Docker</span>
                        <span class="text-muted small">Detecte quand une nouvelle image est poussee sur le registre</span>
                      </span>
                    </label>
                  </div>
                </div>
              </div>

              <!-- Git-specific fields -->
              <template v-if="form.tracker_type === 'git'">
                <div class="col-md-4">
                  <label class="form-label required">Provider</label>
                  <select class="form-select" v-model="form.provider">
                    <option value="github">GitHub</option>
                    <option value="gitlab">GitLab</option>
                    <option value="gitea">Gitea (Codeberg)</option>
                  </select>
                </div>
                <div class="col-md-4">
                  <label class="form-label required">Owner / Org</label>
                  <input type="text" class="form-control" v-model="form.repo_owner" placeholder="ex: home-assistant" />
                </div>
                <div class="col-md-4">
                  <label class="form-label required">Depot</label>
                  <input type="text" class="form-control" v-model="form.repo_name" placeholder="ex: core" />
                </div>
              </template>

              <!-- Docker-specific fields -->
              <template v-else>
                <div class="col-md-8">
                  <label class="form-label required">Image Docker</label>
                  <input type="text" class="form-control" v-model="form.docker_image" placeholder="ex: homeassistant/home-assistant, nginx, ghcr.io/user/app" />
                  <div class="form-hint">Nom de l'image sans le tag (registre Docker Hub par defaut).</div>
                </div>
                <div class="col-md-4">
                  <label class="form-label required">Tag surveille</label>
                  <input type="text" class="form-control" v-model="form.docker_tag" placeholder="latest" />
                  <div class="form-hint">Tag de l'image a surveiller.</div>
                </div>
              </template>
            </template>

            <!-- VM + Task -->
            <!-- Docker trackers: optional dispatch toggle -->
            <div v-if="mode === 'tracker' && form.tracker_type === 'docker'" class="col-12">
              <label class="form-check form-switch">
                <input class="form-check-input" type="checkbox" v-model="form.dispatch_task" />
                <span class="form-check-label fw-medium">Declencher une tache lors d'une mise a jour</span>
              </label>
              <div class="form-hint text-muted">Si desactive, le tracker surveille uniquement et enregistre la version sans executer de script.</div>
            </div>

            <template v-if="mode === 'webhook' || form.tracker_type === 'git' || form.dispatch_task">
              <div class="col-md-6">
                <label class="form-label" :class="(mode === 'webhook' || form.tracker_type === 'git') ? 'required' : ''">VM cible</label>
                <select class="form-select" v-model="form.host_id">
                  <option value="">-- Selectionner un hote --</option>
                  <option v-for="host in hosts" :key="host.id" :value="host.id">{{ host.name }}</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label" :class="(mode === 'webhook' || form.tracker_type === 'git') ? 'required' : ''">Tache (tasks.yaml)</label>
                <select v-if="customTasks.length" class="form-select" v-model="form.custom_task_id">
                  <option value="" disabled>-- Selectionner une tache --</option>
                  <option v-for="task in customTasks" :key="task.id" :value="task.id">{{ task.name }} ({{ task.id }})</option>
                </select>
                <input v-else type="text" class="form-control" v-model="form.custom_task_id" :placeholder="mode === 'webhook' ? 'ex: deploy-mon-app' : 'ex: update-home-assistant'" />
                <div class="form-hint">Correspond a l'<code>id</code> dans <code>tasks.yaml</code> de l'agent.</div>
              </div>
            </template>

            <!-- Notifications -->
            <div class="col-12">
              <label class="form-label">Notifications</label>
              <div class="d-flex flex-wrap gap-3 mt-1">
                <label v-if="mode === 'webhook'" class="form-check">
                  <input class="form-check-input" type="checkbox" v-model="form.notify_on_success" />
                  <span class="form-check-label">En cas de succes</span>
                </label>
                <label v-if="mode === 'webhook'" class="form-check">
                  <input class="form-check-input" type="checkbox" v-model="form.notify_on_failure" />
                  <span class="form-check-label">En cas d'echec</span>
                </label>
                <label v-if="mode === 'tracker'" class="form-check">
                  <input class="form-check-input" type="checkbox" v-model="form.notify_on_release" />
                  <span class="form-check-label">Notifier a chaque mise a jour detectee</span>
                </label>
              </div>
              <div class="d-flex flex-wrap gap-3 mt-2">
                <label v-for="channel in ['smtp', 'ntfy', 'browser']" :key="channel" class="form-check">
                  <input class="form-check-input" type="checkbox" :value="channel" v-model="form.notify_channels" />
                  <span class="form-check-label">{{ channel }}</span>
                </label>
              </div>
            </div>

            <div class="col-12">
              <label class="form-check form-switch">
                <input class="form-check-input" type="checkbox" v-model="form.enabled" />
                <span class="form-check-label">Active</span>
              </label>
            </div>
          </div>

          <!-- Env vars table for trackers -->
          <div v-if="mode === 'tracker'" class="mt-3 pt-3 border-top">
            <div class="text-muted small mb-2">Variables injectees dans votre script :</div>
            <div class="table-responsive">
              <table class="table table-sm mb-0">
                <tbody>
                  <tr v-for="variable in currentEnvVars" :key="variable.name">
                    <td class="py-1"><code class="small">{{ variable.name }}</code></td>
                    <td class="py-1 text-muted small">{{ variable.desc }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="close">Annuler</button>
          <button class="btn btn-primary" @click="submit" :disabled="saving">
            {{ saving ? 'Enregistrement...' : submitLabel }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import api from '../../api'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false,
  },
  mode: {
    type: String,
    default: 'webhook',
  },
  item: {
    type: Object,
    default: null,
  },
  hosts: {
    type: Array,
    default: () => [],
  },
  saving: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
  // Pre-fill for Docker tracker created from Docker page
  prefillDockerImage: {
    type: String,
    default: '',
  },
  prefillDockerTag: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'submit'])

const customTasks = ref([])
const validationError = ref('')

const gitEnvVars = [
  { name: 'SS_REPO_NAME',    desc: 'owner/repo (ex: home-assistant/core)' },
  { name: 'SS_TAG_NAME',     desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
  { name: 'SS_RELEASE_URL',  desc: 'URL de la release sur le provider' },
  { name: 'SS_RELEASE_NAME', desc: 'Titre de la release' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const dockerEnvVars = [
  { name: 'SS_IMAGE_NAME',    desc: 'image:tag surveille (ex: nginx:latest)' },
  { name: 'SS_IMAGE_TAG',     desc: 'Tag surveille (ex: latest)' },
  { name: 'SS_IMAGE_VERSION', desc: 'Version resolue si tag=latest (ex: 1.27.3) ou identique a SS_IMAGE_TAG' },
  { name: 'SS_OLD_DIGEST',    desc: 'Digest manifest SHA256 precedent' },
  { name: 'SS_NEW_DIGEST',    desc: 'Nouveau digest manifest SHA256' },
  { name: 'SS_TRACKER_NAME',  desc: 'Nom du tracker dans ServerSupervisor' },
]

const currentEnvVars = computed(() =>
  form.value.tracker_type === 'docker' ? dockerEnvVars : gitEnvVars
)

const defaultWebhookForm = () => ({
  name: '',
  provider: 'github',
  event_filter: 'push',
  repo_filter: '',
  branch_filter: '',
  host_id: '',
  custom_task_id: '',
  notify_channels: [],
  notify_on_success: false,
  notify_on_failure: true,
  enabled: true,
})

const defaultTrackerForm = () => ({
  name: '',
  tracker_type: props.prefillDockerImage ? 'docker' : 'git',
  provider: 'github',
  repo_owner: '',
  repo_name: '',
  docker_image: props.prefillDockerImage || '',
  docker_tag: props.prefillDockerTag || 'latest',
  host_id: '',
  custom_task_id: '',
  dispatch_task: false,
  notify_channels: [],
  notify_on_release: true,
  enabled: true,
})

const form = ref(defaultWebhookForm())

const title = computed(() => {
  if (props.mode === 'tracker') {
    if (props.item) return 'Modifier le tracker'
    return form.value.tracker_type === 'docker' ? 'Nouveau tracker Docker' : 'Nouveau tracker de release Git'
  }
  return props.item ? 'Modifier le webhook' : 'Nouveau webhook Git'
})

const submitLabel = computed(() => (props.item ? 'Mettre a jour' : 'Creer'))
const errorMessage = computed(() => validationError.value || props.error)

watch(
  () => [props.visible, props.item, props.mode],
  async () => {
    if (!props.visible) return
    validationError.value = ''
    hydrateForm()
    await loadCustomTasks(form.value.host_id)
  },
  { immediate: true, deep: true }
)

watch(
  () => form.value.host_id,
  async (hostId) => {
    if (!props.visible) return
    await loadCustomTasks(hostId)
  }
)

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      document.addEventListener('keydown', onKeyDown)
      return
    }
    document.removeEventListener('keydown', onKeyDown)
  },
  { immediate: true }
)

onUnmounted(() => {
  document.removeEventListener('keydown', onKeyDown)
})

function hydrateForm() {
  if (props.mode === 'tracker') {
    form.value = props.item
      ? {
          name: props.item.name,
          tracker_type: props.item.tracker_type || 'git',
          provider: props.item.provider,
          repo_owner: props.item.repo_owner,
          repo_name: props.item.repo_name,
          docker_image: props.item.docker_image || '',
          docker_tag: props.item.docker_tag || 'latest',
          host_id: props.item.host_id || '',
          custom_task_id: props.item.custom_task_id || '',
          dispatch_task: !!(props.item.host_id && props.item.custom_task_id),
          notify_channels: [...(props.item.notify_channels || [])],
          notify_on_release: props.item.notify_on_release,
          enabled: props.item.enabled,
        }
      : defaultTrackerForm()
    return
  }

  form.value = props.item
    ? {
        name: props.item.name,
        provider: props.item.provider,
        event_filter: props.item.event_filter,
        repo_filter: props.item.repo_filter,
        branch_filter: props.item.branch_filter,
        host_id: props.item.host_id,
        custom_task_id: props.item.custom_task_id,
        notify_channels: [...(props.item.notify_channels || [])],
        notify_on_success: props.item.notify_on_success,
        notify_on_failure: props.item.notify_on_failure,
        enabled: props.item.enabled,
      }
    : defaultWebhookForm()
}

async function loadCustomTasks(hostId) {
  if (!hostId) {
    customTasks.value = []
    return
  }
  try {
    const response = await api.getHostCustomTasks(hostId)
    customTasks.value = Array.isArray(response.data) ? response.data : []
  } catch {
    customTasks.value = []
  }
}

function validate() {
  if (props.mode === 'tracker') {
    if (!form.value.name) return 'Le nom est obligatoire.'
    if (form.value.tracker_type === 'git') {
      if (!form.value.repo_owner || !form.value.repo_name) {
        return 'Owner et depot sont obligatoires pour un tracker Git.'
      }
      if (!form.value.host_id || !form.value.custom_task_id) {
        return 'VM cible et ID de tache sont obligatoires pour un tracker Git.'
      }
    } else {
      if (!form.value.docker_image) {
        return "L'image Docker est obligatoire pour un tracker Docker."
      }
      if (form.value.dispatch_task && (!form.value.host_id || !form.value.custom_task_id)) {
        return 'VM cible et ID de tache sont obligatoires si le declenchement de tache est active.'
      }
    }
    return ''
  }

  if (!form.value.name || !form.value.host_id || !form.value.custom_task_id) {
    return 'Nom, VM cible et ID de tache sont obligatoires.'
  }
  return ''
}

function submit() {
  validationError.value = validate()
  if (validationError.value) return
  const payload = { ...form.value }
  // For Docker trackers in monitor-only mode, clear host/task before sending
  if (props.mode === 'tracker' && payload.tracker_type === 'docker' && !payload.dispatch_task) {
    payload.host_id = ''
    payload.custom_task_id = ''
  }
  delete payload.dispatch_task
  emit('submit', payload)
}

function close() {
  validationError.value = ''
  emit('close')
}

function onKeyDown(event) {
  if (event.key === 'Escape' && props.visible) close()
}
</script>

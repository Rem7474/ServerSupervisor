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

          <template v-if="mode === 'tracker'">
            <div class="alert alert-info small mb-3">
              Le tracker surveille automatiquement les nouvelles releases d'un depot externe et declenche un script sur une VM des qu'une nouvelle version est publiee.
            </div>
          </template>

          <div class="row g-3">
            <div class="col-12">
              <label class="form-label required">Nom</label>
              <input v-model="form.name" type="text" class="form-control" :placeholder="mode === 'webhook' ? 'ex: Deploy mon-app' : 'ex: Mise a jour Home Assistant'" />
            </div>

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

            <template v-else>
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

            <div class="col-md-6">
              <label class="form-label required">VM cible</label>
              <select class="form-select" v-model="form.host_id">
                <option value="">-- Selectionner un hote --</option>
                <option v-for="host in hosts" :key="host.id" :value="host.id">{{ host.name }}</option>
              </select>
            </div>
            <div class="col-md-6">
              <label class="form-label required">Tache (tasks.yaml)</label>
              <select v-if="customTasks.length" class="form-select" v-model="form.custom_task_id">
                <option value="" disabled>-- Selectionner une tache --</option>
                <option v-for="task in customTasks" :key="task.id" :value="task.id">{{ task.name }} ({{ task.id }})</option>
              </select>
              <input v-else type="text" class="form-control" v-model="form.custom_task_id" :placeholder="mode === 'webhook' ? 'ex: deploy-mon-app' : 'ex: update-home-assistant'" />
              <div class="form-hint">Correspond a l'<code>id</code> dans <code>tasks.yaml</code> de l'agent.</div>
            </div>

            <template v-if="mode === 'tracker'">
              <div class="col-12">
                <label class="form-label">Image Docker suivie <span class="text-muted small">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="form.docker_image" placeholder="ex: homeassistant/home-assistant" />
                <div class="form-hint">Si renseigne, la version du conteneur tournant sera comparee au dernier tag sur le dashboard.</div>
              </div>
            </template>

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
                  <span class="form-check-label">Notifier a chaque nouvelle release</span>
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

          <div v-if="mode === 'tracker'" class="mt-3 pt-3 border-top">
            <div class="text-muted small mb-2">Variables injectees dans votre script :</div>
            <div class="table-responsive">
              <table class="table table-sm mb-0">
                <tbody>
                  <tr v-for="variable in trackerEnvVars" :key="variable.name">
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
})

const emit = defineEmits(['close', 'submit'])

const customTasks = ref([])
const validationError = ref('')

const trackerEnvVars = [
  { name: 'SS_REPO_NAME', desc: 'owner/repo (ex: home-assistant/core)' },
  { name: 'SS_TAG_NAME', desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
  { name: 'SS_RELEASE_URL', desc: 'URL de la release sur le provider' },
  { name: 'SS_RELEASE_NAME', desc: 'Titre de la release' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

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
  provider: 'github',
  repo_owner: '',
  repo_name: '',
  docker_image: '',
  host_id: '',
  custom_task_id: '',
  notify_channels: [],
  notify_on_release: true,
  enabled: true,
})

const form = ref(defaultWebhookForm())

const title = computed(() => {
  if (props.mode === 'tracker') {
    return props.item ? 'Modifier le tracker' : 'Nouveau tracker de release'
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
          provider: props.item.provider,
          repo_owner: props.item.repo_owner,
          repo_name: props.item.repo_name,
          docker_image: props.item.docker_image || '',
          host_id: props.item.host_id,
          custom_task_id: props.item.custom_task_id,
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
    if (!form.value.name || !form.value.repo_owner || !form.value.repo_name) {
      return 'Nom, owner et depot sont obligatoires.'
    }
    if (!form.value.host_id || !form.value.custom_task_id) {
      return 'VM cible et ID de tache sont obligatoires.'
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
  emit('submit', { ...form.value })
}

function close() {
  validationError.value = ''
  emit('close')
}

function onKeyDown(event) {
  if (event.key === 'Escape' && props.visible) close()
}
</script>
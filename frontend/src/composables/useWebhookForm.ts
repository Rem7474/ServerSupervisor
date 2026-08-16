import { computed, ref, watch, type Ref } from 'vue'
import api from '../api'
import type { CustomTaskSummary } from '../types/task'

export interface Host {
  id: string
  name?: string
}

// The webhook/tracker form only needs id + name; reuse the shared task type.
export type CustomTask = CustomTaskSummary

export interface RegistryCredential {
  id: string
  name: string
  registry_host?: string
}

/**
 * A running container offered by the docker tracker's container picker, which
 * replaced the free-text image/tag inputs: the ambient version engine already
 * knows every running image, so a tracker is configured by pointing at a real
 * container rather than by retyping its image reference.
 */
export interface PickableContainer {
  host_id: string
  host_name?: string
  container_name?: string
  image: string
  image_tag?: string
  compose_project?: string
  compose_service?: string
  tracked?: boolean
}

export interface WebhookItem {
  name?: string
  tracker_type?: string
  provider?: string
  event_filter?: string
  repo_filter?: string
  branch_filter?: string
  repo_owner?: string
  repo_name?: string
  docker_image?: string
  docker_tag?: string
  host_id?: string
  custom_task_id?: string
  cooldown_hours?: number
  notify_channels?: string[]
  notify_on_release?: boolean
  notify_on_success?: boolean
  notify_on_failure?: boolean
  enabled?: boolean
  update_action?: string
  compose_project?: string
  compose_service?: string
  pre_update_task_id?: string
  post_update_task_id?: string
  cleanup_after_update?: boolean
  healthcheck_timeout_sec?: number
  rollback_on_failure?: boolean
  registry_credentials_id?: string
  reconcile_drift?: boolean
}

export interface WebhookFormData {
  name: string
  provider?: string
  event_filter?: string
  repo_filter?: string
  branch_filter?: string
  tracker_type?: string
  repo_owner?: string
  repo_name?: string
  docker_image?: string
  docker_tag?: string
  host_id?: string
  custom_task_id?: string
  cooldown_hours?: number
  dispatch_task?: boolean
  notify_channels: string[]
  notify_on_release?: boolean
  notify_on_success?: boolean
  notify_on_failure?: boolean
  enabled: boolean
  update_action?: string
  compose_project?: string
  compose_service?: string
  pre_update_task_id?: string
  post_update_task_id?: string
  cleanup_after_update?: boolean
  healthcheck_timeout_sec?: number
  rollback_on_failure?: boolean
  registry_credentials_id?: string
  reconcile_drift?: boolean
}

const gitEnvVars = [
  { name: 'SS_REPO_NAME', desc: 'owner/repo (ex: home-assistant/core)' },
  { name: 'SS_TAG_NAME', desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
  { name: 'SS_RELEASE_URL', desc: 'URL de la release sur le provider' },
  { name: 'SS_RELEASE_NAME', desc: 'Titre de la release' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const dockerEnvVars = [
  { name: 'SS_IMAGE_NAME', desc: 'image:tag surveille (ex: nginx:latest)' },
  { name: 'SS_IMAGE_TAG', desc: 'Tag surveille (ex: latest)' },
  { name: 'SS_IMAGE_VERSION', desc: 'Version exacte resolue a partir du digest (ex: 4.4.1), ou identique a SS_IMAGE_TAG si non resolue' },
  { name: 'SS_OLD_DIGEST', desc: 'Digest manifest SHA256 precedent' },
  { name: 'SS_NEW_DIGEST', desc: 'Nouveau digest manifest SHA256' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

// WebhookFormProps mirrors the subset of WebhookModal props the form logic reads.
export interface WebhookFormProps {
  visible: boolean
  mode: string
  item: WebhookItem | null
  error: string
  prefillDockerImage: string
  prefillDockerTag: string
  prefillComposeProject: string
}

/**
 * useWebhookForm owns the WebhookModal form state and behaviour (defaults,
 * hydration from an edited item, host-scoped custom-task / registry-credential
 * loading, validation and payload normalisation on submit). The component keeps
 * only DOM concerns (focus trap, escape handling) and the markup.
 */
export function useWebhookForm(props: WebhookFormProps, onSubmit: (payload: WebhookFormData) => void) {
  const customTasks = ref<CustomTask[]>([])
  const registryCredentials = ref<RegistryCredential[]>([])
  const pickableContainers = ref<PickableContainer[]>([])
  const validationError = ref('')

  const defaultWebhookForm = (): WebhookFormData => ({
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

  const defaultTrackerForm = (): WebhookFormData => ({
    name: '',
    tracker_type: props.prefillDockerImage ? 'docker' : 'git',
    provider: 'github',
    repo_owner: '',
    repo_name: '',
    docker_image: props.prefillDockerImage || '',
    docker_tag: props.prefillDockerTag || 'latest',
    host_id: '',
    custom_task_id: '',
    cooldown_hours: 0,
    dispatch_task: props.prefillComposeProject ? true : (props.prefillDockerImage ? false : true),
    notify_channels: [],
    notify_on_release: true,
    enabled: true,
    update_action: props.prefillComposeProject ? 'compose' : 'custom',
    compose_project: props.prefillComposeProject || '',
    compose_service: '',
    pre_update_task_id: '',
    post_update_task_id: '',
    cleanup_after_update: false,
    healthcheck_timeout_sec: 0,
    rollback_on_failure: false,
    registry_credentials_id: '',
    reconcile_drift: false,
  })

  const form: Ref<WebhookFormData> = ref(defaultWebhookForm())

  const currentEnvVars = computed(() =>
    form.value.tracker_type === 'docker' ? dockerEnvVars : gitEnvVars,
  )

  const isComposeMode = computed(() =>
    props.mode === 'tracker' &&
    form.value.dispatch_task &&
    form.value.tracker_type === 'docker' &&
    form.value.update_action === 'compose',
  )

  const title = computed(() => {
    if (props.mode === 'tracker') {
      if (props.item) return 'Modifier le tracker'
      return form.value.tracker_type === 'docker' ? 'Nouveau tracker Docker' : 'Nouveau tracker de release Git'
    }
    return props.item ? 'Modifier le webhook' : 'Nouveau webhook Git'
  })

  const submitLabel = computed(() => (props.item ? 'Mettre a jour' : 'Creer'))
  const errorMessage = computed(() => validationError.value || props.error)

  function hydrateForm(): void {
    if (props.mode === 'tracker') {
      form.value = props.item
        ? {
            name: props.item.name ?? '',
            tracker_type: props.item.tracker_type || 'git',
            provider: props.item.provider,
            repo_owner: props.item.repo_owner,
            repo_name: props.item.repo_name,
            docker_image: props.item.docker_image || '',
            docker_tag: props.item.docker_tag || 'latest',
            host_id: props.item.host_id || '',
            custom_task_id: props.item.custom_task_id || '',
            cooldown_hours: Number.isFinite(props.item.cooldown_hours) ? props.item.cooldown_hours : 0,
            dispatch_task: !!(props.item.host_id && (props.item.custom_task_id || props.item.compose_project)),
            notify_channels: [...(props.item.notify_channels || [])],
            notify_on_release: props.item.notify_on_release,
            enabled: props.item.enabled ?? true,
            update_action: props.item.update_action || 'custom',
            compose_project: props.item.compose_project || '',
            compose_service: props.item.compose_service || '',
            pre_update_task_id: props.item.pre_update_task_id || '',
            post_update_task_id: props.item.post_update_task_id || '',
            cleanup_after_update: !!props.item.cleanup_after_update,
            healthcheck_timeout_sec: Number.isFinite(props.item.healthcheck_timeout_sec) ? props.item.healthcheck_timeout_sec : 0,
            rollback_on_failure: !!props.item.rollback_on_failure,
            registry_credentials_id: props.item.registry_credentials_id || '',
            reconcile_drift: !!props.item.reconcile_drift,
          }
        : defaultTrackerForm()
      return
    }

    form.value = props.item
      ? {
          name: props.item.name ?? '',
          provider: props.item.provider,
          event_filter: props.item.event_filter,
          repo_filter: props.item.repo_filter,
          branch_filter: props.item.branch_filter,
          host_id: props.item.host_id,
          custom_task_id: props.item.custom_task_id,
          notify_channels: [...(props.item.notify_channels || [])],
          notify_on_success: props.item.notify_on_success,
          notify_on_failure: props.item.notify_on_failure,
          enabled: props.item.enabled ?? true,
        }
      : defaultWebhookForm()
  }

  async function loadCustomTasks(hostId: string | undefined): Promise<void> {
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

  async function loadRegistryCredentials(): Promise<void> {
    try {
      const response = await api.getRegistryCredentials()
      registryCredentials.value = Array.isArray(response.data?.credentials) ? response.data.credentials : []
    } catch {
      registryCredentials.value = []
    }
  }

  async function loadPickableContainers(): Promise<void> {
    try {
      const response = await api.getPickableContainers()
      pickableContainers.value = Array.isArray(response.data?.containers) ? response.data.containers : []
    } catch {
      pickableContainers.value = []
    }
  }

  // ===== docker container picker =====
  //
  // Replaces the free-text image/tag inputs: a docker tracker now points at a
  // real running container. The image reference is still what gets stored (the
  // update action needs to know exactly what it targets) — it is just derived
  // from the selection instead of retyped.

  /** Host whose containers the picker lists. Not necessarily the dispatch target yet. */
  const containerSourceHostId = ref('')

  const containerHosts = computed(() => {
    const seen = new Map<string, string>()
    for (const c of pickableContainers.value) {
      if (!seen.has(c.host_id)) seen.set(c.host_id, c.host_name || c.host_id)
    }
    return [...seen].map(([id, name]) => ({ id, name })).sort((a, b) => a.name.localeCompare(b.name))
  })

  const containersForHost = computed(() =>
    pickableContainers.value.filter((c) => c.host_id === containerSourceHostId.value),
  )

  function containerKey(c: PickableContainer): string {
    return `${c.host_id}|${c.image}|${c.image_tag || 'latest'}`
  }

  const selectedContainerKey = computed(() =>
    form.value.docker_image
      ? `${containerSourceHostId.value}|${form.value.docker_image}|${form.value.docker_tag || 'latest'}`
      : '',
  )

  /**
   * True when the tracker already targets an image no running container
   * exposes (container stopped, host removed, tracker created before the
   * picker existed). The form keeps that value instead of silently dropping
   * it, and the picker offers it as an explicit extra option.
   */
  const selectedContainerMissing = computed(() =>
    !!form.value.docker_image &&
    !containersForHost.value.some((c) => containerKey(c) === selectedContainerKey.value),
  )

  function selectContainer(key: string): void {
    const picked = containersForHost.value.find((c) => containerKey(c) === key)
    if (!picked) return
    form.value.docker_image = picked.image
    form.value.docker_tag = picked.image_tag || 'latest'
    // The container's own host is the only sensible dispatch target, and its
    // compose coordinates are what a compose-mode update would act on.
    form.value.host_id = picked.host_id
    if (picked.compose_project) {
      form.value.compose_project = picked.compose_project
      form.value.compose_service = picked.compose_service || ''
    }
    if (!form.value.name) {
      form.value.name = picked.container_name || picked.image
    }
  }

  function validate(): string {
    if (props.mode === 'tracker') {
      if (!form.value.name) return 'Le nom est obligatoire.'
      if ((form.value.cooldown_hours ?? 0) < 0 || (form.value.cooldown_hours ?? 0) > 168) {
        return 'Le cooldown doit etre compris entre 0 et 168 heures.'
      }
      if (form.value.tracker_type === 'git') {
        if (!form.value.repo_owner || !form.value.repo_name) {
          return 'Owner et depot sont obligatoires pour un tracker Git.'
        }
        if (form.value.dispatch_task && (!form.value.host_id || !form.value.custom_task_id)) {
          return 'VM cible et ID de tâche sont obligatoires si le déclenchement de tâche est activé.'
        }
      } else {
        if (!form.value.docker_image) {
          return 'Sélectionnez le conteneur à suivre.'
        }
        if ((form.value.repo_owner && !form.value.repo_name) || (!form.value.repo_owner && form.value.repo_name)) {
          return 'Pour le repo Git lie, renseignez owner et depot ensemble (ou laissez les deux vides).'
        }
        if (form.value.dispatch_task) {
          if (form.value.update_action === 'compose') {
            if (!form.value.host_id || !form.value.compose_project) {
              return 'VM cible et projet compose sont obligatoires en mode Compose.'
            }
          } else if (!form.value.host_id || !form.value.custom_task_id) {
            return 'VM cible et ID de tâche sont obligatoires si le déclenchement de tâche est activé.'
          }
        }
      }
      return ''
    }

    if (!form.value.name || !form.value.host_id || !form.value.custom_task_id) {
      return 'Nom, VM cible et ID de tache sont obligatoires.'
    }
    return ''
  }

  function submit(): void {
    validationError.value = validate()
    if (validationError.value) return
    const payload: WebhookFormData = { ...form.value }
    if (props.mode === 'tracker' && (!Array.isArray(payload.notify_channels) || payload.notify_channels.length === 0)) {
      payload.notify_on_release = false
    }
    if (props.mode === 'tracker') {
      // Monitor-only: clear all dispatch targets and reset to custom so the
      // backend's compose CHECK constraint (host+project required) is not hit.
      if (!payload.dispatch_task) {
        payload.host_id = ''
        payload.custom_task_id = ''
        payload.update_action = 'custom'
        payload.compose_project = ''
        payload.compose_service = ''
      } else if (payload.update_action === 'compose') {
        // Compose mode does not use a tasks.yaml command target.
        payload.custom_task_id = ''
      } else {
        // Custom mode does not use compose fields.
        payload.compose_project = ''
        payload.compose_service = ''
        payload.pre_update_task_id = ''
        payload.post_update_task_id = ''
        payload.reconcile_drift = false
      }
      // Git trackers never use the compose path or registry credentials.
      if (payload.tracker_type !== 'docker') {
        payload.update_action = 'custom'
        payload.registry_credentials_id = ''
      }
    }
    delete payload.dispatch_task
    onSubmit(payload)
  }

  function clearError(): void {
    validationError.value = ''
  }

  // Re-hydrate + load dependent data whenever the modal opens or its target changes.
  watch(
    () => [props.visible, props.item, props.mode],
    async () => {
      if (!props.visible) return
      validationError.value = ''
      hydrateForm()
      // The picker starts on the tracker's own host so an existing tracker
      // opens with its container already selected.
      containerSourceHostId.value = form.value.host_id || ''
      await loadCustomTasks(form.value.host_id)
      if (props.mode === 'tracker') {
        await Promise.all([loadRegistryCredentials(), loadPickableContainers()])
        // A tracker created before the picker existed (or a monitor-only one)
        // has no host_id: fall back to the host actually running its image so
        // the selection isn't lost on edit.
        if (!containerSourceHostId.value && form.value.docker_image) {
          const owner = pickableContainers.value.find((c) => c.image === form.value.docker_image)
          if (owner) containerSourceHostId.value = owner.host_id
        }
      }
    },
    { immediate: true, deep: true },
  )

  watch(
    () => form.value.host_id,
    async (hostId) => {
      if (!props.visible) return
      await loadCustomTasks(hostId)
    },
  )

  watch(
    () => form.value.notify_channels,
    (channels) => {
      if (props.mode !== 'tracker') return
      if (!Array.isArray(channels) || channels.length === 0) {
        form.value.notify_on_release = false
      }
    },
    { deep: true },
  )

  return {
    form,
    customTasks,
    registryCredentials,
    pickableContainers,
    containerSourceHostId,
    containerHosts,
    containersForHost,
    containerKey,
    selectedContainerKey,
    selectedContainerMissing,
    selectContainer,
    validationError,
    currentEnvVars,
    isComposeMode,
    title,
    submitLabel,
    errorMessage,
    submit,
    clearError,
  }
}

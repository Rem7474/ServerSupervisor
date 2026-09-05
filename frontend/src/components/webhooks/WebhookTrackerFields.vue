<template>
  <!-- Type selector -->
  <div class="col-12">
    <div class="form-label required">
      {{ t('webhooks.trackingTypeLabel') }}
    </div>
    <div class="row g-2">
      <div class="col-6">
        <label
          class="tracker-type-card"
          :class="form.tracker_type === 'git' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
        >
          <input
            v-model="trackerTypeModel"
            class="tracker-type-input"
            type="radio"
            value="git"
          >
          <span>
            <span class="fw-semibold d-block">{{ t('webhooks.gitReleaseBadge') }}</span>
            <span class="text-muted small">{{ t('webhooks.gitReleaseDescription') }}</span>
          </span>
        </label>
      </div>
      <div class="col-6">
        <label
          class="tracker-type-card"
          :class="form.tracker_type === 'docker' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
        >
          <input
            v-model="trackerTypeModel"
            class="tracker-type-input"
            type="radio"
            value="docker"
          >
          <span>
            <span class="fw-semibold d-block">{{ t('webhooks.dockerImageBadge') }}</span>
            <span class="text-muted small">{{ t('webhooks.dockerImageDescription') }}</span>
          </span>
        </label>
      </div>
    </div>
  </div>

  <!-- Git-specific fields -->
  <template v-if="form.tracker_type === 'git'">
    <div class="col-md-4">
      <label
        for="webhook-tracker-provider"
        class="form-label required"
      >Provider</label>
      <select
        id="webhook-tracker-provider"
        v-model="providerModel"
        class="form-select"
      >
        <option value="github">
          GitHub
        </option>
        <option value="gitlab">
          GitLab
        </option>
        <option value="gitea">
          Gitea (Codeberg)
        </option>
      </select>
    </div>
    <div class="col-md-4">
      <label
        for="webhook-tracker-repo-owner"
        class="form-label required"
      >Owner / Org</label>
      <input
        id="webhook-tracker-repo-owner"
        v-model="repoOwnerModel"
        type="text"
        class="form-control"
        placeholder="ex: home-assistant"
      >
    </div>
    <div class="col-md-4">
      <label
        for="webhook-tracker-repo-name"
        class="form-label required"
      >{{ t('webhooks.repositoryLabel') }}</label>
      <input
        id="webhook-tracker-repo-name"
        v-model="repoNameModel"
        type="text"
        class="form-control"
        placeholder="ex: core"
      >
    </div>
  </template>

  <!-- Docker-specific fields: the image reference comes from a running
       container, never from free text — the ambient engine already knows every
       running image, and typing one by hand was the main source of trackers
       that silently watched an image nobody was running. -->
  <template v-else>
    <div class="col-md-5">
      <label
        for="webhook-tracker-source-host"
        class="form-label required"
      >{{ t('webhooks.hostColumn') }}</label>
      <select
        id="webhook-tracker-source-host"
        v-model="containerSourceHostId"
        class="form-select"
      >
        <option value="">
          {{ t('webhooks.selectHostPlaceholder') }}
        </option>
        <option
          v-for="h in containerHosts"
          :key="h.id"
          :value="h.id"
        >
          {{ h.name }}
        </option>
      </select>
      <div class="form-hint">
        {{ t('webhooks.hostRunningContainerHint') }}
      </div>
    </div>
    <div class="col-md-7">
      <label
        for="webhook-tracker-container"
        class="form-label required"
      >{{ t('webhooks.containerLabel') }}</label>
      <select
        id="webhook-tracker-container"
        class="form-select"
        :disabled="!containerSourceHostId"
        :value="selectedContainerKey"
        aria-describedby="docker-container-hint"
        @change="onContainerChange"
      >
        <option value="">
          {{ containerSourceHostId ? t('webhooks.selectContainerPlaceholder') : t('webhooks.selectHostFirstPlaceholder') }}
        </option>
        <option
          v-if="selectedContainerMissing"
          :value="selectedContainerKey"
        >
          {{ form.docker_image }}:{{ form.docker_tag || 'latest' }} {{ t('webhooks.noRunningContainerOption') }}
        </option>
        <option
          v-for="c in containersForHost"
          :key="containerKey(c)"
          :value="containerKey(c)"
        >
          {{ c.container_name || c.image }} — {{ c.image }}:{{ c.image_tag || 'latest' }}{{ c.tracked ? ' ' + t('webhooks.alreadyTrackedSuffix') : '' }}
        </option>
      </select>
      <div
        id="docker-container-hint"
        class="form-hint"
      >
        <template v-if="form.docker_image">
          {{ t('webhooks.watchedImageHintPrefix') }} <code>{{ form.docker_image }}:{{ form.docker_tag || 'latest' }}</code>
        </template>
        <template v-else>
          {{ t('webhooks.imageTagDeducedFromContainerHint') }}
        </template>
      </div>
    </div>

    <div class="col-md-6">
      <label
        for="webhook-tracker-registry-credentials"
        class="form-label"
      >{{ t('webhooks.privateRegistryLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
      <select
        id="webhook-tracker-registry-credentials"
        v-model="registryCredentialsIdModel"
        class="form-select"
      >
        <option value="">
          {{ t('webhooks.publicNoAuthOption') }}
        </option>
        <option
          v-for="cred in registryCredentials"
          :key="cred.id"
          :value="cred.id"
        >
          {{ cred.name }} ({{ cred.registry_host }})
        </option>
      </select>
      <div class="form-hint">
        {{ t('webhooks.registryCredentialsHint') }}
      </div>
    </div>

    <div class="col-12">
      <div class="border rounded p-2">
        <div class="fw-medium mb-2">
          {{ t('webhooks.linkedGitRepoLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span>
        </div>
        <div class="form-hint mb-2">
          {{ t('webhooks.linkedRepoInfoHint') }}
        </div>
        <div class="row g-2">
          <div class="col-md-4">
            <label
              for="webhook-tracker-linked-provider"
              class="form-label"
            >Provider</label>
            <select
              id="webhook-tracker-linked-provider"
              v-model="providerModel"
              class="form-select"
            >
              <option value="github">
                GitHub
              </option>
              <option value="gitlab">
                GitLab
              </option>
              <option value="gitea">
                Gitea (Codeberg)
              </option>
            </select>
          </div>
          <div class="col-md-4">
            <label
              for="webhook-tracker-linked-repo-owner"
              class="form-label"
            >Owner / Org</label>
            <input
              id="webhook-tracker-linked-repo-owner"
              v-model="repoOwnerModel"
              type="text"
              class="form-control"
              placeholder="ex: home-assistant"
            >
          </div>
          <div class="col-md-4">
            <label
              for="webhook-tracker-linked-repo-name"
              class="form-label"
            >{{ t('webhooks.repositoryLabel') }}</label>
            <input
              id="webhook-tracker-linked-repo-name"
              v-model="repoNameModel"
              type="text"
              class="form-control"
              placeholder="ex: core"
            >
          </div>
        </div>
      </div>
    </div>
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WebhookFormData, RegistryCredential, PickableContainer } from '../../composables/useWebhookForm'

const { t } = useI18n()

const props = defineProps<{
  form: WebhookFormData
  registryCredentials: RegistryCredential[]
  containerHosts: { id: string, name: string }[]
  containersForHost: PickableContainer[]
  containerKey: (c: PickableContainer) => string
  selectedContainerKey: string
  selectedContainerMissing: boolean
}>()

// containerSourceHostId lives in the composable (it drives the container list
// and is reset on hydrate), so it is bound through as an explicit v-model.
const containerSourceHostId = defineModel<string>('containerSourceHostId', { default: '' })

const emit = defineEmits<{
  (e: 'select-container', key: string): void
  (e: 'update:form', value: WebhookFormData): void
}>()

// The `form` prop is owned by the parent (WebhookModal, via
// useWebhookForm). This component never mutates it in place — every field
// write emits a whole-object replacement for the parent to apply (bound as
// `v-model:form` there).
function updateForm<K extends keyof WebhookFormData>(key: K, value: WebhookFormData[K]): void {
  emit('update:form', { ...props.form, [key]: value })
}

function fieldModel<K extends keyof WebhookFormData>(key: K) {
  return computed<WebhookFormData[K]>({
    get: () => props.form[key],
    set: (value) => updateForm(key, value),
  })
}

const trackerTypeModel = fieldModel('tracker_type')
const providerModel = fieldModel('provider')
const repoOwnerModel = fieldModel('repo_owner')
const repoNameModel = fieldModel('repo_name')
const registryCredentialsIdModel = fieldModel('registry_credentials_id')

function onContainerChange(event: Event): void {
  const key = (event.target as HTMLSelectElement).value
  // Re-selecting the "no container currently running" placeholder keeps the
  // stored image as-is; only a real container selection rewrites the form.
  if (key && key !== props.selectedContainerKey) emit('select-container', key)
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

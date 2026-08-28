<template>
  <!-- Type selector -->
  <div class="col-12">
    <div class="form-label required">
      Type de suivi
    </div>
    <div class="row g-2">
      <div class="col-6">
        <label
          class="tracker-type-card"
          :class="form.tracker_type === 'git' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
        >
          <input
            v-model="form.tracker_type"
            class="tracker-type-input"
            type="radio"
            value="git"
          >
          <span>
            <span class="fw-semibold d-block">Release Git</span>
            <span class="text-muted small">Surveille les nouvelles releases/tags sur GitHub, GitLab ou Gitea</span>
          </span>
        </label>
      </div>
      <div class="col-6">
        <label
          class="tracker-type-card"
          :class="form.tracker_type === 'docker' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
        >
          <input
            v-model="form.tracker_type"
            class="tracker-type-input"
            type="radio"
            value="docker"
          >
          <span>
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
      <label
        for="webhook-tracker-provider"
        class="form-label required"
      >Provider</label>
      <select
        id="webhook-tracker-provider"
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
        v-model="form.repo_owner"
        type="text"
        class="form-control"
        placeholder="ex: home-assistant"
      >
    </div>
    <div class="col-md-4">
      <label
        for="webhook-tracker-repo-name"
        class="form-label required"
      >Depot</label>
      <input
        id="webhook-tracker-repo-name"
        v-model="form.repo_name"
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
      >Hôte</label>
      <select
        id="webhook-tracker-source-host"
        v-model="containerSourceHostId"
        class="form-select"
      >
        <option value="">
          Sélectionner un hôte…
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
        Hôte sur lequel tourne le conteneur à suivre.
      </div>
    </div>
    <div class="col-md-7">
      <label
        for="webhook-tracker-container"
        class="form-label required"
      >Conteneur</label>
      <select
        id="webhook-tracker-container"
        class="form-select"
        :disabled="!containerSourceHostId"
        :value="selectedContainerKey"
        aria-describedby="docker-container-hint"
        @change="onContainerChange"
      >
        <option value="">
          {{ containerSourceHostId ? 'Sélectionner un conteneur…' : 'Choisissez d’abord un hôte' }}
        </option>
        <option
          v-if="selectedContainerMissing"
          :value="selectedContainerKey"
        >
          {{ form.docker_image }}:{{ form.docker_tag || 'latest' }} (aucun conteneur en cours)
        </option>
        <option
          v-for="c in containersForHost"
          :key="containerKey(c)"
          :value="containerKey(c)"
        >
          {{ c.container_name || c.image }} — {{ c.image }}:{{ c.image_tag || 'latest' }}{{ c.tracked ? ' (déjà suivi)' : '' }}
        </option>
      </select>
      <div
        id="docker-container-hint"
        class="form-hint"
      >
        <template v-if="form.docker_image">
          Image surveillée : <code>{{ form.docker_image }}:{{ form.docker_tag || 'latest' }}</code>
        </template>
        <template v-else>
          L'image et le tag surveillés sont déduits du conteneur choisi.
        </template>
      </div>
    </div>

    <div class="col-md-6">
      <label
        for="webhook-tracker-registry-credentials"
        class="form-label"
      >Registre privé <span class="text-muted">(optionnel)</span></label>
      <select
        id="webhook-tracker-registry-credentials"
        v-model="form.registry_credentials_id"
        class="form-select"
      >
        <option value="">
          Public (aucune authentification)
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
        Identifiants pour interroger une image sur un registre privé (GHCR, Harbor…).
      </div>
    </div>

    <div class="col-12">
      <div class="border rounded p-2">
        <div class="fw-medium mb-2">
          Dépôt Git lié <span class="text-muted">(optionnel)</span>
        </div>
        <div class="form-hint mb-2">
          Purement informatif : les notes de version du dépôt sont affichées à côté
          de l'historique des digests. La détection des mises à jour reste basée
          sur le registre Docker.
        </div>
        <div class="row g-2">
          <div class="col-md-4">
            <label
              for="webhook-tracker-provider"
              class="form-label"
            >Provider</label>
            <select
              id="webhook-tracker-provider"
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
                Gitea (Codeberg)
              </option>
            </select>
          </div>
          <div class="col-md-4">
            <label
              for="webhook-tracker-repo-owner"
              class="form-label"
            >Owner / Org</label>
            <input
              id="webhook-tracker-repo-owner"
              v-model="form.repo_owner"
              type="text"
              class="form-control"
              placeholder="ex: home-assistant"
            >
          </div>
          <div class="col-md-4">
            <label
              for="webhook-tracker-repo-name"
              class="form-label"
            >Depot</label>
            <input
              id="webhook-tracker-repo-name"
              v-model="form.repo_name"
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
import type { WebhookFormData, RegistryCredential, PickableContainer } from '../../composables/useWebhookForm'

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

const emit = defineEmits<{ (e: 'select-container', key: string): void }>()

function onContainerChange(event: Event): void {
  const key = (event.target as HTMLSelectElement).value
  // Re-selecting the "aucun conteneur en cours" placeholder keeps the stored
  // image as-is; only a real container selection rewrites the form.
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

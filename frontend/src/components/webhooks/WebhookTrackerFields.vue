<template>
  <!-- Type selector -->
  <div class="col-12">
    <label class="form-label required">Type de suivi</label>
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
          Gitea (Codeberg)
        </option>
      </select>
    </div>
    <div class="col-md-4">
      <label class="form-label required">Owner / Org</label>
      <input
        v-model="form.repo_owner"
        type="text"
        class="form-control"
        placeholder="ex: home-assistant"
      >
    </div>
    <div class="col-md-4">
      <label class="form-label required">Depot</label>
      <input
        v-model="form.repo_name"
        type="text"
        class="form-control"
        placeholder="ex: core"
      >
    </div>
  </template>

  <!-- Docker-specific fields -->
  <template v-else>
    <div class="col-md-8">
      <label class="form-label required">Image Docker</label>
      <input
        v-model="form.docker_image"
        type="text"
        class="form-control"
        placeholder="ex: homeassistant/home-assistant, nginx, ghcr.io/user/app"
        aria-describedby="docker-image-hint"
      >
      <div
        id="docker-image-hint"
        class="form-hint"
      >
        Nom de l'image sans le tag (registre Docker Hub par defaut).
      </div>
    </div>
    <div class="col-md-4">
      <label class="form-label required">Tag surveille</label>
      <input
        v-model="form.docker_tag"
        type="text"
        class="form-control"
        placeholder="latest"
        aria-describedby="docker-tag-hint"
      >
      <div
        id="docker-tag-hint"
        class="form-hint"
      >
        Tag de l'image a surveiller.
      </div>
    </div>

    <div class="col-md-6">
      <label class="form-label">Registre privé <span class="text-muted">(optionnel)</span></label>
      <select
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
          Repo Git lie (optionnel, pour les release notes)
        </div>
        <div class="row g-2">
          <div class="col-md-4">
            <label class="form-label">Provider</label>
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
                Gitea (Codeberg)
              </option>
            </select>
          </div>
          <div class="col-md-4">
            <label class="form-label">Owner / Org</label>
            <input
              v-model="form.repo_owner"
              type="text"
              class="form-control"
              placeholder="ex: home-assistant"
            >
          </div>
          <div class="col-md-4">
            <label class="form-label">Depot</label>
            <input
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
import type { WebhookFormData, RegistryCredential } from '../../composables/useWebhookForm'

defineProps<{
  form: WebhookFormData
  registryCredentials: RegistryCredential[]
}>()
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

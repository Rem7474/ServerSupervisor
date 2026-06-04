<template>
  <!-- Env vars card -->
  <div class="card mt-3">
    <div class="card-header">
      <h3 class="card-title">
        Variables disponibles dans le script
      </h3>
    </div>
    <div class="card-body p-0">
      <div class="table-responsive">
        <table class="table table-sm table-vcenter mb-0">
          <tbody>
            <tr
              v-for="v in envVars"
              :key="v.name"
            >
              <td><code class="small">{{ v.name }}</code></td>
              <td class="text-muted small">
                {{ v.desc }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <!-- tasks.yaml snippet card -->
  <div
    v-if="tracker.host_id && !tracker.custom_task_id"
    class="card mt-3"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Exemple de script tasks.yaml
      </h3>
      <div class="d-flex align-items-center gap-2">
        <span
          v-if="detectedComposePath"
          class="badge bg-green-lt text-green"
          title="Chemin détecté depuis les projets Compose de l'hôte"
        >
          Chemin détecté automatiquement
        </span>
        <button
          class="btn btn-sm btn-ghost-secondary"
          :title="copied ? 'Copié !' : 'Copier'"
          @click="copySnippet"
        >
          <svg
            v-if="!copied"
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
          >
            <rect
              x="9"
              y="9"
              width="13"
              height="13"
              rx="2"
              ry="2"
            /><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
          </svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
        </button>
      </div>
    </div>
    <div class="card-body p-0">
      <div
        v-if="loadingSnippet"
        class="p-3"
      >
        <LoadingSkeleton
          variant="list"
          :lines="3"
        />
      </div>
      <template v-else>
        <div
          v-if="tasksYaml"
          class="px-3 pt-2 pb-0"
        >
          <p class="small text-muted mb-1">
            Contenu actuel de <code>tasks.yaml</code> sur l'hôte — ajoutez la tâche ci-dessous :
          </p>
          <pre
            class="bg-dark text-light rounded p-2 small"
            style="max-height:160px;overflow-y:auto;font-size:0.72rem;"
          >{{ tasksYaml }}</pre>
        </div>
        <div class="px-3 pt-2 pb-3">
          <p
            v-if="!tasksYaml"
            class="small text-muted mb-1"
          >
            Ajoutez cette tâche dans <code>/etc/serversupervisor/tasks.yaml</code> sur l'hôte :
          </p>
          <p
            v-else
            class="small text-muted mb-1"
          >
            Tâche à ajouter dans la section <code>tasks:</code> :
          </p>
          <pre
            class="bg-dark text-light rounded p-2 small mb-0"
            style="font-size:0.72rem;"
          >{{ generatedSnippet }}</pre>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const props = defineProps<{
  tracker: any
  composeProjects: any[]
  tasksYaml: string
  loadingSnippet: boolean
}>()

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
  { name: 'SS_OLD_DIGEST', desc: 'Digest manifest SHA256 precedent' },
  { name: 'SS_NEW_DIGEST', desc: 'Nouveau digest manifest SHA256' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const envVars = computed(() =>
  props.tracker?.tracker_type === 'docker' ? dockerEnvVars : gitEnvVars,
)

// Find the compose project whose raw_config references the tracked Docker image.
const detectedComposePath = computed(() => {
  const t = props.tracker
  if (!t || t.tracker_type !== 'docker' || !t.docker_image) return null
  const imageName = t.docker_image.split(':')[0].toLowerCase()
  for (const p of props.composeProjects) {
    const raw = (p.raw_config || '').toLowerCase()
    if (raw.includes(imageName) && p.working_dir) {
      return p.working_dir
    }
  }
  return null
})

// Derive a safe task ID from the tracker name or image name.
const snippetTaskId = computed(() => {
  const t = props.tracker
  if (!t) return 'update-service'
  const base = (t.tracker_type === 'docker' ? t.docker_image?.split('/').pop()?.split(':')[0] : t.repo_name) || t.name
  return 'update-' + base.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 40)
})

// Build the YAML snippet tailored to the tracker type.
const generatedSnippet = computed(() => {
  const t = props.tracker
  if (!t) return ''
  const taskId = snippetTaskId.value

  if (t.tracker_type === 'docker') {
    const image = t.docker_image || 'mon-image'
    const path = detectedComposePath.value || '/opt/mon-projet'
    const name = t.name || image
    return `  - id: ${taskId}
    name: "Pull et redémarrage ${name}"
    command: ["bash", "-c", "cd ${path} && docker compose pull && docker compose down && docker compose up -d"]
    timeout: 3600`
  } else {
    const repo = t.repo_name || 'mon-app'
    const name = t.name || repo
    return `  - id: ${taskId}
    name: "Déploiement ${name}"
    command: ["bash", "-c", "echo 'Nouvelle release: $SS_TAG_NAME' && /opt/${repo}/deploy.sh"]
    timeout: 3600`
  }
})

const copied = ref(false)

function copySnippet(): void {
  navigator.clipboard?.writeText(generatedSnippet.value).then(() => {
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  })
}
</script>

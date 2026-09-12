<template>
  <!-- Env vars card -->
  <div class="card mt-3">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('webhooks.availableVarsInScriptTitle') }}
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
                {{ t(v.descKey) }}
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
        {{ t('webhooks.exampleTasksYamlTitle') }}
      </h3>
      <div class="d-flex align-items-center gap-2">
        <span
          v-if="detectedComposePath"
          class="badge bg-success-lt text-success"
          :title="t('webhooks.detectedPathTooltip')"
        >
          {{ t('webhooks.autoDetectedPathBadge') }}
        </span>
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :title="copied ? t('webhooks.copiedTooltip') : t('webhooks.copyTooltip')"
          @click="copySnippet"
        >
          <IconCopy
            v-if="!copied"
            :size="14"
          />
          <IconCheck
            v-else
            :size="14"
          />
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
            {{ t('webhooks.currentTasksYamlPrefix') }} <code>tasks.yaml</code> {{ t('webhooks.currentTasksYamlSuffix') }}
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
            {{ t('webhooks.addTaskInFilePrefix') }} <code>/etc/serversupervisor/tasks.yaml</code> {{ t('webhooks.addTaskInFileSuffix') }}
          </p>
          <p
            v-else
            class="small text-muted mb-1"
          >
            {{ t('webhooks.taskToAddInSection') }}
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
import { useI18n } from 'vue-i18n'
import { IconCheck, IconCopy } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import type { ReleaseTracker } from '../../types/tracker'
import type { ComposeProject } from '../../types/docker'

const { t } = useI18n()

const props = defineProps<{
  tracker: ReleaseTracker
  composeProjects: ComposeProject[]
  tasksYaml: string
  loadingSnippet: boolean
}>()

const gitEnvVars = [
  { name: 'SS_REPO_NAME', descKey: 'webhooks.repoOwnerDesc' },
  { name: 'SS_TAG_NAME', descKey: 'webhooks.newReleaseTagDesc' },
  { name: 'SS_RELEASE_URL', descKey: 'webhooks.releaseProviderUrlDesc' },
  { name: 'SS_RELEASE_NAME', descKey: 'webhooks.releaseTitleDesc' },
  { name: 'SS_TRACKER_NAME', descKey: 'webhooks.trackerNameDesc' },
]

const dockerEnvVars = [
  { name: 'SS_IMAGE_NAME', descKey: 'webhooks.watchedImageTagDesc' },
  { name: 'SS_IMAGE_TAG', descKey: 'webhooks.watchedTagDesc' },
  { name: 'SS_OLD_DIGEST', descKey: 'webhooks.oldDigestDesc' },
  { name: 'SS_NEW_DIGEST', descKey: 'webhooks.newDigestDesc' },
  { name: 'SS_TRACKER_NAME', descKey: 'webhooks.trackerNameDesc' },
]

const envVars = computed(() =>
  props.tracker?.tracker_type === 'docker' ? dockerEnvVars : gitEnvVars,
)

// Find the compose project whose raw_config references the tracked Docker image.
const detectedComposePath = computed(() => {
  const tracker = props.tracker
  if (!tracker || tracker.tracker_type !== 'docker' || !tracker.docker_image) return null
  const imageName = tracker.docker_image.split(':')[0].toLowerCase()
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
  const tracker = props.tracker
  if (!tracker) return 'update-service'
  const base = (tracker.tracker_type === 'docker' ? tracker.docker_image?.split('/').pop()?.split(':')[0] : tracker.repo_name) || tracker.name
  return 'update-' + base.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 40)
})

// Build the YAML snippet tailored to the tracker type.
const generatedSnippet = computed(() => {
  const tracker = props.tracker
  if (!tracker) return ''
  const taskId = snippetTaskId.value

  if (tracker.tracker_type === 'docker') {
    const image = tracker.docker_image || t('webhooks.exampleImagePlaceholder')
    const path = detectedComposePath.value || `/opt/${t('webhooks.exampleProjectPlaceholder')}`
    const name = tracker.name || image
    return `  - id: ${taskId}
    name: "${t('webhooks.pullAndRestartTaskName', { name })}"
    command: ["bash", "-c", "cd ${path} && docker compose pull && docker compose down && docker compose up -d"]
    timeout: 3600`
  } else {
    const repo = tracker.repo_name || t('webhooks.exampleAppPlaceholder')
    const name = tracker.name || repo
    return `  - id: ${taskId}
    name: "${t('webhooks.deploymentTaskName', { name })}"
    command: ["bash", "-c", "echo '${t('webhooks.newReleaseEchoPrefix')}: $SS_TAG_NAME' && /opt/${repo}/deploy.sh"]
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

<template>
  <div
    v-if="visible"
    ref="modalRef"
    class="modal modal-blur show d-block"
    style="background:rgba(0,0,0,.5)"
    role="dialog"
    aria-modal="true"
  >
    <div class="modal-dialog modal-lg modal-dialog-scrollable">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            Conteneurs détectés
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

          <p class="text-muted small">
            Conteneurs gérés par compose, sans tracker de release. Sélectionnez ceux à suivre :
            un tracker Docker en mode <strong>Compose</strong> sera créé pour chacun (pull + up -d
            automatique à chaque nouvelle image).
          </p>

          <div
            v-if="loading"
            class="text-center py-4"
          >
            <div class="spinner-border text-primary" />
          </div>

          <div
            v-else-if="!containers.length"
            class="empty"
          >
            <p class="empty-title">
              Aucun conteneur à suivre
            </p>
            <p class="empty-subtitle text-muted">
              Tous les conteneurs compose détectés ont déjà un tracker, ou aucun n'a été collecté.
            </p>
          </div>

          <template v-else>
            <!-- Global options applied to all created trackers -->
            <div class="card mb-3">
              <div class="card-body py-2">
                <div class="row g-2 align-items-end">
                  <div class="col-md-4">
                    <label class="form-label mb-1">Healthcheck (s)</label>
                    <input
                      v-model.number="options.healthcheck_timeout_sec"
                      type="number"
                      min="0"
                      max="3600"
                      class="form-control form-control-sm"
                    >
                  </div>
                  <div class="col-md-8">
                    <div class="d-flex flex-wrap gap-3">
                      <label class="form-check mb-0">
                        <input
                          v-model="options.rollback_on_failure"
                          class="form-check-input"
                          type="checkbox"
                        >
                        <span class="form-check-label">Rollback si échec</span>
                      </label>
                      <label class="form-check mb-0">
                        <input
                          v-model="options.cleanup_after_update"
                          class="form-check-input"
                          type="checkbox"
                        >
                        <span class="form-check-label">Nettoyer images orphelines</span>
                      </label>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="table-responsive">
              <table class="table table-sm table-vcenter">
                <thead>
                  <tr>
                    <th style="width:32px">
                      <input
                        class="form-check-input m-0"
                        type="checkbox"
                        :checked="allSelected"
                        :indeterminate="someSelected"
                        @change="toggleAll"
                      >
                    </th>
                    <th>Hôte</th>
                    <th>Image</th>
                    <th>Projet / Service</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(c, idx) in containers"
                    :key="rowKey(c)"
                  >
                    <td>
                      <input
                        v-model="selected"
                        class="form-check-input m-0"
                        type="checkbox"
                        :value="idx"
                      >
                    </td>
                    <td class="text-truncate">
                      {{ c.host_name || c.host_id }}
                    </td>
                    <td>
                      <code class="small">{{ c.image }}:{{ c.image_tag || 'latest' }}</code>
                    </td>
                    <td>
                      <code class="small">{{ c.compose_project }}{{ c.compose_service ? ' / ' + c.compose_service : '' }}</code>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div
              v-if="resultSummary"
              class="alert alert-info mt-2 mb-0"
            >
              {{ resultSummary }}
            </div>
          </template>
        </div>
        <div class="modal-footer">
          <button
            class="btn btn-secondary"
            @click="close"
          >
            Fermer
          </button>
          <button
            class="btn btn-primary"
            :disabled="saving || !selected.length"
            @click="submit"
          >
            {{ saving ? 'Création...' : `Créer ${selected.length} tracker${selected.length > 1 ? 's' : ''}` }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import api from '../../api'
import { useModalFocusTrap } from '../../composables/useModalFocusTrap'

interface Container {
  host_id: string
  host_name?: string
  image: string
  image_tag?: string
  compose_project: string
  compose_service?: string
}

interface BulkResult {
  created?: boolean
  name?: string
}

const props = withDefaults(defineProps<{
  visible?: boolean
}>(), {
  visible: false,
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created'): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalFocusTrap(modalRef)

const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const resultSummary = ref('')
const containers = ref<Container[]>([])
const selected = ref<number[]>([])
const options = ref({
  healthcheck_timeout_sec: 0,
  rollback_on_failure: false,
  cleanup_after_update: false,
})

const allSelected = computed(() => containers.value.length > 0 && selected.value.length === containers.value.length)
const someSelected = computed(() => selected.value.length > 0 && selected.value.length < containers.value.length)

function rowKey(c: Container): string {
  return `${c.host_id}|${c.image}|${c.image_tag}|${c.compose_project}|${c.compose_service}`
}

watch(
  () => props.visible,
  async (visible) => {
    if (!visible) return
    await loadContainers()
  },
  { immediate: true }
)

async function loadContainers(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  resultSummary.value = ''
  selected.value = []
  try {
    const response = await api.getTrackableContainers()
    containers.value = Array.isArray(response.data?.containers) ? response.data.containers : []
    selected.value = containers.value.map((_, idx) => idx)
  } catch {
    errorMessage.value = 'Impossible de charger les conteneurs détectés.'
    containers.value = []
  } finally {
    loading.value = false
  }
}

function toggleAll(): void {
  selected.value = allSelected.value ? [] : containers.value.map((_, idx) => idx)
}

async function submit(): Promise<void> {
  if (!selected.value.length) return
  saving.value = true
  errorMessage.value = ''
  resultSummary.value = ''
  const trackers = selected.value.map((idx) => {
    const c = containers.value[idx]
    const name = c.compose_service
      ? `${c.compose_project}/${c.compose_service}`
      : c.compose_project
    return {
      name,
      tracker_type: 'docker',
      docker_image: c.image,
      docker_tag: c.image_tag || 'latest',
      host_id: c.host_id,
      update_action: 'compose',
      compose_project: c.compose_project,
      compose_service: c.compose_service || '',
      healthcheck_timeout_sec: options.value.healthcheck_timeout_sec || 0,
      rollback_on_failure: options.value.rollback_on_failure,
      cleanup_after_update: options.value.cleanup_after_update,
      notify_channels: [],
      notify_on_release: false,
      enabled: true,
    }
  })
  try {
    const response = await api.createReleaseTrackersBulk(trackers)
    const created = response.data?.created ?? 0
    const failed = ((response.data?.results || []) as BulkResult[]).filter((r) => !r.created)
    emit('created')
    if (failed.length) {
      resultSummary.value = `${created} tracker(s) créé(s), ${failed.length} en échec : ${failed.map((r) => r.name || '?').join(', ')}`
      await loadContainers()
    } else {
      close()
    }
  } catch {
    errorMessage.value = 'La création groupée a échoué.'
  } finally {
    saving.value = false
  }
}

function close(): void {
  emit('close')
}
</script>

<template>
  <div class="card mb-4">
    <div class="card-header">
      <h3 class="card-title">
        Modifier l'hote
      </h3>
    </div>
    <div class="card-body">
      <form
        class="row g-3"
        @submit.prevent="saveEdit"
      >
        <div class="col-md-6">
          <label class="form-label">Nom</label>
          <input
            v-model="editForm.name"
            type="text"
            class="form-control"
            required
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Hostname</label>
          <input
            v-model="editForm.hostname"
            type="text"
            class="form-control"
            required
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Adresse IP</label>
          <input
            v-model="editForm.ip_address"
            type="text"
            class="form-control"
            required
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">OS</label>
          <input
            v-model="editForm.os"
            type="text"
            class="form-control"
            required
          >
        </div>
        <div
          v-if="editError"
          class="col-12"
        >
          <div class="alert alert-danger py-2 mb-0">
            {{ editError }}
          </div>
        </div>
        <div class="col-12 d-flex justify-content-end gap-2">
          <button
            type="button"
            class="btn btn-outline-secondary"
            :disabled="saving"
            @click="$emit('close')"
          >
            Annuler
          </button>
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="saving"
          >
            {{ saving ? 'Enregistrement...' : 'Enregistrer' }}
          </button>
        </div>
        <div class="col-12">
          <div class="border-top pt-3 mt-2">
            <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-2">
              <div>
                <div class="fw-semibold">
                  API Key agent
                </div>
                <div class="text-secondary small">
                  Regenerer la cle pour un hote existant.
                </div>
              </div>
              <button
                type="button"
                class="btn btn-outline-warning"
                :disabled="rotateKeyLoading"
                @click="rotateHostKey"
              >
                {{ rotateKeyLoading ? 'Rotation...' : 'Regenerer la cle' }}
              </button>
            </div>
            <div
              v-if="rotateKeyResult"
              class="alert alert-info mt-3 mb-0"
              role="alert"
            >
              <div class="fw-semibold mb-2">
                Nouvelle cle generee
              </div>
              <div class="text-secondary small mb-2">
                Copiez-la maintenant, elle ne sera plus affichee.
              </div>
              <div class="d-flex align-items-center gap-2 mb-2">
                <div class="bg-dark rounded p-2 flex-fill">
                  <code class="text-light">{{ rotateKeyResult.api_key }}</code>
                </div>
                <button
                  type="button"
                  class="btn btn-outline-light"
                  @click="copyRotatedKey"
                >
                  {{ rotateCopiedKey ? 'Copie' : 'Copier' }}
                </button>
              </div>
              <div class="d-flex align-items-center justify-content-between mb-1">
                <div class="text-secondary small">
                  Configuration agent :
                </div>
                <button
                  type="button"
                  class="btn btn-outline-light btn-sm"
                  @click="copyRotatedConfig"
                >
                  {{ rotateCopiedConfig ? 'Copie' : 'Copier la config' }}
                </button>
              </div>
              <pre class="bg-dark text-light p-2 rounded small mb-0">{{ rotatedAgentConfig }}</pre>
            </div>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import apiClient from '../../api'

const emit = defineEmits(['close', 'updated'])

const props = defineProps({
  hostId: {
    type: [String, Number],
    required: true,
  },
  host: {
    type: Object,
    default: null,
  },
})

const saving = ref(false)
const editError = ref('')
const editForm = ref({ name: '', hostname: '', ip_address: '', os: '' })
const rotateKeyLoading = ref(false)
const rotateKeyResult = ref(null)
const rotateCopiedKey = ref(false)
const rotateCopiedConfig = ref(false)

const serverHostname =
  typeof window !== 'undefined' && window.location?.hostname
    ? window.location.hostname
    : 'localhost'

const rotatedAgentConfig = computed(() => {
  if (!rotateKeyResult.value) return ''
  return `server_url: "http://${serverHostname}:8080"\napi_key: "${rotateKeyResult.value.api_key}"\nreport_interval: 30\ncollect_docker: true\ncollect_apt: true`
})

watch(
  () => props.host,
  (host) => {
    editForm.value = {
      name: host?.name || '',
      hostname: host?.hostname || '',
      ip_address: host?.ip_address || '',
      os: host?.os || '',
    }
  },
  { immediate: true }
)

async function saveEdit() {
  editError.value = ''
  saving.value = true
  try {
    const res = await apiClient.updateHost(props.hostId, editForm.value)
    emit('updated', res.data)
    emit('close')
  } catch (e) {
    editError.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
  }
}

async function rotateHostKey() {
  rotateKeyLoading.value = true
  rotateKeyResult.value = null
  try {
    const res = await apiClient.rotateHostKey(props.hostId)
    rotateKeyResult.value = res.data
  } catch (e) {
    console.error('Failed to rotate API key:', e.response?.data || e.message)
  } finally {
    rotateKeyLoading.value = false
  }
}

async function copyRotatedKey() {
  if (!rotateKeyResult.value?.api_key) return
  await navigator.clipboard.writeText(rotateKeyResult.value.api_key)
  rotateCopiedKey.value = true
  setTimeout(() => {
    rotateCopiedKey.value = false
  }, 1500)
}

async function copyRotatedConfig() {
  if (!rotatedAgentConfig.value) return
  await navigator.clipboard.writeText(rotatedAgentConfig.value)
  rotateCopiedConfig.value = true
  setTimeout(() => {
    rotateCopiedConfig.value = false
  }, 1500)
}
</script>

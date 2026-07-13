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
        <div class="col-12">
          <label class="form-label">Tags</label>
          <input
            v-model="editForm.tags"
            type="text"
            class="form-control"
            placeholder="prod, site-lyon"
          >
          <div class="form-hint">
            Séparés par des virgules.
          </div>
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
                  Régénérer la clé pour un hôte existant.
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
              <div class="d-flex align-items-center gap-2 mb-3">
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
                  Commande d'installation (a executer sur l'hote cible) :
                </div>
                <button
                  type="button"
                  class="btn btn-outline-light btn-sm"
                  @click="copyRotatedInstallCmd"
                >
                  {{ rotateCopiedInstallCmd ? 'Copie' : 'Copier' }}
                </button>
              </div>
              <pre class="bg-dark text-light p-2 rounded small mb-3 text-wrap">{{ rotatedInstallCmd }}</pre>
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

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import apiClient from '../../api'
import { getApiErrorMessage } from '../../api/client'
import { parseTagsInput, formatTagsInput } from '../../utils/tags'
import { buildInstallCommand, buildAgentConfig } from '../../utils/agentInstall'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

interface Host {
  name?: string
  hostname?: string
  ip_address?: string
  os?: string
  tags?: string[]
}

interface HostForm {
  name: string
  hostname: string
  ip_address: string
  os: string
  tags: string
}

interface RotateKeyResult {
  api_key: string
  [key: string]: unknown
}

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated', data: unknown): void
}>()

const props = withDefaults(defineProps<{
  hostId: string | number
  host?: Host | null
}>(), {
  host: null,
})

const dialog = useConfirmDialog()
const saving = ref(false)
const editError = ref('')
const editForm = ref<HostForm>({ name: '', hostname: '', ip_address: '', os: '', tags: '' })
const rotateKeyLoading = ref(false)
const rotateKeyResult = ref<RotateKeyResult | null>(null)
const rotateCopiedKey = ref(false)
const rotateCopiedConfig = ref(false)
const rotateCopiedInstallCmd = ref(false)

const rotatedAgentConfig = computed(() => {
  if (!rotateKeyResult.value?.api_key) return ''
  return buildAgentConfig(rotateKeyResult.value.api_key)
})

const rotatedInstallCmd = computed(() => {
  if (!rotateKeyResult.value?.api_key) return ''
  return buildInstallCommand(rotateKeyResult.value.api_key)
})

watch(
  () => props.host,
  (host) => {
    editForm.value = {
      name: host?.name || '',
      hostname: host?.hostname || '',
      ip_address: host?.ip_address || '',
      os: host?.os || '',
      tags: formatTagsInput(host?.tags),
    }
  },
  { immediate: true }
)

async function saveEdit(): Promise<void> {
  editError.value = ''
  saving.value = true
  try {
    const res = await apiClient.updateHost(String(props.hostId), {
      name: editForm.value.name,
      hostname: editForm.value.hostname,
      ip_address: editForm.value.ip_address,
      os: editForm.value.os,
      tags: parseTagsInput(editForm.value.tags),
    })
    emit('updated', res.data)
    emit('close')
  } catch (e: unknown) {
    editError.value = getApiErrorMessage(e)
  } finally {
    saving.value = false
  }
}

async function rotateHostKey(): Promise<void> {
  const confirmed = await dialog.confirm({
    title: 'Régénérer la clé API',
    message: "L'ancienne clé sera immédiatement invalidée : l'agent actuellement déployé sur cet hôte perdra l'accès tant qu'il n'aura pas été reconfiguré avec la nouvelle clé.",
    variant: 'warning',
  })
  if (!confirmed) return

  rotateKeyLoading.value = true
  rotateKeyResult.value = null
  try {
    const res = await apiClient.rotateHostKey(String(props.hostId))
    rotateKeyResult.value = res.data
  } catch (e: unknown) {
    console.error('Failed to rotate API key:', getApiErrorMessage(e))
  } finally {
    rotateKeyLoading.value = false
  }
}

async function copyRotatedKey(): Promise<void> {
  if (!rotateKeyResult.value?.api_key) return
  await navigator.clipboard.writeText(rotateKeyResult.value.api_key)
  rotateCopiedKey.value = true
  setTimeout(() => {
    rotateCopiedKey.value = false
  }, 1500)
}

async function copyRotatedConfig(): Promise<void> {
  if (!rotatedAgentConfig.value) return
  await navigator.clipboard.writeText(rotatedAgentConfig.value)
  rotateCopiedConfig.value = true
  setTimeout(() => {
    rotateCopiedConfig.value = false
  }, 1500)
}

async function copyRotatedInstallCmd(): Promise<void> {
  if (!rotatedInstallCmd.value) return
  await navigator.clipboard.writeText(rotatedInstallCmd.value)
  rotateCopiedInstallCmd.value = true
  setTimeout(() => {
    rotateCopiedInstallCmd.value = false
  }, 1500)
}
</script>

<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        {{ t('settings.registryTitle') }}
      </h3>
      <button
        v-if="authIsAdmin && !showForm"
        type="button"
        class="btn btn-sm btn-primary"
        @click="openAddForm"
      >
        {{ t('settings.addCredential') }}
      </button>
    </div>

    <div class="card-body border-bottom py-2">
      <p class="text-muted small mb-0">
        {{ t('settings.registryDesc') }}
      </p>
    </div>

    <!-- Add / Edit form -->
    <div
      v-if="showForm && authIsAdmin"
      class="card-body border-bottom"
    >
      <div class="row g-3">
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.name') }} *</label>
          <input
            v-model="form.name"
            type="text"
            class="form-control"
            placeholder="GHCR mon-org"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.registryHostLabel') }} *</label>
          <input
            v-model="form.registry_host"
            type="text"
            class="form-control"
            placeholder="ghcr.io"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('common.user') }} *</label>
          <input
            v-model="form.username"
            type="text"
            class="form-control"
            autocomplete="off"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.passwordToken') }} {{ editingId ? t('settings.unchangedIfEmpty') : '*' }}</label>
          <input
            v-model="form.password"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
      </div>
      <div class="mt-3 d-flex align-items-center gap-2">
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t('common.saving') : (editingId ? t('settings.update') : t('settings.create')) }}
        </button>
        <button
          type="button"
          class="btn btn-outline-secondary"
          @click="cancelForm"
        >
          {{ t('settings.cancel') }}
        </button>
        <span
          v-if="formMsg"
          :class="['ms-auto small', formOk ? 'text-success' : 'text-danger']"
        >{{ formMsg }}</span>
      </div>
    </div>

    <!-- List -->
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('settings.name') }}</th>
            <th>{{ t('settings.host') }}</th>
            <th>{{ t('common.user') }}</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !credentials.length">
            <td colspan="4">
              <LoadingSkeleton variant="table" />
            </td>
          </tr>
          <tr v-else-if="credentials.length === 0">
            <td colspan="4">
              <EmptyState :title="t('settings.noCredentialsConfigured')" />
            </td>
          </tr>
          <tr
            v-for="cred in credentials"
            :key="cred.id"
          >
            <td class="fw-medium">
              {{ cred.name }}
            </td>
            <td class="text-muted small">
              {{ cred.registry_host }}
            </td>
            <td class="text-muted small">
              {{ cred.username }}
            </td>
            <td
              v-if="authIsAdmin"
              class="text-end"
            >
              <div class="d-flex gap-1 justify-content-end">
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :title="t('settings.edit')"
                  :aria-label="t('settings.editCredentialAriaLabel')"
                  @click="openEditForm(cred)"
                >
                  <IconPencil
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :title="t('settings.delete')"
                  :aria-label="t('settings.deleteCredentialAriaLabel')"
                  @click="remove(cred)"
                >
                  <IconTrash
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div
      v-if="listMsg"
      class="card-footer"
    >
      <span :class="['small', listOk ? 'text-success' : 'text-danger']">{{ listMsg }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPencil, IconTrash } from '@tabler/icons-vue'
import api from '../../api/index'
import { getApiErrorMessage } from '../../api/client'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const { t } = useI18n()
const { confirm } = useConfirmDialog()

interface Credential {
  id: string
  name: string
  registry_host: string
  username: string
}

interface CredentialForm {
  name: string
  registry_host: string
  username: string
  password: string
}

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

const credentials = ref<Credential[]>([])
const loading = ref(false)
const showForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const formMsg = ref('')
const formOk = ref(false)
const listMsg = ref('')
const listOk = ref(false)

const emptyForm = (): CredentialForm => ({
  name: '',
  registry_host: '',
  username: '',
  password: '',
})

const form = ref<CredentialForm>(emptyForm())

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await api.getRegistryCredentials()
    credentials.value = Array.isArray(res.data?.credentials) ? res.data.credentials : []
  } catch {
    // silently ignore
  } finally {
    loading.value = false
  }
}

function openAddForm(): void {
  editingId.value = null
  form.value = emptyForm()
  formMsg.value = ''
  showForm.value = true
}

function openEditForm(cred: Credential): void {
  editingId.value = cred.id
  form.value = {
    name: cred.name,
    registry_host: cred.registry_host,
    username: cred.username,
    password: '',
  }
  formMsg.value = ''
  showForm.value = true
}

function cancelForm(): void {
  showForm.value = false
  formMsg.value = ''
  editingId.value = null
}

async function save(): Promise<void> {
  if (!form.value.name || !form.value.registry_host || !form.value.username) {
    formMsg.value = t('settings.nameHostUserRequired')
    formOk.value = false
    return
  }
  if (!editingId.value && !form.value.password) {
    formMsg.value = t('settings.passwordRequiredOnCreate')
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
  try {
    if (editingId.value) {
      await api.updateRegistryCredential(editingId.value, form.value)
    } else {
      await api.createRegistryCredential(form.value)
    }
    formMsg.value = editingId.value ? t('settings.credentialUpdated') : t('settings.credentialCreated')
    formOk.value = true
    await load()
    showForm.value = false
    editingId.value = null
  } catch (e: unknown) {
    formMsg.value = getApiErrorMessage(e, t('settings.saveError'))
    formOk.value = false
  } finally {
    saving.value = false
  }
}

async function remove(cred: Credential): Promise<void> {
  const confirmed = await confirm({
    title: t('settings.deleteCredentialTitle'),
    message: t('settings.deleteCredentialMsg', { name: cred.name }),
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await api.deleteRegistryCredential(cred.id)
    await load()
    listMsg.value = t('settings.credentialDeleted')
    listOk.value = true
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, t('settings.deleteError'))
    listOk.value = false
  }
}

onMounted(load)
</script>

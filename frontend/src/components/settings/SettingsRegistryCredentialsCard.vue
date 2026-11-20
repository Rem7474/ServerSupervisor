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
import { useI18n } from 'vue-i18n'
import { IconPencil, IconTrash } from '@tabler/icons-vue'
import api from '../../api/index'
import type { RegistryCredential } from '../../types/tracker'
import { useConnectionCrud } from '../../composables/useConnectionCrud'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const { t } = useI18n()

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

const {
  items: credentials, loading, showForm, editingId, form, saving, formMsg, formOk, listMsg, listOk,
  openAddForm, openEditForm, cancelForm, save, remove,
} = useConnectionCrud<RegistryCredential, CredentialForm>({
  list: async () => {
    const data = (await api.getRegistryCredentials()).data
    return Array.isArray(data?.credentials) ? data.credentials : []
  },
  create: (f) => api.createRegistryCredential(f),
  update: (id, f) => api.updateRegistryCredential(id, f),
  remove: (id) => api.deleteRegistryCredential(id),
  emptyForm: () => ({ name: '', registry_host: '', username: '', password: '' }),
  toForm: (cred) => ({ name: cred.name, registry_host: cred.registry_host, username: cred.username, password: '' }),
  validate: (f, editing) => {
    if (!f.name || !f.registry_host || !f.username) return t('settings.nameHostUserRequired')
    if (!editing && !f.password) return t('settings.passwordRequiredOnCreate')
    return ''
  },
  keys: {
    created: 'settings.credentialCreated',
    updated: 'settings.credentialUpdated',
    deleted: 'settings.credentialDeleted',
    deleteTitle: 'settings.deleteCredentialTitle',
    deleteMessage: 'settings.deleteCredentialMsg',
  },
})
</script>

<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Nginx Proxy Manager
      </h3>
      <button
        v-if="authIsAdmin && !showForm"
        type="button"
        class="btn btn-sm btn-primary"
        @click="openAddForm"
      >
        <IconPlus
          :size="16"
          class="icon me-1"
        />
        {{ t('settings.addConnection') }}
      </button>
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
            placeholder="Mon NPM"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.apiUrl') }} *</label>
          <input
            v-model="form.api_url"
            type="text"
            class="form-control"
            placeholder="http://192.168.1.10:81"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.identityEmail') }} *</label>
          <input
            v-model="form.identity"
            type="text"
            class="form-control"
            placeholder="admin@example.com"
            autocomplete="username"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.password') }} {{ editingId ? t('settings.unchangedIfEmpty') : '*' }}</label>
          <input
            v-model="form.secret"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
        <div class="col-md-4">
          <label class="form-label">{{ t('settings.pollInterval') }}</label>
          <input
            v-model.number="form.poll_interval_sec"
            type="number"
            class="form-control"
            min="60"
          >
        </div>
        <div class="col-md-4 d-flex align-items-end">
          <label class="form-check form-switch mb-0">
            <input
              v-model="form.enabled"
              class="form-check-input"
              type="checkbox"
            >
            <span class="form-check-label">{{ t('settings.enabled') }}</span>
          </label>
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
        <button
          type="button"
          class="btn btn-outline-secondary ms-2"
          :disabled="testing"
          @click="testForm"
        >
          {{ testing ? t('settings.testingShort') : t('settings.testConnection') }}
        </button>
        <span
          v-if="formMsg"
          :class="['ms-auto small', formOk ? 'text-success' : 'text-danger']"
        >{{ formMsg }}</span>
      </div>
    </div>

    <!-- Connections list -->
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('settings.name') }}</th>
            <th>{{ t('settings.apiUrl') }}</th>
            <th>{{ t('settings.identity') }}</th>
            <th>{{ t('settings.proxyHosts') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('settings.lastContact') }}</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !connections.length">
            <td colspan="7">
              <LoadingSkeleton variant="table" />
            </td>
          </tr>
          <tr v-else-if="connections.length === 0">
            <td colspan="7">
              <EmptyState :title="t('settings.noNpmConnections')" />
            </td>
          </tr>
          <tr
            v-for="conn in connections"
            :key="conn.id"
          >
            <td class="fw-medium">
              {{ conn.name }}
            </td>
            <td class="text-muted small">
              {{ conn.api_url }}
            </td>
            <td class="text-muted small">
              {{ conn.identity }}
            </td>
            <td>{{ conn.proxy_host_count }}</td>
            <td>
              <span
                v-if="!conn.enabled"
                class="badge bg-secondary-lt text-secondary"
              >{{ t('settings.disabled') }}</span>
              <span
                v-else-if="conn.last_error"
                class="badge bg-danger-lt text-danger"
                :title="conn.last_error"
              >{{ t('settings.errorBadge') }}</span>
              <span
                v-else-if="conn.last_success_at"
                class="badge bg-success-lt text-success"
              >OK</span>
              <span
                v-else
                class="badge bg-warning-lt text-warning"
              >{{ t('settings.pendingBadge') }}</span>
            </td>
            <td class="text-muted small">
              <span v-if="conn.last_success_at">{{ formatDate(conn.last_success_at) }}</span>
              <span v-else>—</span>
            </td>
            <td
              v-if="authIsAdmin"
              class="text-end"
            >
              <div class="d-flex gap-1 justify-content-end">
                <!-- Edit -->
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :title="t('settings.edit')"
                  :aria-label="t('settings.edit')"
                  @click="openEditForm(conn)"
                >
                  <IconPencil
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <!-- Refresh -->
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :title="t('settings.refreshNowTooltip')"
                  :aria-label="t('settings.refreshNowTooltip')"
                  @click="refreshNow(conn)"
                >
                  <IconRefresh
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <!-- Delete -->
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :title="t('settings.delete')"
                  :aria-label="t('settings.delete')"
                  @click="remove(conn)"
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
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPencil, IconPlus, IconRefresh, IconTrash } from '@tabler/icons-vue'
import { npmApi } from '../../api/npm'
import type { NPMConnection } from '../../types/npm'
import { getApiErrorMessage } from '../../api/client'
import { useConnectionCrud } from '../../composables/useConnectionCrud'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { formatDateTime } from '../../utils/formatters'

const { t } = useI18n()

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

interface NPMForm {
  name: string
  api_url: string
  identity: string
  secret: string
  enabled: boolean
  poll_interval_sec: number
}

const {
  items: connections, loading, showForm, editingId, form, saving, formMsg, formOk, listMsg, listOk,
  openAddForm, openEditForm, cancelForm, save, remove, runListAction,
} = useConnectionCrud<NPMConnection, NPMForm>({
  list: async () => (await npmApi.listConnections()).data.connections ?? [],
  create: (f) => npmApi.createConnection(f),
  update: (id, f) => npmApi.updateConnection(id, f),
  remove: (id) => npmApi.deleteConnection(id),
  emptyForm: () => ({ name: '', api_url: '', identity: '', secret: '', enabled: true, poll_interval_sec: 3600 }),
  toForm: (conn) => ({
    name: conn.name,
    api_url: conn.api_url,
    identity: conn.identity,
    secret: '',
    enabled: conn.enabled ?? true,
    poll_interval_sec: conn.poll_interval_sec ?? 3600,
  }),
  validate: (f, editing) => {
    if (!f.name || !f.api_url || !f.identity) return t('settings.nameUrlIdentityRequired')
    if (!editing && !f.secret) return t('settings.passwordRequiredOnCreate')
    return ''
  },
  keys: {
    created: 'settings.connectionCreated',
    updated: 'settings.connectionUpdated',
    deleted: 'settings.connectionDeleted',
    deleteTitle: 'settings.deleteNpmConnectionTitle',
    deleteMessage: 'settings.deleteNpmConnectionMsg',
  },
})

const testing = ref(false)

async function testForm(): Promise<void> {
  if (!form.value.api_url || !form.value.identity || !form.value.secret) {
    formMsg.value = t('settings.fillUrlIdentityPasswordToTest')
    formOk.value = false
    return
  }
  testing.value = true
  formMsg.value = ''
  try {
    const res = await npmApi.testConnection({
      api_url: form.value.api_url,
      identity: form.value.identity,
      secret: form.value.secret,
    })
    formOk.value = !!res.data.success
    formMsg.value = res.data.success ? t('settings.connectionSuccessful') : (res.data.error || t('settings.connectionFailed'))
  } catch (e: unknown) {
    formMsg.value = getApiErrorMessage(e, t('settings.networkError'))
    formOk.value = false
  } finally {
    testing.value = false
  }
}

function refreshNow(conn: NPMConnection): Promise<void> {
  return runListAction(() => npmApi.refreshNow(conn.id), t('settings.refreshTriggeredFor', { name: conn.name }))
}

function formatDate(iso: string | undefined): string {
  return formatDateTime(iso, '—')
}
</script>

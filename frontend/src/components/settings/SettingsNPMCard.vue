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
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPencil, IconPlus, IconRefresh, IconTrash } from '@tabler/icons-vue'
import { npmApi } from '../../api/npm'
import type { NPMConnection } from '../../types/npm'
import { getApiErrorMessage } from '../../api/client'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const { t, locale } = useI18n()
const { confirm } = useConfirmDialog()

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

const connections = ref<NPMConnection[]>([])
const loading = ref(false)
const showForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testing = ref(false)
const formMsg = ref('')
const formOk = ref(false)
const listMsg = ref('')
const listOk = ref(false)

const emptyForm = (): NPMForm => ({
  name: '',
  api_url: '',
  identity: '',
  secret: '',
  enabled: true,
  poll_interval_sec: 3600,
})

const form = ref<NPMForm>(emptyForm())

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await npmApi.listConnections()
    connections.value = res.data.connections ?? []
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

function openEditForm(conn: NPMConnection): void {
  editingId.value = conn.id
  form.value = {
    name: conn.name,
    api_url: conn.api_url,
    identity: conn.identity,
    secret: '',
    enabled: conn.enabled ?? true,
    poll_interval_sec: conn.poll_interval_sec ?? 3600,
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
  if (!form.value.name || !form.value.api_url || !form.value.identity) {
    formMsg.value = t('settings.nameUrlIdentityRequired')
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
  try {
    if (editingId.value) {
      await npmApi.updateConnection(editingId.value, form.value)
    } else {
      if (!form.value.secret) {
        formMsg.value = t('settings.passwordRequiredOnCreate')
        formOk.value = false
        saving.value = false
        return
      }
      await npmApi.createConnection(form.value)
    }
    formMsg.value = editingId.value ? t('settings.connectionUpdated') : t('settings.connectionCreated')
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
    if (res.data.success) {
      formMsg.value = t('settings.connectionSuccessful')
      formOk.value = true
    } else {
      formMsg.value = res.data.error || t('settings.connectionFailed')
      formOk.value = false
    }
  } catch (e: unknown) {
    formMsg.value = getApiErrorMessage(e, t('settings.networkError'))
    formOk.value = false
  } finally {
    testing.value = false
  }
}

async function refreshNow(conn: NPMConnection): Promise<void> {
  try {
    await npmApi.refreshNow(conn.id)
    listMsg.value = t('settings.refreshTriggeredFor', { name: conn.name })
    listOk.value = true
    setTimeout(load, 3000)
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, t('settings.genericErrorPeriod'))
    listOk.value = false
  }
}

async function remove(conn: NPMConnection): Promise<void> {
  const confirmed = await confirm({
    title: t('settings.deleteNpmConnectionTitle'),
    message: t('settings.deleteNpmConnectionMsg', { name: conn.name }),
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await npmApi.deleteConnection(conn.id)
    await load()
    listMsg.value = t('settings.connectionDeleted')
    listOk.value = true
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, t('settings.deleteError'))
    listOk.value = false
  }
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
}

onMounted(load)
</script>

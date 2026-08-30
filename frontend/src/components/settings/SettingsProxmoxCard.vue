<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Proxmox VE
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
            placeholder="Mon cluster PVE"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.apiUrl') }} *</label>
          <input
            v-model="form.api_url"
            type="text"
            class="form-control"
            placeholder="https://pve.example.com:8006/api2/json"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.tokenId') }} *</label>
          <input
            v-model="form.token_id"
            type="text"
            class="form-control"
            placeholder="root@pam!supervision"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">{{ t('settings.tokenSecret') }} {{ editingId ? t('settings.unchangedIfEmpty') : '*' }}</label>
          <input
            v-model="form.token_secret"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
        <div class="col-md-4">
          <label class="form-label">{{ t('settings.pollIntervalCollect') }}</label>
          <input
            v-model.number="form.poll_interval_sec"
            type="number"
            class="form-control"
            min="10"
          >
        </div>
        <div class="col-md-4 d-flex align-items-end gap-3">
          <label class="form-check form-switch mb-0">
            <input
              v-model="form.insecure_skip_verify"
              class="form-check-input"
              type="checkbox"
            >
            <span class="form-check-label">{{ t('settings.ignoreTls') }}</span>
          </label>
        </div>
        <div class="col-md-4 d-flex align-items-end gap-3">
          <label class="form-check form-switch mb-0">
            <input
              v-model="form.enabled"
              class="form-check-input"
              type="checkbox"
            >
            <span class="form-check-label">{{ t('settings.enabled') }}</span>
          </label>
        </div>
        <div class="col-12">
          <label class="form-label">
            {{ t('settings.pveConsoleCredsOptional') }}
          </label>
          <div class="form-hint mb-2">
            {{ t('settings.pveConsoleHintIntro') }}
            <code>VM.Console</code>
            {{ t('settings.pveConsoleHintExample') }} <code>root@pam</code>).
          </div>
        </div>
        <div class="col-md-6">
          <label
            for="proxmox-pve-username"
            class="form-label"
          >{{ t('settings.pveUsername') }}</label>
          <input
            id="proxmox-pve-username"
            v-model="form.pve_username"
            type="text"
            class="form-control"
            placeholder="root@pam"
          >
        </div>
        <div class="col-md-6">
          <label
            for="proxmox-pve-password"
            class="form-label"
          >{{ t('settings.pvePassword') }} {{ editingId ? t('settings.unchangedIfEmpty') : '' }}</label>
          <input
            id="proxmox-pve-password"
            v-model="form.pve_password"
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
          :class="['ms-auto small', formWarn ? 'text-warning' : formOk ? 'text-success' : 'text-danger']"
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
            <th>{{ t('settings.tokenId') }}</th>
            <th>{{ t('settings.consoleColumn') }}</th>
            <th>{{ t('settings.nodesColumn') }}</th>
            <th>{{ t('settings.guestsColumn') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('settings.lastContact') }}</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !instances.length">
            <td colspan="9">
              <LoadingSkeleton variant="table" />
            </td>
          </tr>
          <tr v-else-if="instances.length === 0">
            <td colspan="9">
              <EmptyState :title="t('settings.noProxmoxConnections')" />
            </td>
          </tr>
          <tr
            v-for="inst in instances"
            :key="inst.id"
          >
            <td class="fw-medium">
              {{ inst.name }}
            </td>
            <td class="text-muted small">
              {{ inst.api_url }}
            </td>
            <td class="text-muted small">
              {{ inst.token_id }}
            </td>
            <td>
              <span
                v-if="inst.console_configured"
                class="badge bg-success-lt text-success"
              >{{ t('settings.configuredBadge') }}</span>
              <span
                v-else
                class="badge bg-secondary-lt text-secondary"
              >{{ t('settings.notConfiguredBadge') }}</span>
            </td>
            <td>{{ inst.node_count }}</td>
            <td>{{ inst.guest_count }}</td>
            <td>
              <span
                v-if="!inst.enabled"
                class="badge bg-secondary-lt text-secondary"
              >{{ t('settings.disabled') }}</span>
              <span
                v-else-if="inst.last_error"
                class="badge bg-danger-lt text-danger"
                :title="inst.last_error"
              >{{ t('settings.errorBadge') }}</span>
              <span
                v-else-if="inst.last_success_at"
                class="badge bg-success-lt text-success"
              >OK</span>
              <span
                v-else
                class="badge bg-warning-lt text-warning"
              >{{ t('settings.pendingBadge') }}</span>
            </td>
            <td class="text-muted small">
              <span v-if="inst.last_success_at">{{ formatDate(inst.last_success_at) }}</span>
              <span v-else>—</span>
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
                  @click="openEditForm(inst)"
                >
                  <IconPencil
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  :title="t('settings.testTooltip')"
                  @click="testById(inst)"
                >
                  <IconClock
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-primary"
                  :title="t('settings.pollNowTooltip')"
                  @click="pollNow(inst)"
                >
                  <IconRefresh
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :title="t('settings.delete')"
                  @click="remove(inst)"
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
      <span :class="['small', listWarn ? 'text-warning' : listOk ? 'text-success' : 'text-danger']">{{ listMsg }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconClock, IconPencil, IconPlus, IconRefresh, IconTrash } from '@tabler/icons-vue'
import api from '../../api/index'
import type { ProxmoxConnection, ProxmoxTestResult } from '../../types/proxmox'
import { getApiErrorMessage } from '../../api/client'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const { t, locale } = useI18n()
const { confirm } = useConfirmDialog()

// Use the shared domain type (the settings card only reads a subset of fields).
type ProxmoxInstance = ProxmoxConnection

interface ProxmoxForm {
  name: string
  api_url: string
  token_id: string
  token_secret: string
  insecure_skip_verify: boolean
  enabled: boolean
  poll_interval_sec: number
  pve_username: string
  pve_password: string
}

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

const instances = ref<ProxmoxInstance[]>([])
const loading = ref(false)
const showForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testing = ref(false)
const formMsg = ref('')
const formOk = ref(false)
// Set alongside formOk only by test-result paths (formatTestResult's
// 'warning' tone: API reachable but console misconfigured/unreachable) —
// every other message path (save/create/delete/poll) leaves this false.
const formWarn = ref(false)
const listMsg = ref('')
const listOk = ref(false)
const listWarn = ref(false)

const emptyForm = (): ProxmoxForm => ({
  name: '',
  api_url: '',
  token_id: '',
  token_secret: '',
  insecure_skip_verify: false,
  enabled: true,
  poll_interval_sec: 60,
  pve_username: '',
  pve_password: '',
})

const form = ref<ProxmoxForm>(emptyForm())

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await api.getProxmoxInstances()
    instances.value = res.data
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
  formWarn.value = false
  showForm.value = true
}

function openEditForm(inst: ProxmoxInstance): void {
  editingId.value = inst.id
  form.value = {
    name: inst.name,
    api_url: inst.api_url,
    token_id: inst.token_id,
    token_secret: '',
    insecure_skip_verify: inst.insecure_skip_verify ?? false,
    enabled: inst.enabled ?? true,
    poll_interval_sec: inst.poll_interval_sec ?? 60,
    pve_username: inst.pve_username ?? '',
    pve_password: '',
  }
  formMsg.value = ''
  formWarn.value = false
  showForm.value = true
}

function cancelForm(): void {
  showForm.value = false
  formMsg.value = ''
  formWarn.value = false
  editingId.value = null
}

async function save(): Promise<void> {
  if (!form.value.name || !form.value.api_url || !form.value.token_id) {
    formMsg.value = t('settings.nameUrlTokenRequired')
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
  formWarn.value = false
  try {
    if (editingId.value) {
      await api.updateProxmoxInstance(editingId.value, form.value)
    } else {
      if (!form.value.token_secret) {
        formMsg.value = t('settings.tokenSecretRequiredOnCreate')
        formOk.value = false
        saving.value = false
        return
      }
      await api.createProxmoxInstance(form.value)
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

// Shared by testForm/testById: turns a ProxmoxTestResult into a single
// human message plus its ok/warning/error tone. Console fields are only
// meaningful when console_configured is true; console_ok only proves the
// PVE login itself succeeded — it can't confirm the VM.Console privilege
// without a specific guest to open a console against (see the backend's
// TestConnection doc comment), so that caveat is spelled out rather than
// implied by a green badge.
function formatTestResult(result: ProxmoxTestResult): { message: string, tone: 'success' | 'warning' | 'danger' } {
  if (!result.success) {
    return { message: result.error || t('settings.connectionFailed'), tone: 'danger' }
  }
  if (!result.console_configured) {
    return { message: t('settings.apiSuccessConsoleNotConfigured'), tone: 'warning' }
  }
  if (result.console_ok) {
    return { message: t('settings.apiSuccessConsoleValid'), tone: 'success' }
  }
  return {
    message: t('settings.apiSuccessConsoleFailed', { error: result.console_error || t('settings.unknownError') }),
    tone: 'warning',
  }
}

async function testForm(): Promise<void> {
  testing.value = true
  formMsg.value = ''
  formWarn.value = false
  try {
    // Editing: test the connection as actually stored server-side, so
    // fields left blank on purpose (unchanged) are honored instead of being
    // sent empty and failing the test.
    if (editingId.value) {
      const res = await api.testProxmoxInstanceById(editingId.value)
      const { message, tone } = formatTestResult(res.data)
      formMsg.value = message
      formOk.value = tone !== 'danger'
      formWarn.value = tone === 'warning'
      return
    }
    if (!form.value.api_url || !form.value.token_id || !form.value.token_secret) {
      formMsg.value = t('settings.fillUrlTokenSecretToTest')
      formOk.value = false
      return
    }
    const res = await api.testProxmoxConnection({
      api_url: form.value.api_url,
      token_id: form.value.token_id,
      token_secret: form.value.token_secret,
      insecure_skip_verify: form.value.insecure_skip_verify,
      pve_username: form.value.pve_username,
      pve_password: form.value.pve_password,
    })
    const { message, tone } = formatTestResult(res.data)
    formMsg.value = message
    formOk.value = tone !== 'danger'
    formWarn.value = tone === 'warning'
  } catch (e: unknown) {
    formMsg.value = getApiErrorMessage(e, t('settings.networkError'))
    formOk.value = false
  } finally {
    testing.value = false
  }
}

async function testById(inst: ProxmoxInstance): Promise<void> {
  listMsg.value = ''
  listWarn.value = false
  try {
    const res = await api.testProxmoxInstanceById(inst.id)
    const { message, tone } = formatTestResult(res.data)
    listMsg.value = `[${inst.name}] ${message}`
    listOk.value = tone !== 'danger'
    listWarn.value = tone === 'warning'
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, t('settings.networkError'))
    listOk.value = false
  }
}

async function pollNow(inst: ProxmoxInstance): Promise<void> {
  listWarn.value = false
  try {
    await api.pollProxmoxNow(inst.id)
    listMsg.value = t('settings.collectTriggeredFor', { name: inst.name })
    listOk.value = true
    setTimeout(load, 3000)
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, t('settings.genericErrorPeriod'))
    listOk.value = false
  }
}

async function remove(inst: ProxmoxInstance): Promise<void> {
  const confirmed = await confirm({
    title: t('settings.deleteProxmoxConnectionTitle'),
    message: t('settings.deleteProxmoxConnectionMsg', { name: inst.name }),
    variant: 'danger',
  })
  if (!confirmed) return
  listWarn.value = false
  try {
    await api.deleteProxmoxInstance(inst.id)
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

<template>
  <div>
    <!-- Page Header -->
    <div class="page-header d-print-none mb-4">
      <div class="row align-items-center">
        <div class="col">
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              Dashboard
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>{{ t('config.pageTitle') }}</span>
          </div>
          <h2 class="page-title d-flex align-items-center gap-2">
            <IconBrandDocker
              :size="28"
              class="text-primary"
            />
            {{ t('config.pageTitle') }}
          </h2>
          <div class="text-secondary mt-1">
            {{ t('config.pageSubtitle') }}
          </div>
        </div>
        <div class="col-auto ms-auto d-flex align-items-center gap-2">
          <button
            type="button"
            class="btn btn-outline-secondary d-flex align-items-center gap-1"
            @click="toggleRevealSecrets"
          >
            <component
              :is="revealSecrets ? IconEyeOff : IconEye"
              :size="16"
            />
            {{ revealSecrets ? t('config.actions.hideSecrets') : t('config.actions.revealSecrets') }}
          </button>
          <button
            type="button"
            class="btn btn-outline-primary d-flex align-items-center gap-1"
            :disabled="loading"
            @click="loadConfig"
          >
            <IconRefresh
              :size="16"
              :class="{ 'rotate-spinner': loading }"
            />
            {{ t('config.actions.refresh') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Overview Metrics Cards -->
    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="row align-items-center">
              <div class="col-auto">
                <span class="bg-primary text-white avatar">
                  <IconAdjustments :size="20" />
                </span>
              </div>
              <div class="col">
                <div class="font-weight-medium">
                  {{ summary?.total_params ?? 0 }}
                </div>
                <div class="text-secondary small">
                  {{ t('config.metrics.total') }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="row align-items-center">
              <div class="col-auto">
                <span class="bg-azure text-white avatar">
                  <IconBrandDocker :size="20" />
                </span>
              </div>
              <div class="col">
                <div class="font-weight-medium">
                  {{ summary?.env_count ?? 0 }}
                </div>
                <div class="text-secondary small">
                  {{ t('config.metrics.env') }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="row align-items-center">
              <div class="col-auto">
                <span class="bg-success text-white avatar">
                  <IconDatabase :size="20" />
                </span>
              </div>
              <div class="col">
                <div class="font-weight-medium">
                  {{ summary?.ui_count ?? 0 }}
                </div>
                <div class="text-secondary small">
                  {{ t('config.metrics.ui') }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-sm-6 col-lg-3">
        <div
          class="card card-sm"
          :class="{ 'border-danger': (summary?.conflict_count ?? 0) > 0 }"
        >
          <div class="card-body">
            <div class="row align-items-center">
              <div class="col-auto">
                <span
                  :class="(summary?.conflict_count ?? 0) > 0 ? 'bg-danger text-white' : 'bg-secondary text-white'"
                  class="avatar"
                >
                  <IconAlertTriangle :size="20" />
                </span>
              </div>
              <div class="col">
                <div
                  class="font-weight-medium"
                  :class="{ 'text-danger': (summary?.conflict_count ?? 0) > 0 }"
                >
                  {{ summary?.conflict_count ?? 0 }}
                </div>
                <div class="text-secondary small">
                  {{ t('config.metrics.conflicts') }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Priority Info Alert -->
    <div
      class="alert alert-info d-flex align-items-center gap-3 mb-4"
      role="alert"
    >
      <IconInfoCircle
        :size="24"
        class="text-info flex-shrink-0"
      />
      <div>
        <div class="fw-bold">
          {{ t('config.warnings.priorityNotice') }}
        </div>
        <div class="small text-secondary mt-1">
          {{ t('config.warnings.envNotice') }}
        </div>
      </div>
    </div>

    <!-- Filters & Search Toolbar -->
    <div class="card mb-4">
      <div class="card-body p-3">
        <div class="row g-3 align-items-center">
          <div class="col-12 col-md-5">
            <div class="input-icon">
              <span class="input-icon-addon">
                <IconSearch :size="16" />
              </span>
              <input
                id="config-search-input"
                v-model="searchQuery"
                type="text"
                class="form-control"
                :placeholder="t('config.actions.searchPlaceholder')"
                :aria-label="t('config.actions.searchPlaceholder')"
              >
            </div>
          </div>

          <div class="col-6 col-md-4">
            <select
              id="config-category-filter"
              v-model="selectedCategory"
              class="form-select"
              :aria-label="t('config.categories.all')"
            >
              <option value="all">
                {{ t('config.categories.all') }}
              </option>
              <option
                v-for="cat in availableCategories"
                :key="cat"
                :value="cat"
              >
                {{ t(`config.categories.${cat}`) }}
              </option>
            </select>
          </div>

          <div class="col-6 col-md-3">
            <select
              id="config-source-filter"
              v-model="selectedSource"
              class="form-select"
              :aria-label="t('config.actions.allSources')"
            >
              <option value="all">
                {{ t('config.actions.allSources') }}
              </option>
              <option value="env">
                {{ t('config.sources.env') }}
              </option>
              <option value="ui">
                {{ t('config.sources.ui') }}
              </option>
              <option value="default">
                {{ t('config.sources.default') }}
              </option>
              <option value="conflict">
                {{ t('config.badges.conflict') }}
              </option>
            </select>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading State -->
    <div
      v-if="loading && !summary"
      class="text-center py-5"
    >
      <div
        class="spinner-border text-primary"
        role="status"
      />
      <div class="text-secondary mt-2">
        {{ t('config.messages.loading') }}
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-else-if="filteredEntries.length === 0"
      class="empty py-5"
    >
      <div class="empty-icon">
        <IconSearch
          :size="48"
          class="text-muted"
        />
      </div>
      <p class="empty-title">
        {{ t('config.messages.noResults') }}
      </p>
    </div>

    <!-- Configuration Entries Grouped by Category -->
    <div
      v-else
      class="d-flex flex-column gap-4"
    >
      <div
        v-for="group in groupedEntries"
        :key="group.category"
        class="card"
      >
        <div class="card-header d-flex justify-content-between align-items-center">
          <div>
            <h3 class="card-title d-flex align-items-center gap-2 mb-0">
              <component
                :is="categoryIcon(group.category)"
                :size="18"
                class="text-primary"
              />
              {{ t(`config.categories.${group.category}`) }}
            </h3>
            <span class="text-secondary small">{{ group.entries.length }} {{ t('config.table.items') }}</span>
          </div>
          <button
            v-if="hasDirtyInCategory(group.category)"
            type="button"
            class="btn btn-sm btn-primary d-flex align-items-center gap-1"
            :disabled="savingCategories[group.category]"
            @click="saveCategory(group.category)"
          >
            <IconCheck :size="14" />
            {{ t('config.actions.saveCategory') }}
          </button>
        </div>

        <div class="table-responsive">
          <table class="table table-vcenter card-table table-striped">
            <thead>
              <tr>
                <th style="width: 38%;">
                  {{ t('config.table.parameter') }}
                </th>
                <th style="width: 14%;">
                  {{ t('config.table.source') }}
                </th>
                <th style="width: 36%;">
                  {{ t('config.table.value') }}
                </th>
                <th
                  style="width: 12%;"
                  class="text-end"
                >
                  {{ t('config.table.actions') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="entry in group.entries"
                :key="entry.key"
                :class="{ 'table-danger-subtle': entry.has_conflict }"
              >
                <!-- Parameter Info -->
                <td>
                  <div class="d-flex align-items-baseline gap-2 flex-wrap mb-1">
                    <span class="fw-bold">{{ entry.label }}</span>
                    <code class="text-muted small">{{ entry.key }}</code>
                  </div>
                  <div class="text-secondary small mb-2">
                    {{ entry.description }}
                  </div>
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <span
                      v-if="entry.requires_restart"
                      class="badge bg-purple-lt text-purple"
                      :title="t('config.warnings.restartNotice')"
                    >
                      <IconReload
                        :size="12"
                        class="me-1"
                      />
                      {{ t('config.badges.restartRequired') }}
                    </span>
                    <span
                      v-if="entry.is_secret"
                      class="badge bg-secondary-lt text-secondary"
                    >
                      <IconLock
                        :size="12"
                        class="me-1"
                      />
                      {{ t('config.badges.secret') }}
                    </span>
                    <span
                      v-if="!entry.is_editable"
                      class="badge bg-dark-lt text-dark"
                    >
                      {{ t('config.badges.readOnly') }}
                    </span>
                  </div>
                </td>

                <!-- Source Badge -->
                <td>
                  <div class="d-flex flex-column gap-1 align-items-start">
                    <span
                      v-if="entry.source === 'env'"
                      class="badge bg-azure text-white d-inline-flex align-items-center gap-1"
                    >
                      <IconBrandDocker :size="12" />
                      {{ t('config.sources.env') }}
                    </span>
                    <span
                      v-else-if="entry.source === 'ui'"
                      class="badge bg-success text-white d-inline-flex align-items-center gap-1"
                    >
                      <IconDatabase :size="12" />
                      {{ t('config.sources.ui') }}
                    </span>
                    <span
                      v-else
                      class="badge bg-secondary-lt text-secondary"
                    >
                      {{ t('config.sources.default') }}
                    </span>

                    <span
                      v-if="entry.has_conflict"
                      class="badge bg-danger text-white d-inline-flex align-items-center gap-1"
                      :title="t('config.warnings.conflictMessage', { uiVal: entry.ui_value, envVal: entry.env_value })"
                    >
                      <IconAlertTriangle :size="12" />
                      {{ t('config.badges.conflict') }}
                    </span>
                  </div>
                </td>

                <!-- Value Edit Field -->
                <td>
                  <!-- Boolean Switch -->
                  <div
                    v-if="entry.type === 'bool'"
                    class="form-check form-switch mb-0"
                  >
                    <input
                      :id="`input-${entry.key}`"
                      class="form-check-input"
                      type="checkbox"
                      :checked="formValues[entry.key] === 'true' || formValues[entry.key] === '1'"
                      :disabled="!entry.is_editable"
                      :aria-label="entry.label || entry.key"
                      @change="onBoolChange(entry.key, ($event.target as HTMLInputElement).checked)"
                    >
                    <label
                      :for="`input-${entry.key}`"
                      class="form-check-label text-muted small ms-1"
                    >
                      {{ formValues[entry.key] === 'true' || formValues[entry.key] === '1' ? t('config.values.enabled') : t('config.values.disabled') }}
                    </label>
                  </div>

                  <!-- Select Dropdown -->
                  <div v-else-if="entry.type === 'select'">
                    <select
                      :id="`input-${entry.key}`"
                      v-model="formValues[entry.key]"
                      class="form-select form-select-sm"
                      :disabled="!entry.is_editable"
                      :aria-label="entry.label || entry.key"
                    >
                      <option
                        v-for="opt in entry.options"
                        :key="opt"
                        :value="opt"
                      >
                        {{ opt }}
                      </option>
                    </select>
                  </div>

                  <!-- Secret with Toggle -->
                  <div
                    v-else-if="entry.is_secret"
                    class="input-group input-group-sm"
                  >
                    <input
                      :id="`input-${entry.key}`"
                      v-model="formValues[entry.key]"
                      :type="showLocalPassword[entry.key] ? 'text' : 'password'"
                      class="form-control"
                      :placeholder="entry.effective_value ? '••••••••' : t('config.values.notSet')"
                      :disabled="!entry.is_editable"
                      :aria-label="entry.label || entry.key"
                    >
                    <button
                      type="button"
                      class="btn btn-outline-secondary"
                      :aria-label="showLocalPassword[entry.key] ? t('config.actions.hideSecrets') : t('config.actions.revealSecrets')"
                      @click="toggleLocalPassword(entry.key)"
                    >
                      <component
                        :is="showLocalPassword[entry.key] ? IconEyeOff : IconEye"
                        :size="14"
                      />
                    </button>
                  </div>

                  <!-- Number / Duration / String / CSV Text Input -->
                  <div v-else>
                    <input
                      :id="`input-${entry.key}`"
                      v-model="formValues[entry.key]"
                      type="text"
                      class="form-control form-control-sm font-monospace"
                      :placeholder="entry.default_value || '-'"
                      :disabled="!entry.is_editable"
                      :aria-label="entry.label || entry.key"
                    >
                  </div>

                  <!-- Conflict / ENV Notice Under Field -->
                  <div
                    v-if="entry.has_conflict"
                    class="text-danger small mt-1"
                  >
                    <IconAlertTriangle
                      :size="12"
                      class="me-1"
                    />
                    {{ t('config.warnings.conflictMessage', { uiVal: entry.ui_value, envVal: entry.env_value }) }}
                  </div>
                  <div
                    v-else-if="entry.has_env_override"
                    class="text-muted small mt-1"
                  >
                    <IconInfoCircle
                      :size="12"
                      class="me-1"
                    />
                    {{ t('config.warnings.envNotice') }}
                  </div>
                </td>

                <!-- Actions Column -->
                <td class="text-end">
                  <div class="d-inline-flex align-items-center gap-1">
                    <button
                      type="button"
                      class="btn btn-sm btn-outline-primary"
                      :disabled="!isDirty(entry.key) || savingKey === entry.key || !entry.is_editable"
                      @click="saveParam(entry)"
                    >
                      <span
                        v-if="savingKey === entry.key"
                        class="spinner-border spinner-border-sm"
                        role="status"
                      />
                      <span v-else>{{ t('config.actions.save') }}</span>
                    </button>

                    <button
                      v-if="entry.is_editable && (entry.source === 'ui' || entry.ui_value)"
                      type="button"
                      class="btn btn-sm btn-outline-danger"
                      :disabled="resettingKey === entry.key"
                      :title="t('config.actions.reset')"
                      @click="resetParam(entry)"
                    >
                      <span
                        v-if="resettingKey === entry.key"
                        class="spinner-border spinner-border-sm"
                        role="status"
                      />
                      <IconReload
                        v-else
                        :size="14"
                      />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  IconBrandDocker,
  IconDatabase,
  IconAdjustments,
  IconAlertTriangle,
  IconEye,
  IconEyeOff,
  IconRefresh,
  IconCheck,
  IconReload,
  IconSearch,
  IconInfoCircle,
  IconLock,
  IconServer,
  IconShieldLock,
  IconBell,
  IconPlugConnected,
  IconClock,
  IconShieldSearch,
} from '@tabler/icons-vue'
import { configApi } from '../api/config'
import type { ConfigEntry, ConfigSummary } from '../types/config'
import { addToast } from '../composables/useGlobalToast'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const { t } = useI18n()
const { confirm } = useConfirmDialog()

const summary = ref<ConfigSummary | null>(null)
const loading = ref(false)
const revealSecrets = ref(false)
const searchQuery = ref('')
const selectedCategory = ref('all')
const selectedSource = ref('all')

const formValues = ref<Record<string, string>>({})
const originalValues = ref<Record<string, string>>({})
const showLocalPassword = ref<Record<string, boolean>>({})

const savingKey = ref<string | null>(null)
const resettingKey = ref<string | null>(null)
const savingCategories = ref<Record<string, boolean>>({})

const availableCategories = computed(() => {
  return summary.value?.categories || [
    'server',
    'logging',
    'network',
    'database',
    'auth',
    'oidc',
    'notifications',
    'integrations',
    'retention',
    'threats',
  ]
})

function categoryIcon(category: string) {
  switch (category) {
    case 'server':
      return IconServer
    case 'logging':
      return IconAdjustments
    case 'network':
      return IconPlugConnected
    case 'database':
      return IconDatabase
    case 'auth':
      return IconShieldLock
    case 'oidc':
      return IconShieldLock
    case 'notifications':
      return IconBell
    case 'integrations':
      return IconPlugConnected
    case 'retention':
      return IconClock
    case 'threats':
      return IconShieldSearch
    default:
      return IconAdjustments
  }
}

async function loadConfig() {
  loading.value = true
  try {
    const res = await configApi.getConfig(revealSecrets.value)
    summary.value = res.data
    populateForm(res.data.entries)
  } catch {
    addToast(t('config.messages.error'), 'error')
  } finally {
    loading.value = false
  }
}

function populateForm(entries: ConfigEntry[]) {
  const forms: Record<string, string> = {}
  const originals: Record<string, string> = {}

  for (const entry of entries) {
    // If UI value exists, start with that; otherwise effective value
    const val = entry.ui_value ?? entry.effective_value ?? entry.default_value
    forms[entry.key] = val
    originals[entry.key] = val
  }

  formValues.value = forms
  originalValues.value = originals
}

function toggleRevealSecrets() {
  revealSecrets.value = !revealSecrets.value
  loadConfig()
}

function toggleLocalPassword(key: string) {
  showLocalPassword.value[key] = !showLocalPassword.value[key]
}

function onBoolChange(key: string, checked: boolean) {
  formValues.value[key] = checked ? 'true' : 'false'
}

function isDirty(key: string): boolean {
  return formValues.value[key] !== originalValues.value[key]
}

function hasDirtyInCategory(category: string): boolean {
  const entries = summary.value?.entries.filter((e) => e.category === category) || []
  return entries.some((e) => isDirty(e.key))
}

const filteredEntries = computed(() => {
  if (!summary.value) return []
  return summary.value.entries.filter((entry) => {
    // Category filter
    if (selectedCategory.value !== 'all' && entry.category !== selectedCategory.value) {
      return false
    }

    // Source filter
    if (selectedSource.value === 'conflict') {
      if (!entry.has_conflict) return false
    } else if (selectedSource.value !== 'all' && entry.source !== selectedSource.value) {
      return false
    }

    // Search query
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchKey = entry.key.toLowerCase().includes(q)
      const matchEnv = entry.env_var.toLowerCase().includes(q)
      const matchLabel = entry.label.toLowerCase().includes(q)
      const matchDesc = entry.description.toLowerCase().includes(q)
      if (!matchKey && !matchEnv && !matchLabel && !matchDesc) return false
    }

    return true
  })
})

const groupedEntries = computed(() => {
  const groups: Record<string, ConfigEntry[]> = {}
  for (const entry of filteredEntries.value) {
    if (!groups[entry.category]) {
      groups[entry.category] = []
    }
    groups[entry.category].push(entry)
  }

  const order = availableCategories.value
  return order
    .filter((cat) => groups[cat] && groups[cat].length > 0)
    .map((cat) => ({
      category: cat,
      entries: groups[cat],
    }))
})

async function saveParam(entry: ConfigEntry) {
  savingKey.value = entry.key
  try {
    const val = formValues.value[entry.key]
    const res = await configApi.updateParam(entry.key, val)
    if (res.data.warning) {
      addToast(res.data.warning, 'warning')
    } else {
      addToast(t('config.messages.updateSuccess', { key: entry.key }), 'success')
    }
    await loadConfig()
  } catch {
    addToast(t('config.messages.error'), 'error')
  } finally {
    savingKey.value = null
  }
}

async function resetParam(entry: ConfigEntry) {
  const confirmed = await confirm({
    title: t('config.actions.reset'),
    message: t('config.messages.confirmReset', { key: entry.key }),
    variant: 'warning',
  })
  if (!confirmed) {
    return
  }

  resettingKey.value = entry.key
  try {
    const res = await configApi.resetParam(entry.key)
    if (res.data.warning) {
      addToast(res.data.warning, 'warning')
    } else {
      addToast(t('config.messages.resetSuccess', { key: entry.key }), 'success')
    }
    await loadConfig()
  } catch {
    addToast(t('config.messages.error'), 'error')
  } finally {
    resettingKey.value = null
  }
}

async function saveCategory(category: string) {
  savingCategories.value[category] = true
  try {
    const entries = summary.value?.entries.filter((e) => e.category === category && isDirty(e.key)) || []
    const updates: Record<string, string> = {}
    for (const e of entries) {
      updates[e.key] = formValues.value[e.key]
    }
    const res = await configApi.updateBulk(updates)
    if (res.data.warnings && res.data.warnings.length > 0) {
      addToast(res.data.warnings[0], 'warning')
    } else {
      addToast(t('config.messages.bulkSuccess'), 'success')
    }
    await loadConfig()
  } catch {
    addToast(t('config.messages.error'), 'error')
  } finally {
    savingCategories.value[category] = false
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.rotate-spinner {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  100% {
    transform: rotate(360deg);
  }
}
</style>

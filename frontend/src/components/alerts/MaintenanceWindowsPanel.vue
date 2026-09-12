<template>
  <div class="card">
    <div class="card-header d-flex flex-column flex-lg-row align-items-start align-items-lg-center justify-content-between gap-3">
      <div>
        <h3 class="card-title mb-1">
          {{ t('alerts.maintenanceTitle') }}
        </h3>
        <div class="text-muted small">
          {{ t('alerts.maintenanceSubtitle') }}
        </div>
      </div>
      <button
        v-if="isAdmin"
        type="button"
        class="btn btn-primary btn-sm"
        @click="showForm = !showForm"
      >
        <IconPlus
          :size="14"
          class="icon me-1"
        />
        {{ t('alerts.newWindowButton') }}
      </button>
    </div>

    <div
      v-if="showForm"
      class="card-body border-bottom"
    >
      <form @submit.prevent="onSubmit">
        <div class="row g-3">
          <div class="col-12 col-lg-4">
            <label class="form-label">{{ t('alerts.hostLabel') }}</label>
            <select
              v-model="form.hostId"
              class="form-select"
            >
              <option value="">
                {{ t('alerts.allHostsAdminOption') }}
              </option>
              <option
                v-for="h in hosts"
                :key="h.id"
                :value="h.id"
              >
                {{ h.name }}
              </option>
            </select>
            <div class="form-hint">
              {{ t('alerts.allHostsHint') }}
            </div>
          </div>
          <div class="col-12 col-lg-4">
            <label class="form-label required">{{ t('alerts.startsAtLabel') }}</label>
            <input
              v-model="form.startsAt"
              type="datetime-local"
              class="form-control"
              required
            >
          </div>
          <div class="col-12 col-lg-4">
            <label class="form-label required">{{ t('alerts.endsAtLabel') }}</label>
            <input
              v-model="form.endsAt"
              type="datetime-local"
              class="form-control"
              required
            >
          </div>
          <div class="col-12">
            <label class="form-label required">{{ t('alerts.reasonLabel') }}</label>
            <input
              v-model="form.reason"
              type="text"
              class="form-control"
              :placeholder="t('alerts.reasonPlaceholder')"
              required
            >
          </div>
        </div>

        <div
          v-if="saveError"
          class="alert alert-danger mt-3 mb-0"
        >
          {{ saveError }}
        </div>

        <div class="d-flex gap-2 mt-3">
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="saving"
          >
            <span
              v-if="saving"
              class="spinner-border spinner-border-sm me-2"
            />
            {{ t('alerts.createButton') }}
          </button>
          <button
            type="button"
            class="btn btn-outline-secondary"
            @click="showForm = false"
          >
            {{ t('alerts.cancelButton') }}
          </button>
        </div>
      </form>
    </div>

    <div
      v-if="error"
      class="alert alert-danger m-3 mb-0"
    >
      {{ error }}
    </div>

    <LoadingSkeleton
      v-if="loading && !fetched"
      variant="table"
      :lines="3"
      class="m-3"
    />

    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('alerts.scopeColumn') }}</th>
            <th>{{ t('alerts.reasonLabel') }}</th>
            <th>{{ t('alerts.startsAtLabel') }}</th>
            <th>{{ t('alerts.endsAtLabel') }}</th>
            <th>{{ t('alerts.createdByColumn') }}</th>
            <th class="text-end">
              {{ t('alerts.actionsColumn') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="windows.length === 0">
            <td colspan="6">
              <EmptyState
                :title="t('alerts.noWindowsTitle')"
                :subtitle="t('alerts.noWindowsSubtitle')"
              />
            </td>
          </tr>
          <tr
            v-for="w in windows"
            :key="w.id"
          >
            <td>
              <span
                v-if="!w.host_id"
                class="badge bg-orange-lt text-orange"
              >{{ t('alerts.allHostsBadge') }}</span>
              <span v-else>{{ w.host_name || w.host_id }}</span>
            </td>
            <td>{{ w.reason }}</td>
            <td>{{ formatLocaleDateTime(w.starts_at) }}</td>
            <td>{{ formatLocaleDateTime(w.ends_at) }}</td>
            <td class="text-muted">
              {{ w.created_by }}
            </td>
            <td class="text-end">
              <button
                type="button"
                class="btn btn-icon btn-sm btn-ghost-danger"
                :title="t('alerts.deleteTooltip')"
                :aria-label="t('alerts.deleteWindowAriaLabel')"
                @click="remove(w)"
              >
                <IconTrash :size="16" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPlus, IconTrash } from '@tabler/icons-vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { useMaintenanceWindows } from '../../composables/useMaintenanceWindows'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { useHostsStore } from '../../stores/hosts'
import { storeToRefs } from 'pinia'

defineProps<{ isAdmin: boolean }>()

const { t } = useI18n()
const { formatLocaleDateTime } = useDateFormatter()
const hostsStore = useHostsStore()
const { hosts } = storeToRefs(hostsStore)

const { windows, loading, fetched, error, saving, saveError, load, create, remove } = useMaintenanceWindows()

const showForm = ref(false)
const form = reactive({ hostId: '', reason: '', startsAt: '', endsAt: '' })

async function onSubmit(): Promise<void> {
  const ok = await create(form.hostId || null, {
    reason: form.reason,
    starts_at: new Date(form.startsAt).toISOString(),
    ends_at: new Date(form.endsAt).toISOString(),
  })
  if (ok) {
    showForm.value = false
    form.hostId = ''
    form.reason = ''
    form.startsAt = ''
    form.endsAt = ''
  }
}

onMounted(async () => {
  await Promise.all([load(), hostsStore.fetchHosts()])
})
</script>

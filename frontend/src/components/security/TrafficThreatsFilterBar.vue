<template>
  <div class="card mb-4">
    <div class="card-body d-flex flex-wrap gap-2 align-items-end traffic-filters">
      <div class="traffic-filter-field">
        <label class="form-label mb-1">{{ t('security.sourceLabel') }}</label>
        <div class="input-group input-group-sm">
          <select
            v-model="source"
            class="form-select form-select-sm"
            :disabled="loading"
            style="min-width: 9rem;"
          >
            <option value="">
              {{ t('security.allSourcesOption') }}
            </option>
            <option value="npm">
              npm
            </option>
            <option value="nginx">
              nginx
            </option>
            <option value="apache">
              apache
            </option>
            <option value="caddy">
              caddy
            </option>
          </select>
          <span
            v-if="loading"
            class="input-group-text px-2"
          >
            <span
              class="spinner-border"
              style="width:.75rem;height:.75rem;border-width:2px"
            />
          </span>
          <span
            v-else-if="sourceHasNoData"
            class="input-group-text px-2 text-warning"
            :title="t('security.noDataForSourceTooltip')"
          >
            <IconAlertTriangle :size="14" />
          </span>
        </div>
      </div>

      <div class="traffic-filter-field">
        <label class="form-label mb-1">{{ t('security.hostLabel') }}</label>
        <select
          v-model="hostId"
          class="form-select form-select-sm"
          :disabled="loading"
          style="min-width: 12rem;"
        >
          <option value="">
            {{ t('security.allHostsOption') }}
          </option>
          <option
            v-for="h in hostsStore.hosts"
            :key="h.id"
            :value="h.id"
          >
            {{ h.name || h.hostname || h.ip_address }}
          </option>
        </select>
      </div>

      <button
        type="button"
        class="btn btn-primary btn-sm traffic-refresh-btn"
        :disabled="loading"
        @click="$emit('refresh')"
      >
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm me-1"
        />
        {{ t('security.refreshButton') }}
      </button>

      <div class="traffic-filter-field ms-auto">
        <label class="form-label mb-1">{{ t('security.searchDomainOrIpLabel') }}</label>
        <div class="input-group input-group-sm">
          <input
            v-model="searchTerm"
            type="text"
            class="form-control form-control-sm"
            :placeholder="t('security.searchPlaceholderExample')"
            style="min-width: 16rem;"
            @keyup.enter="$emit('search')"
          >
          <button
            type="button"
            class="btn btn-outline-secondary btn-sm"
            :disabled="!searchTerm.trim()"
            @click="$emit('search')"
          >
            {{ t('security.viewRequestsButton') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconAlertTriangle } from '@tabler/icons-vue'
import { useHostsStore } from '../../stores/hosts'

const { t } = useI18n()

withDefaults(defineProps<{
  loading?: boolean
  sourceHasNoData?: boolean
}>(), {
  loading: false,
  sourceHasNoData: false,
})

defineEmits<{
  refresh: []
  search: []
}>()

const source = defineModel<string>('source', { required: true })
const hostId = defineModel<string>('hostId', { required: true })
const searchTerm = defineModel<string>('searchTerm', { required: true })

const hostsStore = useHostsStore()
</script>

<style scoped>
@media (max-width: 992px) {
  .traffic-filters {
    align-items: stretch !important;
  }

  .traffic-filter-field {
    flex: 1 1 220px;
  }

  .traffic-filter-field .form-select,
  .traffic-filter-field .form-control {
    min-width: 0 !important;
    width: 100%;
  }

  .traffic-refresh-btn {
    width: 100%;
  }
}
</style>

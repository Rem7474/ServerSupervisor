<template>
  <DataToolbar
    searchable
    :search="search"
    :search-placeholder="t('docker.searchContainerPlaceholder')"
    @update:search="emit('update:search', $event)"
  >
    <template #bottom>
      <div class="row g-3">
        <div class="col-6 col-md-6 col-lg-4">
          <select
            v-model="hostFilter"
            class="form-select"
          >
            <option value="">
              {{ t('docker.allHosts') }}
            </option>
            <option
              v-for="h in uniqueHosts"
              :key="h"
              :value="h"
            >
              {{ h }}
            </option>
          </select>
        </div>
        <div class="col-6 col-md-6 col-lg-4">
          <select
            v-model="stateFilter"
            class="form-select"
          >
            <option value="">
              {{ t('docker.allStates') }}
            </option>
            <option value="running">
              {{ t('docker.running') }}
            </option>
            <option value="restarting">
              {{ t('docker.stateRestarting') }}
            </option>
            <option value="paused">
              {{ t('docker.statePaused') }}
            </option>
            <option value="created">
              {{ t('docker.stateCreated') }}
            </option>
            <option value="exited">
              {{ t('docker.stopped') }}
            </option>
            <option value="dead">
              {{ t('docker.stateDead') }}
            </option>
          </select>
        </div>
        <div class="col-12 col-md-12 col-lg-4">
          <select
            v-model="composeFilter"
            class="form-select"
          >
            <option value="">
              {{ t('docker.allContainers') }}
            </option>
            <option value="compose">
              Docker Compose
            </option>
            <option value="standalone">
              Standalone
            </option>
          </select>
        </div>
      </div>
    </template>
  </DataToolbar>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DataToolbar from '../common/DataToolbar.vue'

defineProps<{
  search: string
  uniqueHosts: string[]
}>()

const emit = defineEmits<{
  (e: 'update:search', value: string): void
}>()

const hostFilter = defineModel<string>('hostFilter', { required: true })
const stateFilter = defineModel<string>('stateFilter', { required: true })
const composeFilter = defineModel<string>('composeFilter', { required: true })

const { t } = useI18n()
</script>

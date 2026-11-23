<template>
  <div class="row row-cards mb-3">
    <div class="col-6 col-md-4">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('monitoring.downProbesLabel') }}
          </div>
          <div
            class="h2 mb-0 mt-1"
            :class="downCount > 0 ? 'text-danger' : 'text-success'"
          >
            {{ downCount }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-md-4">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('monitoring.certsExpiringLabel') }}
          </div>
          <div
            class="h2 mb-0 mt-1"
            :class="expiringCount > 0 ? 'text-warning' : 'text-success'"
          >
            {{ expiringCount }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-md-4">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('monitoring.totalMonitoredLabel') }}
          </div>
          <div class="h2 mb-0 mt-1">
            {{ totalMonitored }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  downCount: number
  expiringCount: number
  totalMonitored: number
}>()

const { t } = useI18n()
</script>

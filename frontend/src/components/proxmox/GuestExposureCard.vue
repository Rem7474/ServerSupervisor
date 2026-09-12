<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
      <h3 class="card-title mb-0">
        {{ t('proxmox.domainsExposureTitle') }}
      </h3>
      <div class="d-flex align-items-center gap-2 flex-wrap">
        <div class="d-flex gap-2 flex-wrap">
          <button
            v-for="p in PERIODS"
            :key="p.value"
            type="button"
            class="btn btn-sm"
            :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
            @click="period = p.value"
          >
            {{ t(p.labelKey) }}
          </button>
        </div>
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm text-muted"
        />
      </div>
    </div>
    <div class="card-body">
      <EmptyState
        v-if="!ipsKnown"
        :title="t('proxmox.noIpDetectedTitle')"
      />
      <ExposureDomainsPanel
        v-else
        :exposure="exposure"
        :loading="loading"
        :error="error"
        :period="period"
        :period-label="periodLabel"
        :subject-label="t('proxmox.thisGuestSubjectLabel')"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../../api'
import ExposureDomainsPanel from '../security/ExposureDomainsPanel.vue'
import EmptyState from '../EmptyState.vue'
import type { HostExposure } from '../../types/host'
import { getApiErrorMessage } from '../../api/client'

const { t } = useI18n()

const props = defineProps<{ guestId: string }>()

const PERIODS = [
  { value: '24h', labelKey: 'proxmox.timeRange24h' },
  { value: '168h', labelKey: 'proxmox.timeRange7d' },
  { value: '720h', labelKey: 'proxmox.timeRange30d' },
]

const exposure = ref<HostExposure | null>(null)
const loading = ref(false)
const error = ref('')
const period = ref('24h')

const periodLabel = computed(() => {
  const found = PERIODS.find((p) => p.value === period.value)
  return found ? t(found.labelKey) : period.value
})
// GuestExposure returns ip_address = '' when the guest has no live/static IP
// at all (guest stopped, no QEMU agent, ...) — distinct from "has an IP but
// no NPM domain routes to it", which ExposureDomainsPanel already renders as
// its own empty state.
const ipsKnown = computed(() => !!exposure.value?.ip_address || loading.value)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getProxmoxGuestExposure(props.guestId, period.value)
    exposure.value = res.data
  } catch (err: unknown) {
    error.value = getApiErrorMessage(err, t('proxmox.loadErrorGeneric'))
  } finally {
    loading.value = false
  }
}

watch(period, load)
onMounted(load)
</script>

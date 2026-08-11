<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Exposition"
      :interval-sec="EXPOSURE_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="d-flex align-items-center gap-2 mb-3">
      <div class="d-flex gap-2 flex-wrap">
        <button
          v-for="p in PERIODS"
          :key="p.value"
          type="button"
          class="btn btn-sm"
          :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="period = p.value"
        >
          {{ p.label }}
        </button>
      </div>
      <span
        v-if="loading"
        class="spinner-border spinner-border-sm text-muted"
      />
    </div>

    <ExposureDomainsPanel
      :exposure="exposure"
      :loading="loading"
      :error="error"
      :period="period"
      :period-label="periodLabel"
      subject-label="cet hôte"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import api from '../../api'
import ExposureDomainsPanel from '../security/ExposureDomainsPanel.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import type { HostExposure } from '../../types/host'
import { getApiErrorMessage } from '../../api/client'

const props = defineProps<{ hostId: string }>()

const emit = defineEmits<{ (e: 'loaded', domainCount: number): void }>()

const PERIODS = [
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]

const EXPOSURE_REFRESH_SEC = 60

const exposure = ref<HostExposure | null>(null)
const loading = ref(false)
const error = ref('')
const period = ref('24h')
const autoRefresh = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null

const periodLabel = computed(() => PERIODS.find((p) => p.value === period.value)?.label ?? period.value)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getHostExposure(props.hostId, period.value)
    exposure.value = res.data
    lastUpdatedAt.value = new Date()
    emit('loaded', exposure.value?.domains.length ?? 0)
  } catch (err: unknown) {
    error.value = getApiErrorMessage(err, 'Erreur de chargement')
  } finally {
    loading.value = false
  }
}

function startRefreshTimer(): void {
  stopRefreshTimer()
  refreshTimer = setInterval(() => {
    if (autoRefresh.value) load()
  }, EXPOSURE_REFRESH_SEC * 1000)
}

function stopRefreshTimer(): void {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}

watch(period, load)
onMounted(() => {
  load()
  startRefreshTimer()
})
onUnmounted(stopRefreshTimer)
</script>

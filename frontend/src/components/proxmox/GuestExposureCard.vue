<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
      <h3 class="card-title mb-0">
        Domaines &amp; exposition
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
            {{ p.label }}
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
        title="Aucune adresse IP détectée pour ce guest (agent invité PVE absent ou guest arrêté)."
      />
      <ExposureDomainsPanel
        v-else
        :exposure="exposure"
        :loading="loading"
        :error="error"
        :period="period"
        :period-label="periodLabel"
        subject-label="ce guest"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import api from '../../api'
import ExposureDomainsPanel from '../security/ExposureDomainsPanel.vue'
import EmptyState from '../EmptyState.vue'
import type { HostExposure } from '../../types/host'
import { getApiErrorMessage } from '../../api/client'

const props = defineProps<{ guestId: string }>()

const PERIODS = [
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]

const exposure = ref<HostExposure | null>(null)
const loading = ref(false)
const error = ref('')
const period = ref('24h')

const periodLabel = computed(() => PERIODS.find((p) => p.value === period.value)?.label ?? period.value)
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
    error.value = getApiErrorMessage(err, 'Erreur de chargement')
  } finally {
    loading.value = false
  }
}

watch(period, load)
onMounted(load)
</script>

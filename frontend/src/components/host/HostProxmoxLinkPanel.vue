<template>
  <!-- Proxmox link panel -->
  <div
    v-if="proxmoxLink && proxmoxLink.status !== 'ignored'"
    class="card mb-3 border-0 shadow-sm"
  >
    <div class="card-body py-2 px-3 d-flex flex-wrap align-items-center gap-3">
      <!-- Guest info -->
      <div class="d-flex align-items-center gap-2">
        <BadgePill
          text="Proxmox"
          tone="orange"
          compact
        />
        <span class="fw-medium">{{ proxmoxLink.guest_name || `VMID ${proxmoxLink.vmid}` }}</span>
        <span class="text-muted small">({{ proxmoxLink.guest_type?.toUpperCase() }} · {{ proxmoxLink.node_name }})</span>
        <!-- This panel only has the link's own metadata (status, metrics
             source) — no running/stopped state, so power actions can't
             live here without a second API call. Send to the guest's own
             page instead, which already has them (start/shutdown/reboot,
             auto-refresh, full metrics). -->
        <router-link
          :to="`/proxmox/guests/${proxmoxLink.guest_id}`"
          class="text-decoration-none small d-inline-flex align-items-center gap-1"
          :title="t('host.viewGuestTooltip')"
        >
          {{ t('host.viewGuestLabel') }}
          <IconExternalLink :size="14" />
        </router-link>
      </div>

      <!-- Status badge + suggestion actions -->
      <div class="d-flex align-items-center gap-2">
        <BadgePill
          v-if="proxmoxLink.status === 'suggested'"
          :text="t('host.suggestionBadge')"
          tone="warning"
          compact
        />
        <BadgePill
          v-else
          :text="t('host.linkedBadge')"
          tone="success"
          compact
        />
        <template v-if="proxmoxLink.status === 'suggested'">
          <button
            type="button"
            class="btn btn-sm btn-success"
            :disabled="linkSaving"
            @click="$emit('confirm')"
          >
            {{ t('host.confirmLabel') }}
          </button>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="linkSaving"
            @click="$emit('ignore')"
          >
            {{ t('host.ignoreLabel') }}
          </button>
        </template>
      </div>

      <!-- Metrics source selector (shown only when confirmed) -->
      <div
        v-if="proxmoxLink.status === 'confirmed'"
        class="d-flex align-items-center gap-2 ms-auto"
      >
        <label
          :for="`${uid}-1`"
          class="form-label mb-0 text-muted small"
        >{{ t('host.metricsSourceLabel') }}</label>
        <select
          :id="`${uid}-1`"
          class="form-select form-select-sm"
          style="width:auto"
          :value="proxmoxLink.metrics_source"
          @change="$emit('change-metrics-source', ($event.target as HTMLSelectElement).value as 'agent' | 'proxmox' | 'auto')"
        >
          <option value="auto">
            {{ t('host.automaticOption') }}
          </option>
          <option value="agent">
            Agent
          </option>
          <option value="proxmox">
            Proxmox
          </option>
        </select>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-outline-danger"
          :disabled="linkSaving"
          :title="t('host.deleteLinkTooltip')"
          :aria-label="t('host.deleteLinkTooltip')"
          @click="$emit('delete')"
        >
          <IconTrash
            :size="16"
            class="icon icon-sm"
          />
        </button>
      </div>

      <!-- Guest live metrics (source = proxmox) -->
      <template v-if="proxmoxLink.status === 'confirmed' && proxmoxLink.metrics_source !== 'agent'">
        <div class="d-flex align-items-center gap-3 ms-2 border-start ps-3">
          <div class="text-muted small">
            CPU <strong :class="getMetricColorClass((proxmoxLink.cpu_usage ?? 0) * 100)">{{ ((proxmoxLink.cpu_usage ?? 0) * 100).toFixed(1) }}%</strong>
          </div>
          <div class="text-muted small">
            RAM
            <strong
              v-if="linkRamPct != null"
              :class="getMetricColorClass(linkRamPct)"
            >{{ linkRamPct.toFixed(1) }}%</strong> <span class="text-body">({{ formatBytesLink(proxmoxLink.mem_usage) }} / {{ formatBytesLink(proxmoxLink.mem_alloc) }})</span>
          </div>
          <div
            v-if="linkDiskPct != null"
            class="text-muted small"
          >
            {{ t('host.diskLabel') }} <strong :class="getMetricColorClass(linkDiskPct)">{{ linkDiskPct.toFixed(1) }}%</strong>
          </div>
        </div>
      </template>
    </div>
  </div>

  <!-- No link banner + manual link button -->
  <div
    v-else-if="!proxmoxLink && showLinkButton"
    class="d-flex align-items-center gap-2 mb-3"
  >
    <button
      type="button"
      class="btn btn-sm btn-outline-warning"
      @click="$emit('open-form')"
    >
      <IconLink
        :size="16"
        class="icon icon-sm me-1"
      />
      {{ t('host.linkToProxmoxLabel') }}
    </button>
  </div>

  <!-- Manual link form -->
  <div
    v-if="showLinkForm"
    class="card mb-3"
  >
    <div class="card-body">
      <div class="fw-medium mb-2">
        {{ t('host.linkHostToGuestLabel') }}
      </div>
      <div
        v-if="linkCandidatesLoading"
        class="text-muted small"
      >
        {{ t('host.loadingLabel') }}
      </div>
      <div
        v-else-if="linkCandidates.length === 0"
        class="text-muted small"
      >
        {{ t('host.noGuestAvailable') }}
      </div>
      <div
        v-else
        class="d-flex align-items-center gap-2"
      >
        <select
          :id="`${uid}-4`"
          v-model="selectedCandidate"
          :aria-label="t('host.linkHostToGuestLabel')"
          class="form-select form-select-sm candidate-select"
        >
          <option value="">
            {{ t('host.chooseGuestOption') }}
          </option>
          <option
            v-for="g in linkCandidates"
            :key="g.id"
            :value="g.id"
          >
            {{ g.name || `VMID ${g.vmid}` }} ({{ g.guest_type?.toUpperCase() }} · {{ g.node_name }})
          </option>
        </select>
        <button
          type="button"
          class="btn btn-sm btn-primary"
          :disabled="!selectedCandidate || linkSaving"
          @click="$emit('create-link')"
        >
          {{ t('host.linkButtonLabel') }}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-outline-secondary"
          @click="showLinkForm = false; selectedCandidate = ''"
        >
          {{ t('host.cancel') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import { IconExternalLink, IconLink, IconTrash } from '@tabler/icons-vue'
import { useI18n } from 'vue-i18n'
import BadgePill from '../common/BadgePill.vue'
import { getMetricColorClass } from '../../utils/metricColor'
import type { HostRecord } from '../../types/hostDetail'

const uid = useId()

const props = defineProps<{
  proxmoxLink: HostRecord | null
  linkSaving: boolean
  showLinkButton: boolean
  linkCandidates: HostRecord[]
  linkCandidatesLoading: boolean
  formatBytesLink: (bytes: number) => string
}>()

const showLinkForm = defineModel<boolean>('showLinkForm', { required: true })
const selectedCandidate = defineModel<string>('selectedCandidate', { required: true })

defineEmits<{
  confirm: []
  ignore: []
  'change-metrics-source': [source: 'agent' | 'proxmox' | 'auto']
  delete: []
  'open-form': []
  'create-link': []
}>()

const { t } = useI18n()

// CPU/RAM/disk % for the linked guest's live metrics, colored with the same
// shared thresholds as the dashboard/Proxmox views (see metricColor.ts) —
// mirrors ProxmoxGuestView's own KPI cards for the same guest.
const linkRamPct = computed(() => {
  const link = props.proxmoxLink
  if (!link?.mem_alloc) return null
  return (link.mem_usage / link.mem_alloc) * 100
})

const linkDiskPct = computed(() => {
  const link = props.proxmoxLink
  if (!link?.disk_alloc) return null
  return (link.disk_usage / link.disk_alloc) * 100
})
</script>

<style scoped>
.candidate-select {
  max-width: 320px;
}
</style>

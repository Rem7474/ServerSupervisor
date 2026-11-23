<template>
  <div class="page-header mb-3">
    <div class="d-flex flex-column flex-lg-row align-items-lg-start justify-content-between gap-3">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            {{ t('nav.sections.control.items.dashboard') }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ t('host.hostBreadcrumb') }}</span>
        </div>
        <h2 class="page-title">
          {{ host?.name || host?.hostname || t('host.loadingLabel') }}
        </h2>
        <div class="text-secondary">
          {{ host?.hostname || t('host.notConnected') }} - {{ host?.os || t('host.unknownOs') }} - {{ host?.ip_address }}
          <span v-if="host?.last_seen">- {{ t('host.lastActivityLabel') }} <RelativeTime :date="host.last_seen" /></span>
        </div>
        <div class="d-flex flex-wrap align-items-center gap-2 mt-2">
          <span
            v-if="host"
            :class="hostStatusClass(host.status)"
            :aria-label="t('host.hostStatusAriaLabel', { status: formatHostStatus(host.status) })"
          >
            <span :class="['status-dot', host.status === 'online' ? 'status-dot-animated' : '']" />
            {{ formatHostStatus(host.status) }}
          </span>
          <BadgePill
            v-if="host?.agent_version"
            :tone="isAgentUpToDate(host.agent_version) ? 'success' : 'warning'"
            :text="t('host.agentVersionBadge', { version: host.agent_version })"
            :aria-label="isAgentUpToDate(host.agent_version) ? t('host.agentUpToDateAriaSuffix', { version: host.agent_version }) : t('host.agentUpdateAvailableAriaSuffix', { version: host.agent_version })"
            :title="isAgentUpToDate(host.agent_version) ? t('host.agentUpToDateTitle') : t('host.agentUpdateAvailableTitle')"
            compact
          />
        </div>
      </div>
      <div class="d-flex flex-wrap align-items-center justify-content-lg-end gap-2">
        <button
          type="button"
          class="btn btn-outline-secondary"
          @click="$emit('edit')"
        >
          <IconPencil
            :size="16"
            class="icon me-1"
          />
          {{ t('host.editLabel') }}
        </button>
        <button
          v-if="canUpdateAgent"
          type="button"
          class="btn btn-outline-primary"
          :disabled="agentUpdateLoading"
          @click="$emit('update-agent')"
        >
          <IconRefresh
            :size="16"
            class="icon me-1"
          />
          {{ t('host.updateAgentLabel') }}
        </button>
        <button
          v-if="isAdmin"
          type="button"
          class="btn btn-outline-danger"
          @click="$emit('delete')"
        >
          <IconTrash
            :size="16"
            class="icon me-1"
          />
          {{ t('host.deleteLabel') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconPencil, IconRefresh, IconTrash } from '@tabler/icons-vue'
import { useI18n } from 'vue-i18n'
import RelativeTime from '../RelativeTime.vue'
import BadgePill from '../common/BadgePill.vue'
import { formatHostStatus, hostStatusClass } from '../../utils/formatHostStatus'
import type { HostRecord } from '../../types/hostDetail'

defineProps<{
  host: HostRecord | null
  isAdmin: boolean
  canUpdateAgent: boolean
  agentUpdateLoading: boolean
  isAgentUpToDate: (version: string) => boolean
}>()

defineEmits<{
  edit: []
  'update-agent': []
  delete: []
}>()

const { t } = useI18n()
</script>

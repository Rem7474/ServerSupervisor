<template>
  <div class="row row-cards mb-3">
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            APT
          </div>
          <div
            class="h2 mb-0 mt-1"
            :class="(aptStatus?.security_updates || 0) > 0 ? 'text-danger' : (aptStatus?.pending_packages || 0) > 0 ? 'text-warning' : 'text-success'"
          >
            {{ aptStatus?.pending_packages || 0 }}
          </div>
          <div class="text-secondary small">
            {{ aptStatus?.security_updates || 0 }} {{ t('host.securityUpdatesSuffix') }}
            <a
              href="#"
              class="ms-1 text-decoration-none"
              @click.prevent="$emit('go-tab', 'apt')"
            >{{ t('host.seeLabel') }}</a>
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('host.dockerContainersLabel') }}
          </div>
          <div class="h2 mb-0 mt-1">
            {{ dockerRunningCount }} / {{ containersCount }}
          </div>
          <div class="text-secondary small">
            {{ t('host.runningLabel') }}
            <a
              href="#"
              class="ms-1 text-decoration-none"
              @click.prevent="$emit('go-tab', 'docker')"
            >{{ t('host.seeLabel') }}</a>
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('host.scheduledTasksTabLabel') }}
          </div>
          <div class="h2 mb-0 mt-1">
            {{ tasksCount }}
          </div>
          <div class="text-secondary small">
            <a
              href="#"
              class="text-decoration-none"
              @click.prevent="$emit('go-tab', 'planifiees')"
            >{{ t('host.seeLabel') }}</a>
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('host.recentCommandsLabel') }}
          </div>
          <div class="h2 mb-0 mt-1">
            {{ commandsCount }}
          </div>
          <div class="text-secondary small">
            <a
              href="#"
              class="text-decoration-none"
              @click.prevent="$emit('go-commands')"
            >{{ t('host.seeLabel') }}</a>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div class="card mb-3">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        <IconAlertTriangle
          :size="24"
          class="icon me-1"
        />
        {{ t('host.activeAlertsTitle') }}
      </h3>
      <router-link
        to="/alerts?tab=incidents"
        class="btn btn-sm btn-outline-secondary"
      >
        {{ t('host.allAlertsLabel') }}
      </router-link>
    </div>
    <div
      v-if="incidentsLoading"
      class="card-body"
    >
      <LoadingSkeleton variant="list" />
    </div>
    <div
      v-else-if="!incidents.length"
      class="card-body"
    >
      <EmptyState :title="t('host.noActiveAlerts')" />
    </div>
    <div
      v-else
      class="list-group list-group-flush"
    >
      <router-link
        v-for="item in incidents"
        :key="item.id"
        :to="alertsLink"
        class="list-group-item list-group-item-action d-flex align-items-center gap-2"
      >
        <span
          class="badge"
          :class="item.severity === 'crit' ? 'bg-danger-lt text-danger' : 'bg-warning-lt text-warning'"
        >{{ item.severity }}</span>
        <span class="flex-grow-1">{{ item.rule_name || item.metric }}</span>
        <RelativeTime :date="item.triggered_at || ''" />
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconAlertTriangle } from '@tabler/icons-vue'
import { useI18n } from 'vue-i18n'
import RelativeTime from '../RelativeTime.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import type { HostRecord } from '../../types/hostDetail'

defineProps<{
  aptStatus: HostRecord | null
  dockerRunningCount: number
  containersCount: number
  tasksCount: number
  commandsCount: number
  incidents: HostRecord[]
  incidentsLoading: boolean
  alertsLink: { path: string; query: Record<string, string> }
}>()

defineEmits<{
  'go-tab': [tab: string]
  'go-commands': []
}>()

const { t } = useI18n()
</script>

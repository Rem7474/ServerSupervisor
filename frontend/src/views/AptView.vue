<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          {{ t('nav.sections.control.items.dashboard') }}
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>APT</span>
      </div>
      <div class="d-flex align-items-center justify-content-between flex-wrap gap-2">
        <h2 class="page-title">
          {{ t('apt.pageTitle') }}
        </h2>
        <router-link
          to="/audit?module=apt"
          class="btn btn-sm btn-outline-secondary"
        >
          <IconList
            :size="16"
            class="icon icon-sm me-1"
          />
          {{ t('apt.commandHistory') }}
        </router-link>
      </div>
      <div class="text-secondary">
        {{ t('apt.pageDescription') }}
      </div>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      :data-stale-alert="dataStaleAlert"
      @reconnect="reconnect"
      @dismiss-stale-alert="dataStaleAlert = false"
    />

    <AptToolbar
      v-model:search="hostSearch"
      v-model:quick-filter="hostQuickFilter"
      v-model:sort-key="hostSortKey"
      v-model:sort-dir="hostSortDir"
      v-model:all-selected="selectAll"
      :filter-options="hostFilterOptions"
      :can-run-apt="canRunApt"
      :selected-count="selectedHosts.length"
      :bulk-loading="aptBulkLoading"
      :filtered-count="filteredHosts.length"
      :outdated-count="outdatedSelectedHosts.length"
      :agent-update-loading="bulkAgentUpdateLoading"
      @bulk-cmd="bulkAptCmd"
      @agent-update-cmd="bulkAgentUpdate"
    />

    <div class="side-layout">
      <div class="side-main">
        <div class="row row-cards">
          <template v-if="wsStatus === 'connecting' && hosts.length === 0">
            <div
              v-for="n in 3"
              :key="`sk-${n}`"
              class="col-12"
            >
              <LoadingSkeleton
                variant="card"
                :lines="4"
              />
            </div>
          </template>
          <div
            v-else-if="filteredHosts.length === 0"
            class="col-12"
          >
            <div class="card">
              <div class="card-body">
                <EmptyState :title="t('apt.noHostsMatchFilters')" />
              </div>
            </div>
          </div>

          <div
            v-for="host in filteredHosts"
            :key="host.id"
            class="col-12"
          >
            <AptHostCard
              :host="host"
              :apt-status="aptStatuses[host.id]"
              :history="aptHistories[host.id]"
              :expanded="!!hostExpanded[host.id]"
              :selected="selectedHosts.includes(host.id)"
              :can-run-apt="canRunApt"
              :cmd-loading="hostCmdLoading[host.id]"
              :enriching="!!enrichingHosts[host.id]"
              :uu-status="uuStatuses[host.id]"
              :agent-outdated="isAgentOutdated(host)"
              :latest-agent-version="latestAgentVersion"
              @update:selected="val => toggleSelected(host.id, val)"
              @update:expanded="val => hostExpanded[host.id] = val"
              @run-cmd="cmd => runAptCmdForHost(host, cmd)"
              @schedule="openScheduleModal(host)"
              @watch-command="cmd => watchCommand(cmd, host)"
            />
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="liveCommand"
        :show="showConsole"
        :title="t('apt.consoleLive')"
        :empty-text="t('apt.noActiveConsole')"
        wrapper-class="side-panel"
        @open="showConsole = true"
        @close="closeLiveConsole"
      />
    </div>

    <AptScheduleModal
      :host="scheduleHost"
      @close="scheduleHost = null"
      @created="onScheduleCreated"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconList } from '@tabler/icons-vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import AptToolbar from '../components/apt/AptToolbar.vue'
import AptHostCard from '../components/apt/AptHostCard.vue'
import AptScheduleModal from '../components/apt/AptScheduleModal.vue'
import { useApt } from '../composables/useApt'
import { addToast } from '../composables/useGlobalToast'

const { t } = useI18n()

const {
  hosts,
  selectedHosts,
  hostExpanded,
  aptStatuses,
  aptHistories,
  uuStatuses,
  latestAgentVersion,
  hostCmdLoading,
  enrichingHosts,
  canRunApt,
  selectAll,
  toggleSelected,
  scheduleHost,
  openScheduleModal,
  showConsole,
  liveCommand,
  aptBulkLoading,
  hostSearch,
  hostQuickFilter,
  hostSortKey,
  hostSortDir,
  hostFilterOptions,
  filteredHosts,
  isAgentOutdated,
  outdatedSelectedHosts,
  bulkAgentUpdateLoading,
  watchCommand,
  closeLiveConsole,
  runAptCmdForHost,
  bulkAptCmd,
  bulkAgentUpdate,
  wsStatus,
  wsError,
  retryCount,
  dataStaleAlert,
  reconnect,
} = useApt()

function onScheduleCreated(): void {
  addToast(t('apt.scheduledTaskCreated'), 'success')
}
</script>


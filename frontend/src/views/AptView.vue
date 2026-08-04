<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>APT</span>
      </div>
      <div class="d-flex align-items-center justify-content-between flex-wrap gap-2">
        <h2 class="page-title">
          APT — Mises à jour système
        </h2>
        <router-link
          to="/audit?module=apt"
          class="btn btn-sm btn-outline-secondary"
        >
          <IconList
            :size="16"
            class="icon icon-sm me-1"
          />
          Historique des commandes
        </router-link>
      </div>
      <div class="text-secondary">
        Gérer les mises à jour APT sur tous les hôtes
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
      @bulk-cmd="bulkAptCmd"
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
                <EmptyState title="Aucun hôte ne correspond aux filtres." />
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
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="side-panel"
        @open="showConsole = true"
        @close="closeLiveConsole"
      />
    </div>

    <AptScheduleModal
      :host="scheduleHost"
      @close="scheduleHost = null"
    />
  </div>
</template>

<script setup lang="ts">
import { IconList } from '@tabler/icons-vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import AptToolbar from '../components/apt/AptToolbar.vue'
import AptHostCard from '../components/apt/AptHostCard.vue'
import AptScheduleModal from '../components/apt/AptScheduleModal.vue'
import { useApt } from '../composables/useApt'

const {
  hosts,
  selectedHosts,
  hostExpanded,
  aptStatuses,
  aptHistories,
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
  watchCommand,
  closeLiveConsole,
  runAptCmdForHost,
  bulkAptCmd,
  wsStatus,
  wsError,
  retryCount,
  dataStaleAlert,
  reconnect,
} = useApt()
</script>


<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          {{ t('nav.sections.control.items.dashboard') }}
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Docker</span>
      </div>
      <h2 class="page-title">
        Docker
      </h2>
      <div class="text-secondary">
        {{ t('docker.pageSubtitle') }}
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

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'containers' }"
          href="#"
          @click.prevent="activeTab = 'containers'"
        >
          {{ t('docker.containersTab') }}
          <span class="badge bg-azure-lt text-azure ms-1">{{ containers.length }}</span>
          <span
            v-if="runningCount > 0"
            class="badge bg-success-lt text-success ms-1"
          >{{ t('docker.runningCountBadge', { count: runningCount }, runningCount) }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'compose' }"
          href="#"
          @click.prevent="activeTab = 'compose'"
        >
          {{ t('docker.composeProjectsTab') }}
          <span class="badge bg-azure-lt text-azure ms-1">{{ composeProjects.length }}</span>
        </a>
      </li>
    </ul>

    <div class="side-layout">
      <div class="side-main">
        <DockerContainersTab
          v-if="activeTab === 'containers'"
          :containers="(containers as any)"
          :version-comparisons="(versionComparisons as any)"
          :can-run-docker="canRunDocker"
          :action-loading="(dockerActionLoading as any)"
          :bulk-action-loading="bulkActionLoading"
          @container-action="(handleContainerAction as any)"
          @bulk-container-action="(handleBulkContainerAction as any)"
        />
        <ComposeProjectsTab
          v-if="activeTab === 'compose'"
          :compose-projects="(composeProjects as any)"
          :containers="(containers as any)"
          :version-comparisons="(versionComparisons as any)"
          :can-run-docker="canRunDocker"
          :action-loading="(composeActionLoading as any)"
          @compose-action="(handleComposeAction as any)"
        />
      </div>

      <CommandLogPanel
        :command="dockerLiveCmd"
        :show="showDockerConsole"
        :title="t('docker.liveConsoleTitle')"
        :empty-text="t('docker.liveConsoleEmptyText')"
        wrapper-class="side-panel"
        @open="showDockerConsole = true"
        @close="closeDockerConsole"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useLocalStorage } from '../composables/useLocalStorage'
import WsStatusBar from '../components/WsStatusBar.vue'
import DockerContainersTab from '../components/docker/DockerContainersTab.vue'
import ComposeProjectsTab from '../components/docker/ComposeProjectsTab.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { useDocker } from '../composables/useDocker'

const { t } = useI18n()

const activeTab = useLocalStorage('dockerActiveTab', 'containers')

const {
  containers,
  composeProjects,
  versionComparisons,
  canRunDocker,
  runningCount,
  dockerActionLoading,
  composeActionLoading,
  bulkActionLoading,
  showDockerConsole,
  dockerLiveCmd,
  handleContainerAction,
  handleBulkContainerAction,
  handleComposeAction,
  closeDockerConsole,
  wsStatus,
  wsError,
  retryCount,
  dataStaleAlert,
  reconnect,
} = useDocker()
</script>

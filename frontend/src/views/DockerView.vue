<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Docker</span>
      </div>
      <h2 class="page-title">
        Docker
      </h2>
      <div class="text-secondary">
        Vue globale de tous les conteneurs sur l'infrastructure
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
          Conteneurs
          <span class="badge bg-azure-lt text-azure ms-1">{{ containers.length }}</span>
          <span
            v-if="runningCount > 0"
            class="badge bg-green-lt text-green ms-1"
          >{{ runningCount }} actifs</span>
        </a>
      </li>
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'compose' }"
          href="#"
          @click.prevent="activeTab = 'compose'"
        >
          Projets Compose
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
          @container-action="(handleContainerAction as any)"
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
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="side-panel"
        @open="showDockerConsole = true"
        @close="closeDockerConsole"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useLocalStorage } from '../composables/useLocalStorage'
import WsStatusBar from '../components/WsStatusBar.vue'
import DockerContainersTab from '../components/docker/DockerContainersTab.vue'
import ComposeProjectsTab from '../components/docker/ComposeProjectsTab.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { useDocker } from '../composables/useDocker'

const activeTab = useLocalStorage('dockerActiveTab', 'containers')

const {
  containers,
  composeProjects,
  versionComparisons,
  canRunDocker,
  runningCount,
  dockerActionLoading,
  composeActionLoading,
  showDockerConsole,
  dockerLiveCmd,
  handleContainerAction,
  handleComposeAction,
  closeDockerConsole,
  wsStatus,
  wsError,
  retryCount,
  dataStaleAlert,
  reconnect,
} = useDocker()
</script>

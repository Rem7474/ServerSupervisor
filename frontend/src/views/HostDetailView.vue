<template>
  <div class="host-detail-page">
    <HostDetailHeader
      :host="host"
      :is-admin="auth.isAdmin"
      :can-update-agent="canUpdateAgent"
      :agent-update-loading="agentUpdateLoading"
      :is-agent-up-to-date="isAgentUpToDate"
      @edit="isEditing = true"
      @update-agent="sendAgentUpdate"
      @delete="deleteHost"
    />

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      @reconnect="reconnect"
    />

    <HostDiagnosticsBanner
      :diagnostics="hostDiagnostics"
      :last-seen="host?.last_seen"
      :host-status="host?.status"
    />

    <LoadingSkeleton
      v-if="!host"
      :lines="6"
      variant="card"
      class="mb-3"
    />

    <HostProxmoxLinkPanel
      v-model:show-link-form="showLinkForm"
      v-model:selected-candidate="selectedCandidate"
      :proxmox-link="proxmoxLink"
      :link-saving="linkSaving"
      :show-link-button="showLinkButton"
      :link-candidates="linkCandidates"
      :link-candidates-loading="linkCandidatesLoading"
      :format-bytes-link="formatBytesLink"
      @confirm="confirmLink"
      @ignore="ignoreLink"
      @change-metrics-source="changeMetricsSource"
      @delete="deleteLink"
      @open-form="openLinkForm"
      @create-link="createManualLink"
    />

    <div class="side-layout">
      <div class="side-main">
        <HostEditForm
          v-if="isEditing"
          :host-id="hostId"
          :host="host"
          @close="isEditing = false"
          @updated="host = ($event as any)"
        />

        <EntityTabShell
          v-model="activeTab"
          :tabs="hostTabs"
          nav-class="host-detail-tabs"
        >
          <template #overview>
            <HostOverviewTab
              :apt-status="aptStatus"
              :docker-running-count="dockerRunningCount"
              :containers-count="containers.length"
              :tasks-count="tasksCount"
              :commands-count="cmdHistory.length"
              :incidents="hostActiveIncidents"
              :incidents-loading="incidentsLoading"
              :alerts-link="hostAlertsLink"
              @go-tab="activeTab = $event"
              @go-commands="goToTimelineCommands"
            />
          </template>

          <template #metrics>
            <HostMetricsTab
              :host-id="hostId"
              :host="host"
              :effective-metrics="effectiveMetrics"
              :effective-metrics-source="effectiveMetricsSource"
              :proxmox-link="proxmoxLink"
              :metrics-updated-at="metricsUpdatedAt"
              :disk-metrics="(diskMetrics as DiskMetric[] | null)"
              :disk-health="(diskHealth as DiskHealth[] | null)"
              :has-local-smart="hasLocalSmart"
              :is-proxmox-linked="isProxmoxLinked"
            />
          </template>

          <template #docker>
            <HostDockerTab
              :host-id="hostId"
              :containers="(containers as any)"
              :version-comparisons="(versionComparisons as any)"
              :can-run="canRunApt"
              @open-command="openCommand"
              @history-changed="loadCmdHistoryRefresh"
            />
          </template>

          <template #apt>
            <HostAptTab
              :apt-status="aptStatus"
              :can-run-apt="canRunApt"
              :apt-cmd-loading="aptCmdLoading"
              :uu-status="uuStatus"
              :uu-runs="(uuRuns as any)"
              :uu-form="(uuForm as any)"
              :uu-loading="uuLoading"
              @run-apt-command="sendAptCmd"
              @uu-install="handleUUInstall"
              @uu-configure="handleUUConfigure"
              @uu-run-now="handleUURunNow"
              @uu-log="(openUULog as any)"
            />
          </template>

          <template #backup>
            <HostBackupTab
              :host-id="hostId"
              :can-run="canRunApt"
              @watch-run="watchBackupRun"
            />
          </template>

          <template #reseau-flux>
            <NetworkFlowsTable
              :host-id="hostId"
              :initial-data="(networkFlows as any)"
              :collector-available="host?.collectors?.network_flows !== false"
              :active="activeTab === 'reseau-flux'"
              :refresh-tick="metricsUpdatedAt"
            />
          </template>

          <template #exposition>
            <HostExposureTab
              :host-id="hostId"
              @loaded="exposureDomainCount = $event"
            />
          </template>

          <template #systeme>
            <HostSystemTab
              v-if="canRunApt"
              :host-id="hostId"
              :can-run-apt="canRunApt"
              @open-command="openCommand"
              @history-changed="loadCmdHistoryRefresh"
            />
          </template>

          <template #processus>
            <HostProcessesPanel
              v-if="canRunApt"
              :host-id="hostId"
              :can-run="canRunApt"
              @history-changed="loadCmdHistoryRefresh"
            />
          </template>

          <template #taches-personnalisees>
            <HostCustomTasksTab
              :host-id="hostId"
              :can-run-apt="canRunApt"
              :active="activeTab === 'taches-personnalisees'"
              @open-command="openCommand"
              @tasks-count="customTasksCount = $event"
              @history-changed="loadCmdHistoryRefresh"
            />
          </template>

          <template #planifiees>
            <HostTasksTab
              :host-id="hostId"
              :can-run-apt="canRunApt"
              :active="activeTab === 'planifiees'"
              :collectors="hostCollectors"
              @open-command="openCommand"
              @tasks-count="tasksCount = $event"
              @history-changed="loadCmdHistoryRefresh"
            />
          </template>

          <template #timeline>
            <HostTimelineTab
              ref="timelineRef"
              :host-id="hostId"
              @watch-command="(openCommand as any)"
            />
          </template>

          <!-- Security tab: Per-host permissions (admin only) -->
          <template #securite>
            <HostSecurityTab
              v-if="auth.isAdmin"
              :host-perms="hostPerms"
              :perm-loading="permLoading"
              @revoke="revokePermission"
              @add="openAddPermission"
            />
          </template>
          <template #console>
            <div class="card">
              <div class="card-body d-flex align-items-center justify-content-between flex-wrap gap-2">
                <div>
                  <div class="fw-medium">
                    {{ proxmoxLink?.guest_name || `VMID ${proxmoxLink?.vmid}` }}
                    <span class="text-secondary small">({{ proxmoxLink?.node_name }})</span>
                  </div>
                  <div class="text-secondary small">
                    {{ t('host.consoleTabDescription') }}
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-outline-primary"
                  :disabled="showLxcConsole"
                  @click="openLxcConsole"
                >
                  {{ t('host.consoleTabOpen') }}
                </button>
              </div>
            </div>
          </template>
        </EntityTabShell>
      </div>

      <!-- One side panel for both consoles: once the host has an LXC console the
           panel is the ProxmoxConsole wrapper, which shows either the terminal
           or the live command log, so there is a single panel and reopen button. -->
      <ProxmoxConsoleLazy
        v-if="hasLxcConsole && hasOpenedLxcConsole"
        :guest-id="proxmoxLink?.guest_id"
        :guest-name="proxmoxLink?.guest_name || ''"
        :show="showLxcConsole || showConsole"
        :terminal="consoleKind === 'lxc'"
        :command="(liveCommand as any)"
        :log-title="t('host.consoleLive')"
        :empty-text="t('host.noActiveConsole')"
        :clearable="true"
        @open="reopenConsole"
        @close="closeActiveConsole"
        @clear="clearConsoleOutput"
      />
      <CommandLogPanel
        v-else
        :command="(liveCommand as any)"
        :show="showConsole"
        :title="t('host.consoleLive')"
        :empty-text="t('host.noActiveConsole')"
        wrapper-class="side-panel"
        :clearable="true"
        @open="showConsole = true"
        @close="closeConsoleAndStream"
        @clear="clearConsoleOutput"
      />
    </div>

    <!-- Add permission modal -->
    <HostPermissionModal
      v-model:open="addPermModal"
      v-model:username="newPermUsername"
      v-model:level="newPermLevel"
      :available-users="availableUsers"
      :perm-saving="permSaving"
      :perm-error="permError"
      @save="savePermission"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useHostDetail } from '../composables/useHostDetail'
import { useHostLxcConsole } from '../composables/useHostLxcConsole'
import { useHostTabs } from '../composables/useHostTabs'
import NetworkFlowsTable from '../components/network-flows/NetworkFlowsTable.vue'
import HostProcessesPanel from '../components/host/HostProcessesPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import HostAptTab from '../components/host/HostAptTab.vue'
import HostBackupTab from '../components/host/HostBackupTab.vue'
import HostDiagnosticsBanner from '../components/host/HostDiagnosticsBanner.vue'
import EntityTabShell from '../components/EntityTabShell.vue'
import type { DiagnosticIssue } from '../types/host'
import HostDockerTab from '../components/host/HostDockerTab.vue'
import type { DiskMetric } from '../composables/useDiskMetrics'
import type { DiskHealth } from '../composables/useDiskHealth'
import HostDetailHeader from '../components/host/HostDetailHeader.vue'
import HostProxmoxLinkPanel from '../components/host/HostProxmoxLinkPanel.vue'
import HostOverviewTab from '../components/host/HostOverviewTab.vue'
import HostMetricsTab from '../components/host/HostMetricsTab.vue'
import HostSecurityTab from '../components/host/HostSecurityTab.vue'
import HostPermissionModal from '../components/host/HostPermissionModal.vue'
import HostEditForm from '../components/host/HostEditForm.vue'
import HostExposureTab from '../components/host/HostExposureTab.vue'
import HostSystemTab from '../components/host/HostSystemTab.vue'
import HostTasksTab from '../components/host/HostTasksTab.vue'
import HostCustomTasksTab from '../components/host/HostCustomTasksTab.vue'
import HostTimelineTab from '../components/host/HostTimelineTab.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'

const { t } = useI18n()

const {
  auth,
  hostId,
  canRunApt,
  activeTab,
  isEditing,
  tasksCount,
  customTasksCount,
  aptCmdLoading,
  host,
  containers,
  metricsUpdatedAt,
  versionComparisons,
  aptStatus,
  cmdHistory,
  diskMetrics,
  diskHealth,
  networkFlows,
  proxmoxLink,
  linkSaving,
  hostActiveIncidents,
  incidentsLoading,
  effectiveMetrics,
  effectiveMetricsSource,
  showLinkForm,
  showLinkButton,
  linkCandidates,
  linkCandidatesLoading,
  selectedCandidate,
  liveCommand,
  showConsole,
  wsStatus,
  wsError,
  retryCount,
  reconnect,
  openCommand,
  sendAptCmd,
  sendAgentUpdate,
  isAgentUpToDate,
  canUpdateAgent,
  agentUpdateLoading,
  deleteHost,
  loadCmdHistoryRefresh,
  confirmLink,
  ignoreLink,
  changeMetricsSource,
  deleteLink,
  openLinkForm,
  createManualLink,
  closeConsoleAndStream,
  clearConsoleOutput,
  formatBytesLink,
  hostPerms,
  permLoading,
  addPermModal,
  newPermUsername,
  newPermLevel,
  permSaving,
  permError,
  availableUsers,
  openAddPermission,
  savePermission,
  revokePermission,
  uuStatus,
  uuRuns,
  uuForm,
  uuLoading,
  handleUUInstall,
  handleUUConfigure,
  handleUURunNow,
  openUULog,
} = useHostDetail()

// "Recent commands" KPI card jumps to the Timeline tab pre-filtered to
// command-type events — the Timeline tab absorbed the standalone Commands
// tab (same underlying remote_commands data, Timeline already merged it in
// as one of its type filters plus richer context).
const timelineRef = ref<{ filterCommands: () => void } | null>(null)
function goToTimelineCommands(): void {
  activeTab.value = 'timeline'
  nextTick(() => timelineRef.value?.filterCommands())
}

// Sauvegardes tab's run-history table had no way to see a run's detailed
// output, unlike every other module (apt/docker/systemd/runbooks/trackers/
// webhooks) that already reuses this same console for its own "Logs" action.
function watchBackupRun(run: { command_id?: string; profile?: string; status?: string }): void {
  if (!run.command_id) return
  openCommand({
    id: run.command_id,
    module: 'restic',
    action: 'run_backup',
    target: run.profile || '',
    status: 'completed',
    output: '',
  })
}

// Local SMART is unreadable inside an LXC/VM. When the host has no local disk
// health but is linked to Proxmox, we surface the hosting node's disk health
// instead of an empty SMART card.
const dockerRunningCount = computed(() =>
  containers.value.filter((c) => c.state === 'running').length
)

const hasLocalSmart = computed(() => ((diskHealth.value as unknown[] | null)?.length ?? 0) > 0)
const isProxmoxLinked = computed(() => !!proxmoxLink.value && proxmoxLink.value.status !== 'ignored')

// Fed by HostExposureTab's @loaded emit. The tab is one of the few that stay
// eager precisely because of this: the count is its own tab's badge, and a
// lazy tab would leave the badge empty until the user clicked it — which is
// the one thing the badge exists to save them from.
const exposureDomainCount = ref(0)

// Incidents' `host_name` is `hosts.name` (see db_notifications.go), so this
// pre-fills AlertIncidentList's search box to this host instead of landing on
// the undifferentiated full incidents list.
// Narrows the "Scheduled tasks" tab's module picker to what this host's
// agent actually reports collecting (see HostTasksTab's `collectors` prop).
const hostCollectors = computed(() => host.value?.collectors as Record<string, boolean> | undefined)

// The agent's own last self-check of its config against reality (e.g.
// collect_restic: true but resticconf missing) — see HostDiagnosticsBanner.
const hostDiagnostics = computed(() => host.value?.diagnostics as DiagnosticIssue[] | undefined)

const hostAlertsLink = computed(() => ({
  path: '/alerts',
  query: { tab: 'incidents', host: host.value?.name || host.value?.hostname || '' },
}))

const { hasLxcConsole, ProxmoxConsoleLazy, showLxcConsole, hasOpenedLxcConsole, consoleKind, openLxcConsole, reopenConsole, closeActiveConsole } = useHostLxcConsole({
  isAdmin: () => !!auth.isAdmin, proxmoxLink, activeTab, showConsole, closeConsoleAndStream,
})

const hostTabs = useHostTabs({
  isAdmin: () => !!auth.isAdmin,
  canRunApt,
  aptStatus,
  containers,
  hostActiveIncidents,
  exposureDomainCount,
  customTasksCount,
  tasksCount,
  hasLxcConsole,
})
</script>

<style scoped>
/* Same horizontal-scroll behaviour as the Proxmox node tabs: with 15+ tabs the
   bar must scroll on narrow screens instead of wrapping onto several rows. */
:deep(.host-detail-tabs) {
  flex-wrap: nowrap;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
}

:deep(.host-detail-tabs .nav-item) {
  flex: 0 0 auto;
}

:deep(.side-panel) {
  transition: width 0.3s ease-in-out;
  overflow: hidden;
}

@media (max-width: 991px) {
  :deep(.side-panel) {
    width: 100%;
  }
}
</style>

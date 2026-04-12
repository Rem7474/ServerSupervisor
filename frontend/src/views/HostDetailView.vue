<template>
  <div class="host-detail-page">
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              Dashboard
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>Hôte</span>
          </div>
          <h2 class="page-title">
            {{ host?.name || host?.hostname || 'Chargement...' }}
          </h2>
          <div class="text-secondary">
            {{ host?.hostname || 'Non connecté' }} - {{ host?.os || 'OS inconnu' }} - {{ host?.ip_address }}
            <span v-if="host?.last_seen">- Dernière activité: <RelativeTime :date="host.last_seen" /></span>
          </div>
        </div>
        <div class="d-flex align-items-center gap-2">
          <button
            class="btn btn-outline-secondary"
            @click="isEditing = true"
          >
            <svg
              class="icon me-1"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              stroke="currentColor"
              fill="none"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
            </svg>
            Modifier
          </button>
          <button
            class="btn btn-outline-danger"
            @click="deleteHost"
          >
            <svg
              class="icon me-1"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              stroke="currentColor"
              fill="none"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M3 6h18" /><path d="M8 6V4h8v2" /><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6" />
            </svg>
            Supprimer
          </button>
          <span
            v-if="host"
            :class="hostStatusClass(host.status)"
          >
            <span class="status-dot status-dot-animated" />
            <span :data-translation-id="host.status === 'online' ? 'online' : host.status === 'offline' ? 'offline' : 'unknown'">{{ formatHostStatus(host.status) }}</span>
          </span>
          <span
            v-if="host?.agent_version"
            :class="isAgentUpToDate(host.agent_version) ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'"
            :title="isAgentUpToDate(host.agent_version) ? 'Agent à jour' : 'Mise à jour disponible'"
          >
            Agent v{{ host.agent_version }}
          </span>
        </div>
      </div>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      @reconnect="reconnect"
    />

    <LoadingSkeleton
      v-if="!host"
      :lines="6"
      variant="card"
      class="mb-3"
    />

    <!-- Proxmox link panel -->
    <div
      v-if="proxmoxLink && proxmoxLink.status !== 'ignored'"
      class="card mb-3 border-0 shadow-sm"
    >
      <div class="card-body py-2 px-3 d-flex flex-wrap align-items-center gap-3">
        <!-- Guest info -->
        <div class="d-flex align-items-center gap-2">
          <span class="badge bg-purple-lt text-purple">Proxmox</span>
          <span class="fw-medium">{{ proxmoxLink.guest_name || `VMID ${proxmoxLink.vmid}` }}</span>
          <span class="text-muted small">({{ proxmoxLink.guest_type?.toUpperCase() }} · {{ proxmoxLink.node_name }})</span>
        </div>

        <!-- Status badge + suggestion actions -->
        <div class="d-flex align-items-center gap-2">
          <span
            v-if="proxmoxLink.status === 'suggested'"
            class="badge bg-warning-lt text-warning"
          >Suggestion</span>
          <span
            v-else
            class="badge bg-success-lt text-success"
          >Lié</span>
          <template v-if="proxmoxLink.status === 'suggested'">
            <button
              class="btn btn-sm btn-success"
              :disabled="linkSaving"
              @click="confirmLink"
            >
              Confirmer
            </button>
            <button
              class="btn btn-sm btn-outline-secondary"
              :disabled="linkSaving"
              @click="ignoreLink"
            >
              Ignorer
            </button>
          </template>
        </div>

        <!-- Metrics source selector (shown only when confirmed) -->
        <div
          v-if="proxmoxLink.status === 'confirmed'"
          class="d-flex align-items-center gap-2 ms-auto"
        >
          <label class="form-label mb-0 text-muted small">Source métriques :</label>
          <select
            class="form-select form-select-sm"
            style="width:auto"
            :value="proxmoxLink.metrics_source"
            @change="changeMetricsSource($event.target.value)"
          >
            <option value="auto">
              Automatique
            </option>
            <option value="agent">
              Agent
            </option>
            <option value="proxmox">
              Proxmox
            </option>
          </select>
          <button
            class="btn btn-sm btn-outline-danger"
            :disabled="linkSaving"
            title="Supprimer le lien"
            @click="deleteLink"
          >
            <svg
              class="icon icon-sm"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6" />
              <path d="M10 11v6M14 11v6M9 6V4h6v2" />
            </svg>
          </button>
        </div>

        <!-- Guest live metrics (source = proxmox) -->
        <template v-if="proxmoxLink.status === 'confirmed' && proxmoxLink.metrics_source !== 'agent'">
          <div class="d-flex align-items-center gap-3 ms-2 border-start ps-3">
            <div class="text-muted small">
              CPU <strong class="text-body">{{ ((proxmoxLink.cpu_usage ?? 0) * 100).toFixed(1) }}%</strong>
            </div>
            <div class="text-muted small">
              RAM <strong class="text-body">{{ formatBytesLink(proxmoxLink.mem_usage) }}</strong> / {{ formatBytesLink(proxmoxLink.mem_alloc) }}
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
        class="btn btn-sm btn-outline-purple"
        @click="openLinkForm"
      >
        <svg
          class="icon icon-sm me-1"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
          <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
        </svg>
        Lier à Proxmox
      </button>
    </div>

    <!-- Manual link form -->
    <div
      v-if="showLinkForm"
      class="card mb-3"
    >
      <div class="card-body">
        <div class="fw-medium mb-2">
          Lier cet hôte à un guest Proxmox
        </div>
        <div
          v-if="linkCandidatesLoading"
          class="text-muted small"
        >
          Chargement...
        </div>
        <div
          v-else-if="linkCandidates.length === 0"
          class="text-muted small"
        >
          Aucun guest Proxmox disponible (non encore lié).
        </div>
        <div
          v-else
          class="d-flex align-items-center gap-2"
        >
          <select
            v-model="selectedCandidate"
            class="form-select form-select-sm"
            style="max-width:320px"
          >
            <option value="">
              -- Choisir un guest --
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
            class="btn btn-sm btn-primary"
            :disabled="!selectedCandidate || linkSaving"
            @click="createManualLink"
          >
            Lier
          </button>
          <button
            class="btn btn-sm btn-outline-secondary"
            @click="showLinkForm = false; selectedCandidate = ''"
          >
            Annuler
          </button>
        </div>
      </div>
    </div>

    <div class="side-layout">
      <div class="side-main">
        <HostEditForm
          v-if="isEditing"
          :host-id="hostId"
          :host="host"
          @close="isEditing = false"
          @updated="host = $event"
        />

        <HostDetailTabs
          v-model="activeTab"
          :can-run-apt="canRunApt"
          :containers-count="containers.length"
          :pending-packages="aptStatus?.pending_packages || 0"
          :commands-count="cmdHistory.length"
          :tasks-count="tasksCount"
        />

        <div v-show="activeTab === 'metrics'">
          <HostMetricsPanel
            :host-id="hostId"
            :metrics="effectiveMetrics"
            :metrics-source="effectiveMetricsSource"
            :proxmox-guest-id="proxmoxLink?.guest_id ?? null"
          />
          <DiskMetricsCard
            :host-id="hostId"
            :initial-metrics="diskMetrics"
            class="mb-4"
          />
          <DiskHealthCard
            :host-id="hostId"
            :initial-health="diskHealth"
            class="mb-4"
          />
        </div>

        <div v-show="activeTab === 'docker'">
          <HostDockerTab
            :containers="containers"
            :version-comparisons="versionComparisons"
          />
        </div>

        <div v-show="activeTab === 'apt'">
          <HostAptTab
            :apt-status="aptStatus"
            :can-run-apt="canRunApt"
            :apt-cmd-loading="aptCmdLoading"
            @run-apt-command="sendAptCmd"
          />
        </div>

        <div v-show="activeTab === 'commandes'">
          <HostCommandsTab
            :commands="cmdHistory"
            @watch-command="openCommand"
          />
        </div>

        <div v-show="activeTab === 'systeme'">
          <HostSystemTab
            v-if="canRunApt"
            :host-id="hostId"
            :can-run-apt="canRunApt"
            @open-command="openCommand"
            @history-changed="loadCmdHistoryRefresh"
          />
        </div>

        <div v-show="activeTab === 'processus'">
          <HostProcessesPanel
            v-if="canRunApt"
            :host-id="hostId"
            :can-run="canRunApt"
            @history-changed="loadCmdHistoryRefresh"
          />
        </div>

        <div v-show="activeTab === 'planifiees'">
          <HostTasksTab
            :host-id="hostId"
            :can-run-apt="canRunApt"
            :active="activeTab === 'planifiees'"
            @open-command="openCommand"
            @tasks-count="tasksCount = $event"
            @history-changed="loadCmdHistoryRefresh"
          />
        </div>
      </div>

      <CommandLogPanel
        :command="liveCommand"
        :show="showConsole"
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="side-panel"
        :clearable="true"
        @open="showConsole = true"
        @close="closeConsoleAndStream"
        @clear="clearConsoleOutput"
      />
    </div>

    <!-- Per-host permissions (admin only) -->
    <div
      v-if="auth.isAdmin"
      class="card mt-4"
    >
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title mb-0">
          Permissions par hôte
        </h3>
        <button
          class="btn btn-sm btn-outline-primary"
          @click="openAddPermission"
        >
          + Ajouter
        </button>
      </div>
      <div class="card-body p-0">
        <div
          v-if="permLoading"
          class="text-center py-3"
        >
          <span class="spinner-border spinner-border-sm" />
        </div>
        <div
          v-else-if="!hostPerms.length"
          class="text-center py-3 text-muted small"
        >
          Aucune restriction — tous les utilisateurs accèdent à cet hôte selon leur rôle global.
        </div>
        <table
          v-else
          class="table table-vcenter mb-0"
        >
          <thead>
            <tr>
              <th>Utilisateur</th>
              <th>Niveau</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in hostPerms"
              :key="p.username"
            >
              <td>{{ p.username }}</td>
              <td>
                <span :class="p.level === 'operator' ? 'badge bg-blue-lt' : 'badge bg-secondary-lt'">
                  {{ p.level }}
                </span>
              </td>
              <td class="text-end">
                <button
                  class="btn btn-sm btn-ghost-danger"
                  title="Révoquer"
                  @click="revokePermission(p.username)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="icon icon-sm"
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="M18 6l-12 12" /><path d="M6 6l12 12" />
                  </svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Add permission modal -->
    <div
      v-if="addPermModal"
      class="modal modal-blur show d-block"
      tabindex="-1"
      style="background:rgba(0,0,0,.5);z-index:1050"
      @click.self="addPermModal = false"
    >
      <div
        class="modal-dialog modal-dialog-centered"
        style="max-width:380px"
      >
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              Ajouter une permission
            </h5>
            <button
              type="button"
              class="btn-close"
              @click="addPermModal = false"
            />
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">Utilisateur</label>
              <select
                v-model="newPermUsername"
                class="form-select"
              >
                <option value="">
                  -- Choisir --
                </option>
                <option
                  v-for="u in availableUsers"
                  :key="u.username"
                  :value="u.username"
                >
                  {{ u.username }}
                </option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-label">Niveau</label>
              <select
                v-model="newPermLevel"
                class="form-select"
              >
                <option value="viewer">
                  viewer — lecture seule
                </option>
                <option value="operator">
                  operator — lecture + commandes
                </option>
              </select>
            </div>
            <div
              v-if="permError"
              class="alert alert-danger py-2"
            >
              {{ permError }}
            </div>
          </div>
          <div class="modal-footer">
            <button
              class="btn btn-secondary"
              @click="addPermModal = false"
            >
              Annuler
            </button>
            <button
              class="btn btn-primary"
              :disabled="!newPermUsername || permSaving"
              @click="savePermission"
            >
              <span
                v-if="permSaving"
                class="spinner-border spinner-border-sm me-1"
              />
              Enregistrer
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useHostDetail } from '../composables/useHostDetail'
import RelativeTime from '../components/RelativeTime.vue'
import DiskMetricsCard from '../components/DiskMetricsCard.vue'
import DiskHealthCard from '../components/DiskHealthCard.vue'
import HostMetricsPanel from '../components/HostMetricsPanel.vue'
import HostProcessesPanel from '../components/HostProcessesPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import HostAptTab from '../components/host/HostAptTab.vue'
import HostCommandsTab from '../components/host/HostCommandsTab.vue'
import HostDetailTabs from '../components/host/HostDetailTabs.vue'
import HostDockerTab from '../components/host/HostDockerTab.vue'
import HostEditForm from '../components/host/HostEditForm.vue'
import HostSystemTab from '../components/host/HostSystemTab.vue'
import HostTasksTab from '../components/host/HostTasksTab.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'

const {
  auth,
  hostId,
  canRunApt,
  activeTab,
  isEditing,
  tasksCount,
  aptCmdLoading,
  host,
  containers,
  versionComparisons,
  aptStatus,
  cmdHistory,
  diskMetrics,
  diskHealth,
  proxmoxLink,
  linkSaving,
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
  isAgentUpToDate,
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
} = useHostDetail()
</script>

<style scoped>
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

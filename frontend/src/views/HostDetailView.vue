<template>
  <div class="host-detail-page">
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-start justify-content-between gap-3">
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
          <div class="d-flex flex-wrap align-items-center gap-2 mt-2">
            <span
              v-if="host"
              :class="hostStatusClass(host.status)"
              :aria-label="`Statut de l'hôte : ${formatHostStatus(host.status)}`"
            >
              <span :class="['status-dot', host.status === 'online' ? 'status-dot-animated' : '']" />
              {{ formatHostStatus(host.status) }}
            </span>
            <BadgePill
              v-if="host?.agent_version"
              :tone="isAgentUpToDate(host.agent_version) ? 'success' : 'warning'"
              :text="`Agent v${host.agent_version}`"
              :aria-label="isAgentUpToDate(host.agent_version) ? `Agent version ${host.agent_version}, à jour` : `Agent version ${host.agent_version}, mise à jour disponible`"
              :title="isAgentUpToDate(host.agent_version) ? 'Agent à jour' : 'Mise à jour de l\'agent disponible'"
              compact
            />
          </div>
        </div>
        <div class="d-flex flex-wrap align-items-center justify-content-lg-end gap-2">
          <button
            type="button"
            class="btn btn-outline-secondary"
            @click="isEditing = true"
          >
            <IconPencil
              :size="16"
              class="icon me-1"
            />
            Modifier
          </button>
          <button
            v-if="canUpdateAgent"
            type="button"
            class="btn btn-outline-primary"
            :disabled="agentUpdateLoading"
            @click="sendAgentUpdate"
          >
            <IconRefresh
              :size="16"
              class="icon me-1"
            />
            Mettre à jour l'agent
          </button>
          <button
            v-if="auth.isAdmin"
            type="button"
            class="btn btn-outline-danger"
            @click="deleteHost"
          >
            <IconTrash
              :size="16"
              class="icon me-1"
            />
            Supprimer
          </button>
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
            title="Voir le guest Proxmox (démarrer/arrêter, métriques détaillées)"
          >
            Voir le guest
            <IconExternalLink :size="14" />
          </router-link>
        </div>

        <!-- Status badge + suggestion actions -->
        <div class="d-flex align-items-center gap-2">
          <BadgePill
            v-if="proxmoxLink.status === 'suggested'"
            text="Suggestion"
            tone="warning"
            compact
          />
          <BadgePill
            v-else
            text="Lié"
            tone="success"
            compact
          />
          <template v-if="proxmoxLink.status === 'suggested'">
            <button
              type="button"
              class="btn btn-sm btn-success"
              :disabled="linkSaving"
              @click="confirmLink"
            >
              Confirmer
            </button>
            <button
              type="button"
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
            @change="changeMetricsSource(($event.target as HTMLSelectElement).value as 'agent' | 'proxmox' | 'auto')"
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
            type="button"
            class="btn btn-icon btn-sm btn-outline-danger"
            :disabled="linkSaving"
            title="Supprimer le lien"
            @click="deleteLink"
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
        type="button"
        class="btn btn-sm btn-outline-warning"
        @click="openLinkForm"
      >
        <IconLink
          :size="16"
          class="icon icon-sm me-1"
        />
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
            class="form-select form-select-sm candidate-select"
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
            type="button"
            class="btn btn-sm btn-primary"
            :disabled="!selectedCandidate || linkSaving"
            @click="createManualLink"
          >
            Lier
          </button>
          <button
            type="button"
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
          @updated="host = ($event as any)"
        />

        <EntityTabShell
          v-model="activeTab"
          :tabs="hostTabs"
        >
          <template #overview>
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
                      {{ aptStatus?.security_updates || 0 }} sécurité
                      <a
                        href="#"
                        class="ms-1"
                        @click.prevent="activeTab = 'apt'"
                      >voir</a>
                    </div>
                  </div>
                </div>
              </div>
              <div class="col-6 col-lg-3">
                <div class="card card-sm h-100">
                  <div class="card-body">
                    <div class="subheader">
                      Conteneurs Docker
                    </div>
                    <div class="h2 mb-0 mt-1">
                      {{ dockerRunningCount }} / {{ containers.length }}
                    </div>
                    <div class="text-secondary small">
                      en cours
                      <a
                        href="#"
                        class="ms-1"
                        @click.prevent="activeTab = 'docker'"
                      >voir</a>
                    </div>
                  </div>
                </div>
              </div>
              <div class="col-6 col-lg-3">
                <div class="card card-sm h-100">
                  <div class="card-body">
                    <div class="subheader">
                      Tâches planifiées
                    </div>
                    <div class="h2 mb-0 mt-1">
                      {{ tasksCount }}
                    </div>
                    <div class="text-secondary small">
                      <a
                        href="#"
                        @click.prevent="activeTab = 'planifiees'"
                      >voir</a>
                    </div>
                  </div>
                </div>
              </div>
              <div class="col-6 col-lg-3">
                <div class="card card-sm h-100">
                  <div class="card-body">
                    <div class="subheader">
                      Commandes récentes
                    </div>
                    <div class="h2 mb-0 mt-1">
                      {{ cmdHistory.length }}
                    </div>
                    <div class="text-secondary small">
                      <a
                        href="#"
                        @click.prevent="activeTab = 'commandes'"
                      >voir</a>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="card mb-3">
              <div class="card-header d-flex align-items-center justify-content-between">
                <h3 class="card-title mb-0">
                  <IconAlertTriangle
                    :size="18"
                    class="icon me-1"
                  />
                  Alertes actives sur cet hôte
                </h3>
                <router-link
                  to="/alerts?tab=incidents"
                  class="btn btn-sm btn-outline-secondary"
                >
                  Toutes les alertes
                </router-link>
              </div>
              <div
                v-if="incidentsLoading"
                class="card-body text-center text-secondary py-4"
              >
                <div class="spinner-border spinner-border-sm me-2" />
                Chargement…
              </div>
              <div
                v-else-if="!hostActiveIncidents.length"
                class="card-body text-center text-secondary py-4"
              >
                Aucune alerte active sur cet hôte.
              </div>
              <div
                v-else
                class="list-group list-group-flush"
              >
                <router-link
                  v-for="item in hostActiveIncidents"
                  :key="item.id"
                  :to="hostAlertsLink"
                  class="list-group-item list-group-item-action d-flex align-items-center gap-2"
                >
                  <span
                    class="badge"
                    :class="item.severity === 'crit' ? 'bg-red-lt text-red' : 'bg-yellow-lt text-yellow'"
                  >{{ item.severity }}</span>
                  <span class="flex-grow-1">{{ item.rule_name || item.metric }}</span>
                  <RelativeTime :date="item.triggered_at || ''" />
                </router-link>
              </div>
            </div>
          </template>

          <template #metrics>
            <div
              v-if="host && host.status !== 'online'"
              class="alert alert-warning mb-3"
            >
              <IconAlertCircle
                :size="16"
                class="icon me-2"
              />
              Agent hors ligne — les données affichées peuvent être obsolètes ou indisponibles.
            </div>
            <HostMetricsPanel
              :host-id="hostId"
              :metrics="effectiveMetrics"
              :metrics-source="effectiveMetricsSource"
              :proxmox-guest-id="proxmoxLink?.guest_id ?? null"
              :refresh-tick="metricsUpdatedAt"
            />
            <DiskMetricsCard
              :host-id="hostId"
              :initial-metrics="(diskMetrics as any)"
              class="mb-4"
            />
            <DiskHistoryChart
              :host-id="hostId"
              :mounts="(diskMetrics?.map((d: any) => d.mount_point) ?? [])"
              :refresh-tick="metricsUpdatedAt"
              class="mb-4"
            />
            <DiskHealthCard
              v-if="hasLocalSmart || !isProxmoxLinked"
              :host-id="hostId"
              :initial-health="(diskHealth as any)"
              class="mb-4"
            />
            <ProxmoxHostDiskHealthCard
              v-else
              :host-id="hostId"
              :node-name="proxmoxLink?.node_name ?? null"
              class="mb-4"
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

          <template #commandes>
            <HostCommandsTab
              :commands="(cmdHistory as any)"
              @watch-command="(openCommand as any)"
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

          <template #planifiees>
            <HostTasksTab
              :host-id="hostId"
              :can-run-apt="canRunApt"
              :active="activeTab === 'planifiees'"
              @open-command="openCommand"
              @tasks-count="tasksCount = $event"
              @history-changed="loadCmdHistoryRefresh"
            />
          </template>

          <template #timeline>
            <HostTimelineTab :host-id="hostId" />
          </template>

          <!-- Security tab: Per-host permissions (admin only) -->
          <template #securite>
            <div
              v-if="auth.isAdmin"
              class="card"
            >
              <div class="card-header d-flex align-items-center justify-content-between">
                <h3 class="card-title mb-0 d-flex align-items-center gap-2">
                  <IconLock
                    :size="16"
                    class="icon icon-sm text-warning"
                  />
                  Permissions par hôte
                </h3>
                <span class="badge badge-sm bg-red text-white">Admin only</span>
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
                          type="button"
                          class="btn btn-icon btn-sm btn-ghost-danger"
                          title="Révoquer"
                          @click="revokePermission(p.username)"
                        >
                          <IconX
                            :size="16"
                            class="icon icon-sm"
                          />
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="card-footer d-flex justify-content-end">
                <button
                  type="button"
                  class="btn btn-sm btn-outline-primary"
                  @click="openAddPermission"
                >
                  + Ajouter
                </button>
              </div>
            </div>
          </template>
        </EntityTabShell>
      </div>

      <CommandLogPanel
        :command="(liveCommand as any)"
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

    <!-- Add permission modal -->
    <div
      v-if="addPermModal"
      ref="permModalRef"
      class="modal modal-blur fade show d-block modal-permissions-overlay"
      tabindex="-1"
      @click.self="addPermModal = false"
    >
      <div
        class="modal-dialog modal-dialog-centered modal-permissions-dialog"
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
              type="button"
              class="btn btn-secondary"
              @click="addPermModal = false"
            >
              Annuler
            </button>
            <button
              type="button"
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

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IconLink, IconLock, IconPencil, IconRefresh, IconTrash, IconX, IconAlertCircle, IconAlertTriangle, IconExternalLink } from '@tabler/icons-vue'
import { useHostDetail } from '../composables/useHostDetail'
import { useModalChrome } from '../composables/useModalChrome'
import RelativeTime from '../components/RelativeTime.vue'
import DiskMetricsCard from '../components/disk/DiskMetricsCard.vue'
import DiskHealthCard from '../components/disk/DiskHealthCard.vue'
import ProxmoxHostDiskHealthCard from '../components/proxmox/ProxmoxHostDiskHealthCard.vue'
import DiskHistoryChart from '../components/disk/DiskHistoryChart.vue'
import HostMetricsPanel from '../components/host/HostMetricsPanel.vue'
import HostProcessesPanel from '../components/host/HostProcessesPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import HostAptTab from '../components/host/HostAptTab.vue'
import HostCommandsTab from '../components/host/HostCommandsTab.vue'
import EntityTabShell from '../components/EntityTabShell.vue'
import type { EntityTab } from '../components/EntityTabShell.vue'
import HostDockerTab from '../components/host/HostDockerTab.vue'
import HostEditForm from '../components/host/HostEditForm.vue'
import HostExposureTab from '../components/host/HostExposureTab.vue'
import HostSystemTab from '../components/host/HostSystemTab.vue'
import HostTasksTab from '../components/host/HostTasksTab.vue'
import HostTimelineTab from '../components/host/HostTimelineTab.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import BadgePill from '../components/common/BadgePill.vue'
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
  metricsUpdatedAt,
  versionComparisons,
  aptStatus,
  cmdHistory,
  diskMetrics,
  diskHealth,
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

const permModalRef = ref<HTMLElement | null>(null)
useModalChrome(permModalRef, () => addPermModal.value, { onClose: () => { addPermModal.value = false } })

// Local SMART is unreadable inside an LXC/VM. When the host has no local disk
// health but is linked to Proxmox, we surface the hosting node's disk health
// instead of an empty SMART card.
const dockerRunningCount = computed(() =>
  containers.value.filter((c) => c.state === 'running').length
)

const hasLocalSmart = computed(() => ((diskHealth.value as unknown[] | null)?.length ?? 0) > 0)
const isProxmoxLinked = computed(() => !!proxmoxLink.value && proxmoxLink.value.status !== 'ignored')

// Fed by HostExposureTab's @loaded emit — the tab mounts eagerly (Host's
// tabs use v-show, not lazy), so this is already populated before the user
// ever clicks "Exposition".
const exposureDomainCount = ref(0)

// Incidents' `host_name` is `hosts.name` (see db_notifications.go), so this
// pre-fills AlertIncidentList's search box to this host instead of landing on
// the undifferentiated full incidents list.
const hostAlertsLink = computed(() => ({
  path: '/alerts',
  query: { tab: 'incidents', host: host.value?.name || host.value?.hostname || '' },
}))

const hostTabs = computed<EntityTab[]>(() => {
  const securityUpdates = aptStatus.value?.security_updates || 0
  const pendingPackages = aptStatus.value?.pending_packages || 0

  const tabs: EntityTab[] = [
    {
      key: 'overview',
      label: "Vue d'ensemble",
      badges: hostActiveIncidents.value.length
        ? [{ value: hostActiveIncidents.value.length, badgeClass: 'badge bg-red-lt text-red ms-1' }]
        : [],
    },
    { key: 'metrics', label: 'Métriques' },
    {
      key: 'docker',
      label: 'Docker',
      badges: containers.value.length ? [{ value: containers.value.length, badgeClass: 'badge bg-blue-lt text-blue ms-1' }] : [],
    },
    {
      key: 'apt',
      label: 'APT',
      badges: securityUpdates > 0
        ? [{ value: securityUpdates, badgeClass: 'badge bg-red-lt text-red ms-1' }]
        : pendingPackages > 0
          ? [{ value: pendingPackages, badgeClass: 'badge bg-yellow-lt text-yellow ms-1' }]
          : [],
    },
    {
      key: 'commandes',
      label: 'Commandes',
      badges: cmdHistory.value.length ? [{ value: cmdHistory.value.length, badgeClass: 'badge bg-secondary-lt text-secondary ms-1' }] : [],
    },
    {
      key: 'exposition',
      label: 'Exposition',
      badges: exposureDomainCount.value > 0
        ? [{ value: exposureDomainCount.value, badgeClass: 'badge bg-azure-lt text-azure ms-1' }]
        : [],
    },
  ]

  if (canRunApt.value) {
    tabs.push(
      { key: 'systeme', label: 'Systeme' },
      { key: 'processus', label: 'Processus' }
    )
  }

  // Labeled "Permissions" (not "Sécurité") to avoid colliding with
  // ProxmoxNodeView's "Journaux sécurité" tab — same word, unrelated content
  // (per-host RBAC here vs. PVE syslog auth-failure search there).
  tabs.push({ key: 'securite', label: 'Permissions' })
  tabs.push({
    key: 'planifiees',
    label: 'Tâches planifiées',
    badges: tasksCount.value ? [{ value: tasksCount.value, badgeClass: 'badge bg-secondary-lt text-secondary ms-1' }] : [],
  })
  tabs.push({ key: 'timeline', label: 'Timeline' })

  return tabs
})
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

.candidate-select {
  max-width: 320px;
}
</style>

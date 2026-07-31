<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>Audit</span>
        </div>
        <h2 class="page-title">
          Audit
        </h2>
        <div class="text-secondary">
          Historique des actions, connexions et commandes
        </div>
      </div>
    </div>

    <!-- Tab navigation -->
    <ul class="nav nav-tabs mb-4">
      <li
        v-if="canViewCommands"
        class="nav-item"
      >
        <a
          class="nav-link"
          :class="{ active: activeTab === 'commandes' }"
          href="#"
          @click.prevent="switchToCommandes"
        >
          Commandes
        </a>
      </li>
      <li
        v-if="auth.role === 'admin'"
        class="nav-item"
      >
        <a
          class="nav-link"
          :class="{ active: activeTab === 'connexions' }"
          href="#"
          @click.prevent="switchToConnexions"
        >
          Connexions
        </a>
      </li>
    </ul>

    <!-- ── Commandes tab ────────────────────────────────────────────────────── -->
    <div
      v-show="activeTab === 'commandes'"
      class="side-layout"
    >
      <div class="side-main">
        <DataToolbar
          searchable
          :search="cmdSearch"
          search-placeholder="Rechercher hôte, commande, utilisateur…"
          @update:search="onSearchUpdate"
        >
          <template #bottom>
            <div class="row g-2">
              <div class="col-6 col-md-4">
                <select
                  v-model="cmdStatusFilter"
                  class="form-select form-select-sm"
                  @change="onFilterChange"
                >
                  <option value="">
                    Tous les états
                  </option>
                  <option value="pending">
                    En attente
                  </option>
                  <option value="running">
                    En cours
                  </option>
                  <option value="completed">
                    Terminé
                  </option>
                  <option value="failed">
                    Échoué
                  </option>
                </select>
              </div>
              <div class="col-6 col-md-4">
                <select
                  v-model="cmdModuleFilter"
                  class="form-select form-select-sm"
                  @change="onFilterChange"
                >
                  <option value="">
                    Tous les modules
                  </option>
                  <option value="apt">
                    APT
                  </option>
                  <option value="docker">
                    Docker
                  </option>
                  <option value="systemd">
                    Systemd
                  </option>
                  <option value="journal">
                    Journal
                  </option>
                  <option value="processes">
                    Processus
                  </option>
                  <option value="custom">
                    Custom
                  </option>
                </select>
              </div>
            </div>
          </template>
        </DataToolbar>

        <div class="card">
          <div class="table-responsive scroll-table">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>
                    <SortableHeader
                      label="Date"
                      :active="cmdSortBy === 'created_at'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('created_at')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Hôte"
                      :active="cmdSortBy === 'host_name'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('host_name')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Type"
                      :active="cmdSortBy === 'module'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('module')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Commande"
                      :active="cmdSortBy === 'command'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('command')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Utilisateur"
                      :active="cmdSortBy === 'triggered_by'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('triggered_by')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Statut"
                      :active="cmdSortBy === 'status'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('status')"
                    />
                  </th>
                  <th>Durée</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-if="cmdsLoading">
                  <td
                    colspan="8"
                    class="py-2"
                  >
                    <LoadingSkeleton
                      variant="table"
                      :lines="5"
                    />
                  </td>
                </tr>
                <tr v-else-if="!sortedCmds.length">
                  <td colspan="8">
                    <EmptyState title="Aucune commande enregistrée" />
                  </td>
                </tr>
                <tr
                  v-for="cmd in sortedCmds"
                  :key="cmd.id"
                  :class="{ 'table-active': selectedCmd?.id === cmd.id }"
                >
                  <td class="text-secondary small">
                    {{ formatDate(cmd.created_at) }}
                  </td>
                  <td>
                    <router-link
                      :to="`/hosts/${cmd.host_id}`"
                      class="text-decoration-none fw-semibold"
                    >
                      {{ cmd.host_name || cmd.host_id }}
                    </router-link>
                  </td>
                  <td>
                    <span :class="moduleClass(cmd.module)">{{ moduleLabel(cmd.module) }}</span>
                  </td>
                  <td>
                    <code class="small">{{ cmdLabel(cmd) }}</code>
                  </td>
                  <td class="text-secondary small">
                    {{ cmd.triggered_by || '—' }}
                  </td>
                  <td>
                    <span :class="statusClass(cmd.status)">{{ statusLabel(cmd.status) }}</span>
                  </td>
                  <td class="text-secondary small">
                    {{ formatDuration(cmd.started_at, cmd.ended_at) }}
                  </td>
                  <td class="text-end">
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      :disabled="!cmd.output && cmd.status === 'pending'"
                      title="Voir les logs"
                      @click="openLogViewer(cmd)"
                    >
                      <IconList
                        :size="16"
                        class="icon icon-sm"
                      />
                    </button>
                    <button
                      v-if="cmd.status === 'pending' || cmd.status === 'running'"
                      type="button"
                      class="btn btn-sm btn-outline-danger ms-1"
                      :disabled="cancellingId === cmd.id"
                      @click="cancelCmd(cmd.id)"
                    >
                      <span
                        v-if="cancellingId === cmd.id"
                        class="spinner-border spinner-border-sm me-1"
                      />
                      Annuler
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="card-footer d-flex align-items-center justify-content-between">
            <div class="text-secondary small">
              {{ cmdsTotal }} commande{{ cmdsTotal !== 1 ? 's' : '' }} — page {{ cmdsPage }} / {{ totalCmdsPages }}
            </div>
            <PaginationNav
              :current-page="cmdsPage"
              :total-pages="totalCmdsPages"
              @select="selectCmdsPage"
            />
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="selectedCmd"
        :show="showLogViewer"
        wrapper-class="side-panel"
        title="Logs"
        empty-text="Aucun log sélectionné"
        @close="closeLogViewer"
        @open="showLogViewer = true"
      />
    </div>

    <!-- ── Connexions tab (admin only) ────────────────────────────────────── -->
    <div v-show="activeTab === 'connexions'">
      <AuditSecurityPanel
        :security="security"
        :period="securityPeriod"
        :period-label="securityPeriodLabel"
        :period-options="secPeriodOptions"
        :unblocking-ip="unblockingIP"
        @set-period="setSecurityPeriod"
        @unblock="unblockIP"
      />

      <!-- All login events -->
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">
            Toutes les connexions
          </h3>
        </div>
        <ConnectionsTable
          :events="connexions"
          :loading="connexionsLoading"
          show-username
        />
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">
            Page {{ connexionsPage }} / {{ totalConnexionsPages }}
          </div>
          <PaginationNav
            :current-page="connexionsPage"
            :total-pages="totalConnexionsPages"
            @select="selectConnexionsPage"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconList } from '@tabler/icons-vue'
import { useDateFormatter } from '../composables/useDateFormatter'
import { useAuditLogs } from '../composables/useAuditLogs'
import PaginationNav from '../components/PaginationNav.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import EmptyState from '../components/EmptyState.vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import AuditSecurityPanel from '../components/security/AuditSecurityPanel.vue'
import ConnectionsTable from '../components/common/ConnectionsTable.vue'

const { formatLocaleDateTime: formatDate } = useDateFormatter()

const {
  auth,
  canViewCommands,
  activeTab,
  switchToCommandes,
  switchToConnexions,
  cmdsPage,
  cmdsTotal,
  cmdsLoading,
  totalCmdsPages,
  cmdSearch,
  cmdStatusFilter,
  cmdModuleFilter,
  cmdSortBy,
  cmdSortDir,
  sortedCmds,
  toggleCmdSort,
  onSearchUpdate,
  onFilterChange,
  selectedCmd,
  showLogViewer,
  openLogViewer,
  closeLogViewer,
  cancellingId,
  cancelCmd,
  selectCmdsPage,
  connexions,
  connexionsPage,
  connexionsLoading,
  totalConnexionsPages,
  selectConnexionsPage,
  security,
  secPeriodOptions,
  securityPeriod,
  securityPeriodLabel,
  setSecurityPeriod,
  unblockingIP,
  unblockIP,
  moduleLabel,
  moduleClass,
  statusLabel,
  cmdLabel,
  formatDuration,
  statusClass,
} = useAuditLogs()
</script>

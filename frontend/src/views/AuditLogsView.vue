<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            {{ t('account.dashboardBreadcrumb') }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ t('account.auditPageTitle') }}</span>
        </div>
        <h2 class="page-title">
          {{ t('account.auditPageTitle') }}
        </h2>
        <div class="text-secondary">
          {{ t('account.auditPageSubtitle') }}
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
          {{ t('account.commandsTab') }}
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
          {{ t('account.connectionsTab') }}
        </a>
      </li>
      <li
        v-if="auth.role === 'admin'"
        class="nav-item"
      >
        <a
          class="nav-link"
          :class="{ active: activeTab === 'journal' }"
          href="#"
          @click.prevent="switchToJournal"
        >
          {{ t('account.journalTab') }}
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
          :search-placeholder="t('account.cmdSearchPlaceholder')"
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
                    {{ t('account.allStatesOption') }}
                  </option>
                  <option value="pending">
                    {{ t('common.statePending') }}
                  </option>
                  <option value="running">
                    {{ t('common.stateRunning') }}
                  </option>
                  <option value="completed">
                    {{ t('common.stateCompleted') }}
                  </option>
                  <option value="failed">
                    {{ t('common.stateFailed') }}
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
                    {{ t('account.allModulesOption') }}
                  </option>
                  <option
                    v-for="opt in moduleFilterOptions"
                    :key="opt.value"
                    :value="opt.value"
                  >
                    {{ opt.label }}
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
                      :label="t('account.dateColumn')"
                      :active="cmdSortBy === 'created_at'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('created_at')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      :label="t('account.hostColumnLabel')"
                      :active="cmdSortBy === 'host_name'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('host_name')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      :label="t('account.typeColumn')"
                      :active="cmdSortBy === 'module'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('module')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      :label="t('account.commandColumn')"
                      :active="cmdSortBy === 'command'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('command')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      :label="t('common.user')"
                      :active="cmdSortBy === 'triggered_by'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('triggered_by')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      :label="t('common.status')"
                      :active="cmdSortBy === 'status'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('status')"
                    />
                  </th>
                  <th>{{ t('account.durationColumn') }}</th>
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
                    <EmptyState :title="t('account.noCommandsRecordedTitle')" />
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
                      :title="t('account.viewLogsTooltip')"
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
                      class="btn btn-icon btn-sm btn-ghost-danger ms-1"
                      :disabled="cancellingId === cmd.id"
                      :title="t('common.cancel')"
                      :aria-label="t('account.cancelCommandAriaLabel')"
                      @click="cancelCmd(cmd.id)"
                    >
                      <span
                        v-if="cancellingId === cmd.id"
                        class="spinner-border spinner-border-sm"
                      />
                      <IconX
                        v-else
                        :size="16"
                        class="icon icon-sm"
                      />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="card-footer d-flex align-items-center justify-content-between">
            <div class="text-secondary small">
              {{ t('account.cmdsCountLabel', { count: cmdsTotal, page: cmdsPage, total: totalCmdsPages }, cmdsTotal) }}
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
        :title="t('common.commandLogTitle')"
        :empty-text="t('common.commandLogEmptyText')"
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
            {{ t('account.allConnectionsTitle') }}
          </h3>
        </div>
        <ConnectionsTable
          :events="connexions"
          :loading="connexionsLoading"
          show-username
        />
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">
            {{ t('account.pageOfTotalLabel', { page: connexionsPage, total: totalConnexionsPages }) }}
          </div>
          <PaginationNav
            :current-page="connexionsPage"
            :total-pages="totalConnexionsPages"
            @select="selectConnexionsPage"
          />
        </div>
      </div>
    </div>

    <!-- ── Journal tab (raw audit_logs, admin only) ───────────────────────── -->
    <div v-show="activeTab === 'journal'">
      <div class="card">
        <div class="card-header d-flex flex-wrap align-items-center justify-content-between gap-2">
          <h3 class="card-title mb-0">
            {{ t('account.auditJournalTitle') }}
          </h3>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="journalExporting"
            @click="exportJournal"
          >
            <span
              v-if="journalExporting"
              class="spinner-border spinner-border-sm me-1"
            />
            <IconDownload
              v-else
              :size="14"
              class="icon me-1"
            />
            {{ t('account.exportCsvButton') }}
          </button>
        </div>
        <div class="card-body border-bottom py-3">
          <div class="row g-2">
            <div class="col-6 col-md-3">
              <select
                v-model="journalCategoryFilter"
                class="form-select form-select-sm"
              >
                <option
                  v-for="opt in journalCategoryFilterOptions"
                  :key="opt.value"
                  :value="opt.value"
                >
                  {{ opt.label }}
                </option>
              </select>
            </div>
            <div class="col-6 col-md-3">
              <input
                v-model="journalFrom"
                type="date"
                class="form-control form-control-sm"
                :aria-label="t('account.sinceFilterAriaLabel')"
              >
            </div>
            <div class="col-6 col-md-3">
              <input
                v-model="journalTo"
                type="date"
                class="form-control form-control-sm"
                :aria-label="t('account.untilFilterAriaLabel')"
              >
            </div>
          </div>
        </div>
        <div class="table-responsive scroll-table">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>{{ t('account.dateColumn') }}</th>
                <th>{{ t('account.categoryColumn') }}</th>
                <th>{{ t('account.actionColumn') }}</th>
                <th>{{ t('common.user') }}</th>
                <th>{{ t('account.hostColumnLabel') }}</th>
                <th>{{ t('common.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="journalLoading">
                <td
                  colspan="6"
                  class="py-2"
                >
                  <LoadingSkeleton
                    variant="table"
                    :lines="5"
                  />
                </td>
              </tr>
              <tr v-else-if="!journalLogs.length">
                <td colspan="6">
                  <EmptyState :title="t('account.noJournalEntriesTitle')" />
                </td>
              </tr>
              <tr
                v-for="log in journalLogs"
                :key="log.id"
              >
                <td class="text-secondary small">
                  {{ formatDate(log.created_at) }}
                </td>
                <td>
                  <span class="badge bg-secondary-lt text-secondary">{{ log.category }}</span>
                </td>
                <td>
                  <code class="small">{{ log.action }}</code>
                </td>
                <td class="text-secondary small">
                  {{ log.username || '—' }}
                </td>
                <td>
                  <router-link
                    v-if="log.host_id"
                    :to="`/hosts/${log.host_id}`"
                    class="text-decoration-none"
                  >
                    {{ log.host_name || log.host_id }}
                  </router-link>
                  <span v-else>—</span>
                </td>
                <td>
                  <span :class="statusClass(log.status)">{{ statusLabel(log.status) }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">
            {{ t('account.journalPageLabel', { page: journalPage }) }}
          </div>
          <div class="d-flex gap-2">
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              :disabled="journalPage <= 1"
              @click="selectJournalPage(journalPage - 1)"
            >
              {{ t('account.previousButton') }}
            </button>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              :disabled="!journalHasMore"
              @click="selectJournalPage(journalPage + 1)"
            >
              {{ t('account.nextButton') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconDownload, IconList, IconX } from '@tabler/icons-vue'
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

const { t } = useI18n()
const { formatLocaleDateTime: formatDate } = useDateFormatter()

const {
  auth,
  canViewCommands,
  activeTab,
  switchToCommandes,
  switchToConnexions,
  switchToJournal,
  journalLogs,
  journalPage,
  journalLoading,
  journalExporting,
  journalHasMore,
  journalCategoryFilter,
  journalFrom,
  journalTo,
  selectJournalPage,
  exportJournal,
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
  moduleFilterOptions,
  journalCategoryFilterOptions,
  statusLabel,
  cmdLabel,
  formatDuration,
  statusClass,
} = useAuditLogs()
</script>

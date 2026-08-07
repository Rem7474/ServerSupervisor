<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      label="Tâches planifiées"
      :interval-sec="TASKS_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="page-header mb-3">
      <div class="d-flex align-items-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              Dashboard
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>Tâches planifiées</span>
          </div>
          <h2 class="page-title">
            Tâches planifiées
          </h2>
        </div>
        <div class="d-flex gap-2">
          <button
            v-if="canManage"
            type="button"
            class="btn btn-primary btn-sm"
            @click="openCreate"
          >
            + Nouvelle tâche
          </button>
        </div>
      </div>
    </div>

    <DataToolbar
      searchable
      :search="filterText"
      search-placeholder="Rechercher une tâche…"
      @update:search="filterText = $event"
    >
      <template #right>
        <span class="text-muted small">
          {{ filteredTasks.length }}&thinsp;/&thinsp;{{ tasks.length }}
          tâche{{ tasks.length !== 1 ? 's' : '' }}
        </span>
      </template>
      <template #bottom>
        <div class="d-flex flex-wrap gap-2 align-items-center">
          <select
            v-model="filterHost"
            class="form-select form-select-sm tasks-filter-select"
          >
            <option value="">
              Tous les hôtes
            </option>
            <option
              v-for="host in hostList"
              :key="host"
              :value="host"
            >
              {{ host }}
            </option>
          </select>
          <select
            v-model="filterModule"
            class="form-select form-select-sm tasks-filter-select"
          >
            <option value="">
              Tous les modules
            </option>
            <option value="apt">
              apt
            </option>
            <option value="docker">
              docker
            </option>
            <option value="systemd">
              systemd
            </option>
            <option value="journal">
              journal
            </option>
            <option value="processes">
              processes
            </option>
            <option value="custom">
              custom
            </option>
            <option value="restic">
              restic
            </option>
          </select>
          <select
            v-model="filterStatus"
            class="form-select form-select-sm tasks-filter-select"
          >
            <option value="">
              Tous les statuts
            </option>
            <option value="enabled">
              Activées
            </option>
            <option value="disabled">
              Désactivées
            </option>
            <option value="manual">
              Manuelles
            </option>
            <option value="failed">
              En échec
            </option>
          </select>
        </div>
      </template>
    </DataToolbar>

    <div
      v-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <div class="card">
      <div
        v-if="loading"
        class="card-body"
      >
        <LoadingSkeleton variant="table" />
      </div>
      <div
        v-else-if="!filteredTasks.length"
        class="card-body"
      >
        <EmptyState
          :icon="IconClock"
          title="Aucune tâche trouvée"
          :subtitle="tasks.length ? 'Modifiez vos filtres.' : canManage ? 'Cliquez sur « Nouvelle tâche » pour commencer.' : 'Aucune tâche configurée.'"
        />
      </div>
      <div
        v-else
        class="table-responsive scroll-table"
      >
        <table class="table table-vcenter table-hover card-table mb-0">
          <thead>
            <tr>
              <th
                v-if="canManage"
                class="tasks-select-col"
              >
                <label class="form-check mb-0">
                  <input
                    class="form-check-input"
                    type="checkbox"
                    :checked="allVisibleSelected"
                    aria-label="Sélectionner toutes les tâches affichées"
                    @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
                  >
                </label>
              </th>
              <th>
                <SortableHeader
                  label="Hôte"
                  :active="sortKey === 'host_name'"
                  :direction="sortDir"
                  @toggle="toggleSort('host_name')"
                />
              </th>
              <th>
                <SortableHeader
                  label="Nom"
                  :active="sortKey === 'name'"
                  :direction="sortDir"
                  @toggle="toggleSort('name')"
                />
              </th>
              <th class="d-none d-sm-table-cell">
                Module / Action
              </th>
              <th class="d-none d-md-table-cell">
                <SortableHeader
                  label="Planification"
                  :active="sortKey === 'next_run_at'"
                  :direction="sortDir"
                  @toggle="toggleSort('next_run_at')"
                />
              </th>
              <th class="d-none d-md-table-cell">
                <SortableHeader
                  label="Dernier résultat"
                  :active="sortKey === 'last_run_at'"
                  :direction="sortDir"
                  @toggle="toggleSort('last_run_at')"
                />
              </th>
              <th>Activée</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="task in filteredTasks"
              :key="task.id"
              :class="{ 'table-active': selectedIds.has(task.id), 'opacity-60': !task.enabled }"
            >
              <td v-if="canManage">
                <label class="form-check mb-0">
                  <input
                    class="form-check-input"
                    type="checkbox"
                    :checked="selectedIds.has(task.id)"
                    :aria-label="`Sélectionner ${task.name}`"
                    @change="toggleSelected(task.id, ($event.target as HTMLInputElement).checked)"
                  >
                </label>
              </td>
              <td>
                <router-link
                  :to="`/hosts/${task.host_id}`"
                  class="text-decoration-none fw-medium"
                >
                  {{ task.host_name }}
                </router-link>
              </td>
              <td>{{ task.name }}</td>
              <td class="d-none d-sm-table-cell">
                <span class="badge bg-blue-lt me-1">{{ task.module }}</span>
                <span class="text-secondary small">{{ task.action }}</span>
                <span
                  v-if="task.target"
                  class="text-muted small ms-1"
                >— {{ task.target }}</span>
              </td>
              <td class="d-none d-md-table-cell">
                <span
                  v-if="isManualOnly(task)"
                  class="badge bg-secondary-lt text-secondary"
                >Manuel</span>
                <template v-else>
                  <code class="small">{{ task.cron_expression }}</code>
                  <div
                    v-if="describeCron(task.cron_expression)"
                    class="text-muted small"
                  >
                    {{ describeCron(task.cron_expression) }}
                  </div>
                  <div
                    v-if="task.next_run_at"
                    class="text-primary small"
                  >
                    → {{ formatDate(task.next_run_at) }}
                  </div>
                </template>
              </td>
              <td class="d-none d-md-table-cell">
                <span
                  v-if="task.last_run_status"
                  :class="statusBadge(task.last_run_status)"
                >
                  {{ commandStatusLabel(task.last_run_status) }}
                  <span
                    v-if="task.last_run_at"
                    class="ms-1 text-muted small"
                  >{{ formatDate(task.last_run_at) }}</span>
                </span>
                <span
                  v-else
                  class="text-muted"
                >jamais</span>
              </td>
              <td>
                <span
                  v-if="isManualOnly(task)"
                  class="text-muted small"
                >—</span>
                <span
                  v-else-if="!canManage"
                  class="badge"
                  :class="task.enabled ? 'bg-success-lt' : 'bg-secondary-lt'"
                >
                  {{ task.enabled ? 'Oui' : 'Non' }}
                </span>
                <input
                  v-else
                  type="checkbox"
                  class="form-check-input"
                  :checked="task.enabled"
                  @change="toggleTask(task)"
                >
              </td>
              <td class="text-end">
                <div class="d-flex gap-1 justify-content-end">
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    title="Historique d'exécutions"
                    @click="openHistory(task)"
                  >
                    <IconClock
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="canManage"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    :disabled="runningId === task.id"
                    title="Exécuter"
                    aria-label="Exécuter la tâche maintenant"
                    @click="runNow(task)"
                  >
                    <span
                      v-if="runningId === task.id"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerPlay
                      v-else
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="canManage"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    title="Modifier"
                    @click="openEdit(task)"
                  >
                    <IconPencil
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="canManage"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    title="Supprimer"
                    @click="confirmDelete(task)"
                  >
                    <IconTrash
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <BulkActionBar
      :count="selectedIds.size"
      @clear="clearSelection"
    >
      <button
        type="button"
        class="btn btn-sm btn-success"
        :disabled="bulkLoading"
        @click="handleBulkEnable(selectedTasks)"
      >
        Activer
      </button>
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary"
        :disabled="bulkLoading"
        @click="handleBulkDisable(selectedTasks)"
      >
        Désactiver
      </button>
      <button
        type="button"
        class="btn btn-sm btn-outline-primary"
        :disabled="bulkLoading"
        @click="handleBulkRun(selectedTasks)"
      >
        Exécuter
      </button>
      <button
        type="button"
        class="btn btn-sm btn-outline-danger"
        :disabled="bulkLoading"
        @click="handleBulkDelete(selectedTasks)"
      >
        Supprimer
      </button>
    </BulkActionBar>

    <!-- Create task modal -->
    <template v-if="createModalOpen">
      <div
        ref="createModalRef"
        class="modal modal-blur fade show d-block"
        tabindex="-1"
        @click.self="createModalOpen = false"
      >
        <div class="modal-dialog modal-lg modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                Nouvelle tâche planifiée
              </h5>
              <button
                type="button"
                class="btn-close"
                @click="createModalOpen = false"
              />
            </div>
            <form @submit.prevent="saveCreate">
              <div class="modal-body">
                <div
                  v-if="createError"
                  class="alert alert-danger py-2 mb-3"
                >
                  {{ createError }}
                </div>
                <div class="row g-3">
                  <div class="col-md-6">
                    <label class="form-label required">Nom</label>
                    <input
                      v-model="createForm.name"
                      type="text"
                      class="form-control"
                      placeholder="Ex: Mise à jour quotidienne"
                      required
                    >
                  </div>
                  <div class="col-12">
                    <DispatchStepEditor
                      v-model:host-id="createForm.host_id"
                      v-model:module="createForm.module"
                      v-model:action="createForm.action"
                      v-model:target="createForm.target"
                      v-model:cron-expression="createForm.cron_expression"
                      :actions-for-module="scheduledTaskActionsForModule"
                      :target-config="scheduledTaskTargetConfig"
                      :modules="scheduledTaskModules"
                      :show-cron="!createManualOnly"
                    />
                  </div>
                  <div class="col-12">
                    <label class="form-check form-switch">
                      <input
                        v-model="createManualOnly"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
                    </label>
                  </div>
                  <div
                    v-if="!createManualOnly && createNextRun"
                    class="col-12"
                  >
                    <div class="form-hint text-primary">
                      → prochain : {{ formatDate(createNextRun?.toISOString()) }}
                    </div>
                  </div>
                  <div
                    v-if="!createManualOnly"
                    class="col-12"
                  >
                    <label class="form-check">
                      <input
                        v-model="createForm.enabled"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Activée (planifiée automatiquement)</span>
                    </label>
                  </div>
                </div>
              </div>
              <div class="modal-footer">
                <button
                  type="button"
                  class="btn link-secondary"
                  :disabled="createSaving"
                  @click="createModalOpen = false"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  class="btn btn-primary"
                  :disabled="createSaving || !createForm.host_id"
                >
                  <span
                    v-if="createSaving"
                    class="spinner-border spinner-border-sm me-1"
                  />
                  Créer la tâche
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show" />
    </template>

    <!-- Edit task modal -->
    <template v-if="editTask">
      <div
        ref="editModalRef"
        class="modal modal-blur fade show d-block"
        tabindex="-1"
        @click.self="editTask = null"
      >
        <div class="modal-dialog modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                Modifier la tâche
              </h5>
              <button
                type="button"
                class="btn-close"
                @click="editTask = null"
              />
            </div>
            <div class="modal-body">
              <div class="mb-3">
                <label class="form-label">Nom</label>
                <input
                  v-model="editForm.name"
                  type="text"
                  class="form-control"
                >
              </div>
              <div class="mb-3 form-check form-switch">
                <input
                  id="editManualOnly"
                  v-model="editManualOnly"
                  type="checkbox"
                  class="form-check-input"
                >
                <label
                  class="form-check-label"
                  for="editManualOnly"
                >Exécution manuelle uniquement (pas de planification automatique)</label>
              </div>
              <div
                v-if="!editManualOnly"
                class="mb-3"
              >
                <label class="form-label">Planification</label>
                <CronBuilder v-model="editForm.cron_expression" />
                <div
                  v-if="editNextRun"
                  class="form-hint text-primary"
                >
                  → prochain : {{ formatDate(editNextRun?.toISOString()) }}
                </div>
              </div>
              <div
                v-if="!editManualOnly"
                class="mb-3 form-check"
              >
                <input
                  id="editEnabled"
                  v-model="editForm.enabled"
                  type="checkbox"
                  class="form-check-input"
                >
                <label
                  class="form-check-label"
                  for="editEnabled"
                >Activée</label>
              </div>
              <div
                v-if="editError"
                class="alert alert-danger py-2"
              >
                {{ editError }}
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn btn-secondary"
                @click="editTask = null"
              >
                Annuler
              </button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="editSaving"
                @click="saveEdit"
              >
                <span
                  v-if="editSaving"
                  class="spinner-border spinner-border-sm me-1"
                />
                Enregistrer
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show" />
    </template>

    <!-- Execution history modal -->
    <template v-if="historyTask">
      <div
        ref="historyModalRef"
        class="modal modal-blur fade show d-block"
        tabindex="-1"
        @click.self="historyTask = null"
      >
        <div class="modal-dialog modal-xl modal-dialog-centered modal-dialog-scrollable">
          <div class="modal-content">
            <div class="modal-header">
              <div>
                <h5 class="modal-title mb-0">
                  Historique d'exécutions
                </h5>
                <div class="text-muted small mt-1">
                  <span class="badge bg-blue-lt me-1">{{ historyTask.module }}</span>
                  {{ historyTask.name }}
                  <span class="text-muted ms-1">— {{ historyTask.host_name }}</span>
                </div>
              </div>
              <button
                type="button"
                class="btn-close"
                @click="historyTask = null"
              />
            </div>
            <div class="modal-body p-0">
              <div v-if="historyLoading">
                <LoadingSkeleton variant="table" />
              </div>
              <div
                v-else-if="historyError"
                class="alert alert-danger m-3"
              >
                {{ historyError }}
              </div>
              <EmptyState
                v-else-if="!executions.length"
                title="Aucune exécution enregistrée pour cette tâche."
              />
              <div v-else>
                <table class="table table-vcenter table-hover mb-0">
                  <thead>
                    <tr>
                      <th>Date</th>
                      <th>Statut</th>
                      <th>Durée</th>
                      <th>Déclenché par</th>
                      <th class="text-end" />
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="ex in executions"
                      :key="ex.id"
                    >
                      <td class="text-nowrap">
                        {{ formatDate(ex.created_at) }}
                      </td>
                      <td>
                        <span :class="statusBadge(ex.status)">{{ commandStatusLabel(ex.status) }}</span>
                      </td>
                      <td class="text-nowrap">
                        <span v-if="ex.ended_at && ex.started_at">{{ durationSec(ex.started_at, ex.ended_at) }}s</span>
                        <span
                          v-else
                          class="text-muted"
                        >—</span>
                      </td>
                      <td>{{ ex.triggered_by || '—' }}</td>
                      <td class="text-end">
                        <button
                          type="button"
                          class="btn btn-icon btn-sm btn-ghost-secondary"
                          title="Voir les logs"
                          aria-label="Voir les logs"
                          @click="watchExecution(historyTask, ex)"
                        >
                          <IconList
                            :size="16"
                            class="icon icon-sm"
                          />
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div class="modal-footer">
              <span class="text-muted small me-auto">{{ executions.length }} exécution{{ executions.length !== 1 ? 's' : '' }} (20 dernières)</span>
              <button
                type="button"
                class="btn btn-secondary"
                @click="historyTask = null"
              >
                Fermer
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show" />
    </template>

    <CommandLogPanel
      :command="(liveCommand as any)"
      :show="showConsole"
      title="Logs de l'exécution"
      empty-text="Aucune console active"
      wrapper-class="side-panel"
      @open="showConsole = true"
      @close="closeExecutionConsole"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconClock, IconList, IconPencil, IconPlayerPlay, IconTrash } from '@tabler/icons-vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import BulkActionBar from '../components/BulkActionBar.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import CronBuilder from '../components/CronBuilder.vue'
import DispatchStepEditor from '../components/DispatchStepEditor.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { availableScheduledTaskModules, scheduledTaskActions, scheduledTaskTargetConfig } from '../utils/scheduledTaskDispatch'
import { useGlobalScheduledTasks } from '../composables/useGlobalScheduledTasks'
import { useModalChrome } from '../composables/useModalChrome'

const {
  hostsStore,
  tasks,
  loading,
  error,
  autoRefresh,
  lastUpdatedAt,
  TASKS_REFRESH_SEC,
  runningId,
  filterText,
  filterHost,
  filterModule,
  filterStatus,
  sortKey,
  sortDir,
  selectedIds,
  bulkLoading,
  editTask,
  editForm,
  editManualOnly,
  editSaving,
  editError,
  historyTask,
  executions,
  historyLoading,
  historyError,
  liveCommand,
  showConsole,
  watchExecution,
  closeExecutionConsole,
  createModalOpen,
  createForm,
  createManualOnly,
  createSaving,
  createError,
  canManage,
  createNextRun,
  editNextRun,
  hostList,
  filteredTasks,
  allVisibleSelected,
  selectedTasks,
  toggleSelected,
  toggleSelectAll,
  clearSelection,
  toggleSort,
  openCreate,
  saveCreate,
  formatDate,
  statusBadge,
  commandStatusLabel,
  durationSec,
  isManualOnly,
  describeCron,
  openEdit,
  saveEdit,
  confirmDelete,
  openHistory,
  toggleTask,
  runNow,
  handleBulkEnable,
  handleBulkDisable,
  handleBulkDelete,
  handleBulkRun,
} = useGlobalScheduledTasks()

const createModalRef = ref<HTMLElement | null>(null)
const editModalRef = ref<HTMLElement | null>(null)
const historyModalRef = ref<HTMLElement | null>(null)
useModalChrome(createModalRef, () => createModalOpen.value, { onClose: () => { createModalOpen.value = false } })
useModalChrome(editModalRef, () => !!editTask.value, { onClose: () => { editTask.value = null } })
useModalChrome(historyModalRef, () => !!historyTask.value, { onClose: () => { historyTask.value = null } })

// The scheduled-task-specific module list/actions/target config live in
// utils/scheduledTaskDispatch.ts, shared with HostTasksTab.vue's per-host
// editor so both stay in sync (see that file's doc comment for why restic
// is scoped here rather than to the shared DISPATCH_MODULES). Narrowed to
// whichever modules the selected host's agent actually reports as active
// (collectors) — a host with e.g. collect_docker off shouldn't offer it.
const scheduledTaskModules = computed(() => {
  const host = hostsStore.hosts.find((h) => h.id === createForm.value.host_id)
  return availableScheduledTaskModules(host?.collectors)
})
const scheduledTaskActionsForModule = scheduledTaskActions

// Reset module/action back to something valid for the newly selected host
// when the current pick falls outside its available modules.
watch(() => createForm.value.host_id, () => {
  if (scheduledTaskModules.value.some((m) => m.value === createForm.value.module)) return
  const fallback = scheduledTaskModules.value[0]?.value || 'apt'
  createForm.value.module = fallback
  createForm.value.action = scheduledTaskActionsForModule(fallback)[0]?.value || ''
  createForm.value.target = ''
})
</script>

<style scoped>
.tasks-select-col {
  width: 1%;
}

.tasks-filter-select {
  min-width: 150px;
}

@media (max-width: 576px) {
  .tasks-filter-select {
    min-width: 0;
    width: 100%;
  }
}
</style>

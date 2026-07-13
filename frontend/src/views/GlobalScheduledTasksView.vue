<template>
  <div>
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
          <button
            type="button"
            class="btn btn-outline-secondary btn-sm"
            @click="loadTasks"
          >
            <IconRefresh
              :size="16"
              class="icon icon-sm"
            />
            Actualiser
          </button>
        </div>
      </div>
    </div>

    <DataToolbar
      searchable
      :search="filterText"
      search-placeholder="Rechercher une tâche..."
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
        class="card-body text-center py-5"
      >
        <span class="spinner-border text-primary" />
      </div>
      <div
        v-else-if="!filteredTasks.length"
        class="card-body text-center py-5"
      >
        <IconClock
          :size="40"
          class="icon mb-3 text-muted"
          :stroke-width="1.5"
        />
        <h3 class="mb-1">
          Aucune tâche trouvée
        </h3>
        <p class="text-secondary mb-0">
          {{ tasks.length ? 'Modifiez vos filtres.' : canManage ? 'Cliquez sur « Nouvelle tâche » pour commencer.' : 'Aucune tâche configurée.' }}
        </p>
      </div>
      <div
        v-else
        class="table-responsive"
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
                Planification
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
              :class="{ 'table-active': selectedIds.has(task.id) }"
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
                  {{ task.last_run_status }}
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
                    class="btn btn-sm btn-outline-secondary"
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
                    class="btn btn-sm btn-outline-primary"
                    :disabled="runningId === task.id"
                    @click="runNow(task)"
                  >
                    <span
                      v-if="runningId === task.id"
                      class="spinner-border spinner-border-sm"
                    />
                    <span v-else>Exécuter</span>
                  </button>
                  <button
                    v-if="canManage"
                    type="button"
                    class="btn btn-sm btn-outline-secondary"
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
                    class="btn btn-sm btn-outline-danger"
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
    <div
      v-if="createModalOpen"
      class="modal modal-blur show d-block"
      tabindex="-1"
      style="background:rgba(0,0,0,.5);z-index:1050"
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
                <div class="col-12">
                  <label class="form-label required">Hôte</label>
                  <select
                    v-model="createForm.host_id"
                    class="form-select"
                    required
                  >
                    <option value="">
                      Sélectionner un hôte...
                    </option>
                    <option
                      v-for="h in hostsStore.hosts"
                      :key="h.id"
                      :value="h.id"
                    >
                      {{ h.name || h.hostname || h.ip_address }}
                    </option>
                  </select>
                </div>
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
                <div class="col-md-3">
                  <label class="form-label required">Module</label>
                  <select
                    v-model="createForm.module"
                    class="form-select"
                    required
                    @change="onModuleChange"
                  >
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
                  </select>
                </div>
                <div class="col-md-3">
                  <label class="form-label required">Action</label>
                  <select
                    v-if="moduleActions[createForm.module]"
                    v-model="createForm.action"
                    class="form-select"
                    required
                  >
                    <option
                      v-for="a in moduleActions[createForm.module]"
                      :key="a"
                      :value="a"
                    >
                      {{ a }}
                    </option>
                  </select>
                  <input
                    v-else
                    v-model="createForm.action"
                    type="text"
                    class="form-control"
                    required
                  >
                </div>
                <div
                  v-if="targetLabel(createForm.module)"
                  class="col-12"
                >
                  <label class="form-label">{{ targetLabel(createForm.module) }}</label>
                  <input
                    v-model="createForm.target"
                    type="text"
                    class="form-control"
                    :placeholder="targetPlaceholder(createForm.module)"
                  >
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
                  v-if="!createManualOnly"
                  class="col-12"
                >
                  <label class="form-label">Planification</label>
                  <CronBuilder v-model="createForm.cron_expression" />
                  <div
                    v-if="createNextRun"
                    class="form-hint text-primary"
                  >
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

    <!-- Edit task modal -->
    <div
      v-if="editTask"
      class="modal modal-blur show d-block"
      tabindex="-1"
      style="background:rgba(0,0,0,.5);z-index:1050"
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

    <!-- Execution history modal -->
    <div
      v-if="historyTask"
      class="modal modal-blur show d-block"
      tabindex="-1"
      style="background:rgba(0,0,0,.5);z-index:1050"
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
            <div
              v-if="historyLoading"
              class="text-center py-5"
            >
              <span class="spinner-border text-primary" />
            </div>
            <div
              v-else-if="historyError"
              class="alert alert-danger m-3"
            >
              {{ historyError }}
            </div>
            <div
              v-else-if="!executions.length"
              class="text-center py-5 text-muted"
            >
              Aucune exécution enregistrée pour cette tâche.
            </div>
            <div v-else>
              <table class="table table-vcenter table-hover mb-0">
                <thead>
                  <tr>
                    <th>Date</th>
                    <th>Statut</th>
                    <th>Durée</th>
                    <th>Déclenché par</th>
                    <th>Sortie</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="ex in executions"
                    :key="ex.id"
                    :class="expandedId === ex.id ? 'table-active' : ''"
                  >
                    <td class="text-nowrap">
                      {{ formatDate(ex.created_at) }}
                    </td>
                    <td>
                      <span :class="statusBadge(ex.status)">{{ ex.status }}</span>
                    </td>
                    <td class="text-nowrap">
                      <span v-if="ex.ended_at && ex.started_at">{{ durationSec(ex.started_at, ex.ended_at) }}s</span>
                      <span
                        v-else
                        class="text-muted"
                      >—</span>
                    </td>
                    <td>{{ ex.triggered_by || '—' }}</td>
                    <td style="max-width:400px">
                      <div
                        v-if="!ex.output"
                        class="text-muted small"
                      >
                        —
                      </div>
                      <template v-else>
                        <div
                          v-if="expandedId !== ex.id"
                          class="d-flex align-items-center gap-2"
                        >
                          <span
                            class="text-truncate small font-monospace"
                            style="max-width:300px"
                          >{{ firstLine(ex.output) }}</span>
                          <button
                            type="button"
                            class="btn btn-xs btn-ghost-secondary ms-auto flex-shrink-0"
                            @click="expandedId = ex.id"
                          >
                            Voir tout
                          </button>
                        </div>
                        <div v-else>
                          <pre
                            class="mb-1 small"
                            style="max-height:300px;overflow-y:auto;white-space:pre-wrap;word-break:break-all"
                          >{{ ex.output }}</pre>
                          <button
                            type="button"
                            class="btn btn-xs btn-ghost-secondary"
                            @click="expandedId = null"
                          >
                            Réduire
                          </button>
                        </div>
                      </template>
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
  </div>
</template>

<script setup lang="ts">
import { IconClock, IconPencil, IconRefresh, IconTrash } from '@tabler/icons-vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import BulkActionBar from '../components/BulkActionBar.vue'
import CronBuilder from '../components/CronBuilder.vue'
import { useGlobalScheduledTasks } from '../composables/useGlobalScheduledTasks'

const {
  hostsStore,
  tasks,
  loading,
  error,
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
  expandedId,
  createModalOpen,
  createForm,
  createManualOnly,
  createSaving,
  createError,
  canManage,
  moduleActions,
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
  targetLabel,
  targetPlaceholder,
  onModuleChange,
  openCreate,
  saveCreate,
  formatDate,
  statusBadge,
  durationSec,
  firstLine,
  isManualOnly,
  describeCron,
  openEdit,
  saveEdit,
  confirmDelete,
  openHistory,
  loadTasks,
  toggleTask,
  runNow,
  handleBulkEnable,
  handleBulkDisable,
  handleBulkDelete,
  handleBulkRun,
} = useGlobalScheduledTasks()
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

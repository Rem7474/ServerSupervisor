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
              <polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" />
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
            </svg>
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
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="icon mb-3 text-muted"
          width="40"
          height="40"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle
            cx="12"
            cy="12"
            r="10"
          /><polyline points="12 6 12 12 16 14" />
        </svg>
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
            >
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
                      <circle
                        cx="12"
                        cy="12"
                        r="10"
                      /><polyline points="12 6 12 12 16 14" />
                    </svg>
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
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                    </svg>
                  </button>
                  <button
                    v-if="canManage"
                    type="button"
                    class="btn btn-sm btn-outline-danger"
                    title="Supprimer"
                    @click="confirmDelete(task)"
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
                      <polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14H6L5 6" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M9 6V4h6v2" />
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>


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
                  <label class="form-label">
                    Planification (cron)
                    <span class="text-muted ms-1 small">— laisser vide pour manuel uniquement</span>
                  </label>
                  <input
                    v-model="createForm.cron_expression"
                    type="text"
                    class="form-control font-monospace"
                    placeholder="ex: 0 3 * * *"
                  >
                  <div
                    v-if="createForm.cron_expression"
                    class="form-hint"
                  >
                    <span v-if="createCronDesc">{{ createCronDesc }}</span>
                    <span
                      v-if="createNextRun"
                      :class="createCronDesc ? 'ms-2 text-primary' : 'text-primary'"
                    >→ prochain : {{ formatDate(createNextRun?.toISOString()) }}</span>
                    <span
                      v-else-if="!createCronDesc"
                      class="text-warning"
                    >Expression non reconnue</span>
                  </div>
                  <div class="mt-2 d-flex flex-wrap gap-2">
                    <button
                      v-for="preset in cronPresets"
                      :key="preset.value"
                      type="button"
                      class="btn btn-sm btn-outline-secondary"
                      @click="createForm.cron_expression = preset.value"
                    >
                      {{ preset.label }}
                    </button>
                  </div>
                </div>
                <div class="col-12">
                  <label class="form-check">
                    <input
                      v-model="createForm.enabled"
                      type="checkbox"
                      class="form-check-input"
                      :disabled="!createForm.cron_expression"
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
            <div class="mb-3">
              <label class="form-label">Expression cron</label>
              <input
                v-model="editForm.cron_expression"
                type="text"
                class="form-control font-monospace"
                placeholder="ex: 0 3 * * *"
              >
              <div
                v-if="editForm.cron_expression"
                class="form-hint"
              >
                <span v-if="editCronDesc">{{ editCronDesc }}</span>
                <span
                  v-if="editNextRun"
                  :class="editCronDesc ? 'ms-2 text-primary' : 'text-primary'"
                >→ prochain : {{ formatDate(editNextRun?.toISOString()) }}</span>
                <span
                  v-else-if="!editCronDesc"
                  class="text-warning"
                >Expression non reconnue</span>
              </div>
            </div>
            <div class="mb-3 form-check">
              <input
                id="editEnabled"
                v-model="editForm.enabled"
                type="checkbox"
                class="form-check-input"
                :disabled="isManualOnly(editTask)"
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
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useHostsStore } from '../stores/hosts'
import { addToast } from '../composables/useGlobalToast'
import api from '../api'
import { isManualOnly, describeCron, nextCronRun, MANUAL_SENTINEL } from '../utils/cron'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import DataToolbar from '../components/common/DataToolbar.vue'
import SortableHeader from '../components/common/SortableHeader.vue'
import type { ScheduledTaskWithHost } from '../types/task'

const auth = useAuthStore()
const hostsStore = useHostsStore()
const dialog = useConfirmDialog()

const tasks = ref<ScheduledTaskWithHost[]>([])
const loading = ref(false)
const error = ref('')
const runningId = ref<string | number | null>(null)

const filterText = ref('')
const filterHost = ref('')
const filterModule = ref('')
const filterStatus = ref('')
const sortKey = ref('name')
const sortDir = ref<'asc' | 'desc'>('asc')

const editTask = ref<any>(null)
const editForm = ref({ name: '', cron_expression: '', enabled: false })
const editSaving = ref(false)
const editError = ref('')

const historyTask = ref<any>(null)
const executions = ref<any[]>([])
const historyLoading = ref(false)
const historyError = ref('')
const expandedId = ref<string | number | null>(null)

const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

const moduleActions: Record<string, string[]> = {
  apt: ['update', 'upgrade', 'install', 'remove'],
  docker: ['start', 'stop', 'restart', 'pull', 'prune'],
  systemd: ['restart', 'start', 'stop', 'enable', 'disable'],
  journal: ['tail'],
  processes: ['list'],
}

const cronPresets = [
  { label: 'Toutes les heures', value: '@hourly' },
  { label: 'Tous les jours à 3h', value: '0 3 * * *' },
  { label: 'Dimanche minuit', value: '@weekly' },
  { label: '1er du mois', value: '@monthly' },
]

function targetLabel(module: string): string {
  if (module === 'docker') return 'Conteneur (nom ou ID)'
  if (module === 'systemd') return 'Service systemd'
  if (module === 'custom') return 'ID de tâche custom'
  if (module === 'apt') return 'Paquet (optionnel pour install/remove)'
  return ''
}

function targetPlaceholder(module: string): string {
  if (module === 'docker') return 'nginx'
  if (module === 'systemd') return 'nginx.service'
  if (module === 'custom') return 'my-deploy-task'
  if (module === 'apt') return 'nginx'
  return ''
}

function emptyCreateForm() {
  return { host_id: '', name: '', module: 'apt', action: 'update', target: '', cron_expression: '', enabled: false }
}

const createModalOpen = ref(false)
const createForm = ref(emptyCreateForm())
const createSaving = ref(false)
const createError = ref('')

const createCronDesc = computed(() => describeCron(createForm.value.cron_expression))
const createNextRun = computed(() => {
  const expr = createForm.value.cron_expression
  if (!expr || expr === MANUAL_SENTINEL) return null
  return nextCronRun(expr)
})

function onModuleChange(): void {
  const actions = moduleActions[createForm.value.module]
  createForm.value.action = actions ? actions[0] : ''
  createForm.value.target = ''
}

function openCreate(): void {
  createForm.value = emptyCreateForm()
  createError.value = ''
  createModalOpen.value = true
}

async function saveCreate(): Promise<void> {
  createSaving.value = true
  createError.value = ''
  try {
    const { host_id, ...body } = createForm.value
    const cron = body.cron_expression.trim() || MANUAL_SENTINEL
    await api.createScheduledTask(host_id, {
      name: body.name,
      module: body.module,
      action: body.action,
      target: body.target,
      payload: '',
      cron_expression: cron,
      enabled: cron !== MANUAL_SENTINEL && body.enabled,
    })
    createModalOpen.value = false
    await loadTasks()
  } catch (e: any) {
    createError.value = e?.response?.data?.error || 'Erreur lors de la création'
  } finally {
    createSaving.value = false
  }
}

const hostList = computed(() => {
  const names = [...new Set(tasks.value.map((t: any) => t.host_name))]
  return names.sort()
})

const editCronDesc = computed(() => describeCron(editForm.value.cron_expression))
const editNextRun = computed(() => {
  const expr = editForm.value.cron_expression
  if (!expr || expr === MANUAL_SENTINEL) return null
  return nextCronRun(expr)
})

const filteredTasks = computed(() => {
  const filtered = tasks.value.filter((task: any) => {
    if (filterHost.value && task.host_name !== filterHost.value) return false
    if (filterModule.value && task.module !== filterModule.value) return false
    if (filterStatus.value === 'enabled' && (!task.enabled || isManualOnly(task))) return false
    if (filterStatus.value === 'disabled' && (task.enabled || isManualOnly(task))) return false
    if (filterStatus.value === 'manual' && !isManualOnly(task)) return false
    if (filterStatus.value === 'failed' && task.last_run_status !== 'failed') return false
    if (filterText.value) {
      const q = filterText.value.toLowerCase()
      if (!task.name.toLowerCase().includes(q) &&
          !task.host_name.toLowerCase().includes(q) &&
          !task.module.toLowerCase().includes(q) &&
          !(task.target || '').toLowerCase().includes(q)) return false
    }
    return true
  })

  return [...filtered].sort((a, b) => {
    const key = sortKey.value
    const av = (a as Record<string, unknown>)[key] ?? ''
    const bv = (b as Record<string, unknown>)[key] ?? ''
    const cmp = String(av).localeCompare(String(bv), 'fr', { numeric: true })
    return sortDir.value === 'asc' ? cmp : -cmp
  })
})

function toggleSort(key: string): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}

function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

function statusBadge(status: string | undefined): string {
  if (status === 'completed') return 'badge bg-success-lt'
  if (status === 'failed') return 'badge bg-danger-lt'
  if (status === 'running') return 'badge bg-info-lt'
  return 'badge bg-warning-lt'
}

function durationSec(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime()
  return (ms / 1000).toFixed(1)
}

function firstLine(output: string | undefined): string {
  return (output || '').split('\n')[0].trim()
}

function openEdit(task: any): void {
  editTask.value = task
  editForm.value = { name: task.name, cron_expression: task.cron_expression, enabled: task.enabled }
  editError.value = ''
}

async function saveEdit(): Promise<void> {
  editSaving.value = true
  editError.value = ''
  try {
    await api.updateScheduledTask(editTask.value.id, {
      name: editForm.value.name,
      module: editTask.value.module,
      action: editTask.value.action,
      target: editTask.value.target,
      payload: editTask.value.payload,
      cron_expression: editForm.value.cron_expression,
      enabled: editForm.value.enabled,
    })
    editTask.value = null
    await loadTasks()
  } catch (e: any) {
    editError.value = e?.response?.data?.error || 'Erreur lors de la sauvegarde'
  } finally {
    editSaving.value = false
  }
}

async function confirmDelete(task: any): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Supprimer la tâche',
    message: `Supprimer « ${task.name} » sur ${task.host_name} ?`,
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteScheduledTask(task.id)
    await loadTasks()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur lors de la suppression'
  }
}

async function openHistory(task: any): Promise<void> {
  historyTask.value = task
  executions.value = []
  expandedId.value = null
  historyError.value = ''
  historyLoading.value = true
  try {
    const { data } = await api.getScheduledTaskExecutions(task.id, 20)
    executions.value = data
  } catch (e: any) {
    historyError.value = e?.response?.data?.error || 'Erreur de chargement'
  } finally {
    historyLoading.value = false
  }
}

async function loadTasks(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.getAllScheduledTasks()
    tasks.value = data
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur de chargement'
  } finally {
    loading.value = false
  }
}

async function toggleTask(task: any): Promise<void> {
  const enabling = !task.enabled
  const ok = await dialog.confirm({
    title: enabling ? 'Activer la tâche' : 'Désactiver la tâche',
    message: `Voulez-vous ${enabling ? 'activer' : 'désactiver'} « ${task.name} » sur ${task.host_name} ?`,
    variant: 'warning',
  })
  if (!ok) return
  try {
    await api.updateScheduledTask(task.id, {
      name: task.name, module: task.module, action: task.action,
      target: task.target, payload: task.payload,
      cron_expression: task.cron_expression, enabled: enabling,
    })
    await loadTasks()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur'
  }
}

async function runNow(task: any): Promise<void> {
  runningId.value = task.id
  try {
    const { data } = await api.runScheduledTask(task.id)
    addToast(`${task.name} déclenchée — commande ${data.command_id}`, 'success')
    await loadTasks()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur'
  } finally {
    runningId.value = null
  }
}

onMounted(() => {
  hostsStore.fetchHosts()
  loadTasks()
})
</script>

<style scoped>
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

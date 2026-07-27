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
        <span>Commandes</span>
      </div>
      <div class="d-flex align-items-center justify-content-between">
        <h2 class="page-title mb-0">
          Commandes en cours
          <span
            v-if="activeCount > 0"
            class="badge bg-blue-lt text-blue ms-2"
          >{{ activeCount }}</span>
        </h2>
        <div class="d-flex gap-2 align-items-center">
          <select
            v-model="statusFilter"
            class="form-select form-select-sm"
            style="width: auto"
          >
            <option value="">
              Tous statuts
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
            <option value="cancelled">
              Annulé
            </option>
          </select>
          <select
            v-model="moduleFilter"
            class="form-select form-select-sm"
            style="width: auto"
          >
            <option value="">
              Tous modules
            </option>
            <option value="docker">
              Docker
            </option>
            <option value="apt">
              APT
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
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="loading"
            @click="load"
          >
            <span
              v-if="loading"
              class="spinner-border spinner-border-sm me-1"
            />
            Actualiser
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div class="side-layout">
      <div class="side-main">
        <div class="card">
          <div
            v-if="loading && commands.length === 0"
            class="card-body text-center text-muted py-5"
          >
            <div class="spinner-border mb-2" />
            <div>Chargement…</div>
          </div>

          <div
            v-else-if="commands.length === 0"
            class="card-body"
          >
            <EmptyState
              :icon="IconTerminal2"
              title="Aucune commande trouvée."
            />
          </div>

          <div
            v-else
            class="table-responsive"
          >
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Hôte</th>
                  <th>Module</th>
                  <th>Action / Cible</th>
                  <th>Statut</th>
                  <th>Déclencheur</th>
                  <th>Créé</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="cmd in commands"
                  :key="cmd.id"
                >
                  <td class="text-muted small">
                    {{ cmd.host_name }}
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="moduleBadge(cmd.module)"
                    >{{ cmd.module }}</span>
                  </td>
                  <td>
                    <span class="fw-medium">{{ cmd.action }}</span>
                    <span
                      v-if="cmd.target"
                      class="text-muted ms-1"
                    >{{ cmd.target }}</span>
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="statusBadge(cmd.status)"
                    >{{ cmd.status }}</span>
                  </td>
                  <td class="text-muted small">
                    {{ cmd.triggered_by || '—' }}
                  </td>
                  <td class="text-muted small">
                    <RelativeTime :date="cmd.created_at" />
                  </td>
                  <td class="text-end">
                    <button
                      type="button"
                      class="btn btn-sm btn-outline-secondary me-1"
                      title="Voir les logs"
                      @click="openLogs(cmd)"
                    >
                      <IconFileText :size="14" />
                    </button>
                    <button
                      v-if="cmd.status === 'pending' || cmd.status === 'running'"
                      type="button"
                      class="btn btn-sm btn-outline-danger"
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

          <div
            v-if="totalPages > 1"
            class="card-footer d-flex align-items-center justify-content-between"
          >
            <span class="text-muted small">{{ total }} commande{{ total !== 1 ? 's' : '' }}</span>
            <PaginationNav
              :current-page="page"
              :total-pages="totalPages"
              @select="setPage"
            />
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="selectedCommand"
        :show="showLogPanel"
        title="Logs de la commande"
        empty-text="Aucune commande sélectionnée"
        wrapper-class="side-panel"
        @open="showLogPanel = true"
        @close="closeLogs"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconFileText, IconTerminal2 } from '@tabler/icons-vue'
import PaginationNav from '../components/PaginationNav.vue'
import RelativeTime from '../components/RelativeTime.vue'
import EmptyState from '../components/EmptyState.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { useActiveCommands } from '../composables/useActiveCommands'

const {
  commands,
  total,
  page,
  loading,
  error,
  cancellingId,
  statusFilter,
  moduleFilter,
  totalPages,
  activeCount,
  load,
  cancelCmd,
  setPage,
  moduleBadge,
  statusBadge,
  selectedCommand,
  showLogPanel,
  openLogs,
  closeLogs,
} = useActiveCommands()
</script>

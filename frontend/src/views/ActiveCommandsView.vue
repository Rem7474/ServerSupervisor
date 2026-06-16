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
        class="card-body text-center text-muted py-5"
      >
        Aucune commande trouvée.
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
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import api from '../api'
import { addToast } from '../composables/useGlobalToast'
import PaginationNav from '../components/PaginationNav.vue'
import RelativeTime from '../components/RelativeTime.vue'
import type { RemoteCommandWithHost } from '../types/audit'

const PAGE_SIZE = 50
const POLL_INTERVAL = 10_000

const commands = ref<RemoteCommandWithHost[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const error = ref('')
const cancellingId = ref<string | null>(null)
const statusFilter = ref('')
const moduleFilter = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const activeCount = computed(() => commands.value.filter((c) => c.status === 'pending' || c.status === 'running').length)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getCommandsHistory(page.value, PAGE_SIZE, {
      status: statusFilter.value || undefined,
      module: moduleFilter.value || undefined,
    })
    commands.value = res.data.commands || []
    total.value = res.data.total || 0
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || 'Erreur de chargement'
  } finally {
    loading.value = false
  }
}

async function cancelCmd(id: string): Promise<void> {
  cancellingId.value = id
  try {
    await api.cancelCommand(id)
    commands.value = commands.value.map((c) =>
      c.id === id ? { ...c, status: 'cancelled' } : c
    )
    addToast('Commande annulée', 'success')
  } catch (err: any) {
    addToast(err?.response?.data?.error || 'Impossible d\'annuler', 'error')
  } finally {
    cancellingId.value = null
  }
}

function setPage(p: number): void {
  page.value = p
  load()
}

function moduleBadge(module: string): string {
  const map: Record<string, string> = {
    docker: 'bg-blue-lt text-blue',
    apt: 'bg-yellow-lt text-yellow',
    systemd: 'bg-cyan-lt text-cyan',
    journal: 'bg-purple-lt text-purple',
    custom: 'bg-teal-lt text-teal',
  }
  return map[module] || 'bg-secondary-lt text-secondary'
}

function statusBadge(status: string): string {
  const map: Record<string, string> = {
    pending: 'bg-secondary-lt text-secondary',
    running: 'bg-blue-lt text-blue',
    completed: 'bg-green-lt text-green',
    failed: 'bg-red-lt text-red',
    cancelled: 'bg-orange-lt text-orange',
  }
  return map[status] || 'bg-secondary-lt text-secondary'
}

watch([statusFilter, moduleFilter], () => {
  page.value = 1
  load()
})

onMounted(() => {
  load()
  pollTimer = setInterval(load, POLL_INTERVAL)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

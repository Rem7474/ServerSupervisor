<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Nginx Proxy Manager
      </h3>
      <button
        v-if="authIsAdmin && !showForm"
        class="btn btn-sm btn-primary"
        @click="openAddForm"
      >
        + Ajouter une connexion
      </button>
    </div>

    <!-- Add / Edit form -->
    <div
      v-if="showForm && authIsAdmin"
      class="card-body border-bottom"
    >
      <div class="row g-3">
        <div class="col-md-6">
          <label class="form-label">Nom *</label>
          <input
            v-model="form.name"
            type="text"
            class="form-control"
            placeholder="Mon NPM"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">URL API *</label>
          <input
            v-model="form.api_url"
            type="text"
            class="form-control"
            placeholder="http://192.168.1.10:81"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Identifiant (email) *</label>
          <input
            v-model="form.identity"
            type="text"
            class="form-control"
            placeholder="admin@example.com"
            autocomplete="username"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Mot de passe {{ editingId ? '(vide = inchangé)' : '*' }}</label>
          <input
            v-model="form.secret"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
        <div class="col-md-4">
          <label class="form-label">Intervalle de rafraîchissement (s)</label>
          <input
            v-model.number="form.poll_interval_sec"
            type="number"
            class="form-control"
            min="60"
          >
        </div>
        <div class="col-md-4 d-flex align-items-end">
          <label class="form-check form-switch mb-0">
            <input
              v-model="form.enabled"
              class="form-check-input"
              type="checkbox"
            >
            <span class="form-check-label">Activé</span>
          </label>
        </div>
      </div>
      <div class="mt-3 d-flex align-items-center gap-2">
        <button
          class="btn btn-primary"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? 'Enregistrement…' : (editingId ? 'Mettre à jour' : 'Créer') }}
        </button>
        <button
          class="btn btn-outline-secondary"
          @click="cancelForm"
        >
          Annuler
        </button>
        <button
          class="btn btn-outline-info ms-2"
          :disabled="testing"
          @click="testForm"
        >
          {{ testing ? 'Test…' : 'Tester la connexion' }}
        </button>
        <span
          v-if="formMsg"
          :class="['ms-auto small', formOk ? 'text-success' : 'text-danger']"
        >{{ formMsg }}</span>
      </div>
    </div>

    <!-- Connections list -->
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>URL API</th>
            <th>Identifiant</th>
            <th>Proxy hosts</th>
            <th>Statut</th>
            <th>Dernier contact</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="connections.length === 0">
            <td
              colspan="7"
              class="text-center text-muted py-4"
            >
              Aucune connexion NPM configurée.
            </td>
          </tr>
          <tr
            v-for="conn in connections"
            :key="conn.id"
          >
            <td class="fw-medium">
              {{ conn.name }}
            </td>
            <td class="text-muted small">
              {{ conn.api_url }}
            </td>
            <td class="text-muted small">
              {{ conn.identity }}
            </td>
            <td>{{ conn.proxy_host_count }}</td>
            <td>
              <span
                v-if="!conn.enabled"
                class="badge bg-secondary-lt text-secondary"
              >Désactivé</span>
              <span
                v-else-if="conn.last_error"
                class="badge bg-danger-lt text-danger"
                :title="conn.last_error"
              >Erreur</span>
              <span
                v-else-if="conn.last_success_at"
                class="badge bg-success-lt text-success"
              >OK</span>
              <span
                v-else
                class="badge bg-warning-lt text-warning"
              >En attente</span>
            </td>
            <td class="text-muted small">
              <span v-if="conn.last_success_at">{{ formatDate(conn.last_success_at) }}</span>
              <span v-else>—</span>
            </td>
            <td
              v-if="authIsAdmin"
              class="text-end"
            >
              <div class="d-flex gap-1 justify-content-end">
                <!-- Edit -->
                <button
                  class="btn btn-sm btn-outline-secondary"
                  title="Modifier"
                  @click="openEditForm(conn)"
                >
                  <svg
                    class="icon icon-sm"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
                    <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
                  </svg>
                </button>
                <!-- Refresh -->
                <button
                  class="btn btn-sm btn-outline-info"
                  title="Rafraîchir maintenant"
                  @click="refreshNow(conn)"
                >
                  <svg
                    class="icon icon-sm"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <polyline points="1 4 1 10 7 10" />
                    <path d="M3.51 15a9 9 0 1 0 .49-3.86" />
                  </svg>
                </button>
                <!-- Delete -->
                <button
                  class="btn btn-sm btn-outline-danger"
                  title="Supprimer"
                  @click="remove(conn)"
                >
                  <svg
                    class="icon icon-sm"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <polyline points="3 6 5 6 21 6" />
                    <path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6" />
                    <path d="M10 11v6M14 11v6M9 6V4h6v2" />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div
      v-if="listMsg"
      class="card-footer"
    >
      <span :class="['small', listOk ? 'text-success' : 'text-danger']">{{ listMsg }}</span>
    </div>
  </div>

</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { npmApi } from '../../api/npm'
import type { NPMConnection } from '../../types/npm'

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

interface NPMForm {
  name: string
  api_url: string
  identity: string
  secret: string
  enabled: boolean
  poll_interval_sec: number
}

const connections = ref<NPMConnection[]>([])
const showForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testing = ref(false)
const formMsg = ref('')
const formOk = ref(false)
const listMsg = ref('')
const listOk = ref(false)

const emptyForm = (): NPMForm => ({
  name: '',
  api_url: '',
  identity: '',
  secret: '',
  enabled: true,
  poll_interval_sec: 3600,
})

const form = ref<NPMForm>(emptyForm())

async function load(): Promise<void> {
  try {
    const res = await npmApi.listConnections()
    connections.value = res.data.connections ?? []
  } catch {
    // silently ignore
  }
}

function openAddForm(): void {
  editingId.value = null
  form.value = emptyForm()
  formMsg.value = ''
  showForm.value = true
}

function openEditForm(conn: NPMConnection): void {
  editingId.value = conn.id
  form.value = {
    name: conn.name,
    api_url: conn.api_url,
    identity: conn.identity,
    secret: '',
    enabled: conn.enabled ?? true,
    poll_interval_sec: conn.poll_interval_sec ?? 3600,
  }
  formMsg.value = ''
  showForm.value = true
}

function cancelForm(): void {
  showForm.value = false
  formMsg.value = ''
  editingId.value = null
}

async function save(): Promise<void> {
  if (!form.value.name || !form.value.api_url || !form.value.identity) {
    formMsg.value = 'Nom, URL API et identifiant sont obligatoires.'
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
  try {
    if (editingId.value) {
      await npmApi.updateConnection(editingId.value, form.value)
    } else {
      if (!form.value.secret) {
        formMsg.value = 'Le mot de passe est obligatoire à la création.'
        formOk.value = false
        saving.value = false
        return
      }
      await npmApi.createConnection(form.value)
    }
    formMsg.value = editingId.value ? 'Connexion mise à jour.' : 'Connexion créée.'
    formOk.value = true
    await load()
    showForm.value = false
    editingId.value = null
  } catch (e: any) {
    formMsg.value = e?.response?.data?.error || 'Erreur lors de l\'enregistrement.'
    formOk.value = false
  } finally {
    saving.value = false
  }
}

async function testForm(): Promise<void> {
  if (!form.value.api_url || !form.value.identity || !form.value.secret) {
    formMsg.value = 'Renseignez l\'URL, l\'identifiant et le mot de passe pour tester.'
    formOk.value = false
    return
  }
  testing.value = true
  formMsg.value = ''
  try {
    const res = await npmApi.testConnection({
      api_url: form.value.api_url,
      identity: form.value.identity,
      secret: form.value.secret,
    })
    if (res.data.success) {
      formMsg.value = 'Connexion réussie !'
      formOk.value = true
    } else {
      formMsg.value = res.data.error || 'Échec de connexion.'
      formOk.value = false
    }
  } catch (e: any) {
    formMsg.value = e?.response?.data?.error || 'Erreur réseau.'
    formOk.value = false
  } finally {
    testing.value = false
  }
}

async function refreshNow(conn: NPMConnection): Promise<void> {
  try {
    await npmApi.refreshNow(conn.id)
    listMsg.value = `[${conn.name}] Rafraîchissement déclenché.`
    listOk.value = true
    setTimeout(load, 3000)
  } catch (e: any) {
    listMsg.value = e?.response?.data?.error || 'Erreur.'
    listOk.value = false
  }
}

async function remove(conn: NPMConnection): Promise<void> {
  if (!confirm(`Supprimer la connexion NPM « ${conn.name} » ? Les proxy hosts et leurs sondes uptime/SSL associées ne seront PAS supprimés.`)) return
  try {
    await npmApi.deleteConnection(conn.id)
    await load()
    listMsg.value = 'Connexion supprimée.'
    listOk.value = true
  } catch (e: any) {
    listMsg.value = e?.response?.data?.error || 'Erreur lors de la suppression.'
    listOk.value = false
  }
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

onMounted(load)
</script>

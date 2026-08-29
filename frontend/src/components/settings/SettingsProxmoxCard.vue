<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Proxmox VE
      </h3>
      <button
        v-if="authIsAdmin && !showForm"
        type="button"
        class="btn btn-sm btn-primary"
        @click="openAddForm"
      >
        <IconPlus
          :size="16"
          class="icon me-1"
        />
        Ajouter une connexion
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
            placeholder="Mon cluster PVE"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">URL API *</label>
          <input
            v-model="form.api_url"
            type="text"
            class="form-control"
            placeholder="https://pve.example.com:8006/api2/json"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Token ID *</label>
          <input
            v-model="form.token_id"
            type="text"
            class="form-control"
            placeholder="root@pam!supervision"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Token secret {{ editingId ? '(vide = inchangé)' : '*' }}</label>
          <input
            v-model="form.token_secret"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
        <div class="col-md-4">
          <label class="form-label">Intervalle de collecte (s)</label>
          <input
            v-model.number="form.poll_interval_sec"
            type="number"
            class="form-control"
            min="10"
          >
        </div>
        <div class="col-md-4 d-flex align-items-end gap-3">
          <label class="form-check form-switch mb-0">
            <input
              v-model="form.insecure_skip_verify"
              class="form-check-input"
              type="checkbox"
            >
            <span class="form-check-label">Ignorer TLS (self-signed)</span>
          </label>
        </div>
        <div class="col-md-4 d-flex align-items-end gap-3">
          <label class="form-check form-switch mb-0">
            <input
              v-model="form.enabled"
              class="form-check-input"
              type="checkbox"
            >
            <span class="form-check-label">Activé</span>
          </label>
        </div>
        <div class="col-12">
          <label class="form-label">
            Identifiants console PVE (optionnel)
          </label>
          <div class="form-hint mb-2">
            Nécessaires uniquement pour ouvrir une console interactive sur un conteneur LXC — l'API PVE
            n'accepte pas le token pour cette fonctionnalité, un utilisateur PVE avec le rôle
            <code>VM.Console</code> est requis (ex. <code>root@pam</code>).
          </div>
        </div>
        <div class="col-md-6">
          <label
            for="proxmox-pve-username"
            class="form-label"
          >Utilisateur PVE</label>
          <input
            id="proxmox-pve-username"
            v-model="form.pve_username"
            type="text"
            class="form-control"
            placeholder="root@pam"
          >
        </div>
        <div class="col-md-6">
          <label
            for="proxmox-pve-password"
            class="form-label"
          >Mot de passe PVE {{ editingId ? '(vide = inchangé)' : '' }}</label>
          <input
            id="proxmox-pve-password"
            v-model="form.pve_password"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
      </div>
      <div class="mt-3 d-flex align-items-center gap-2">
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? 'Enregistrement...' : (editingId ? 'Mettre à jour' : 'Créer') }}
        </button>
        <button
          type="button"
          class="btn btn-outline-secondary"
          @click="cancelForm"
        >
          Annuler
        </button>
        <button
          type="button"
          class="btn btn-outline-secondary ms-2"
          :disabled="testing"
          @click="testForm"
        >
          {{ testing ? 'Test...' : 'Tester la connexion' }}
        </button>
        <span
          v-if="formMsg"
          :class="['ms-auto small', formWarn ? 'text-warning' : formOk ? 'text-success' : 'text-danger']"
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
            <th>Token ID</th>
            <th>Console</th>
            <th>Nœuds</th>
            <th>Guests</th>
            <th>Statut</th>
            <th>Dernier contact</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && !instances.length">
            <td colspan="9">
              <LoadingSkeleton variant="table" />
            </td>
          </tr>
          <tr v-else-if="instances.length === 0">
            <td colspan="9">
              <EmptyState title="Aucune connexion Proxmox configurée." />
            </td>
          </tr>
          <tr
            v-for="inst in instances"
            :key="inst.id"
          >
            <td class="fw-medium">
              {{ inst.name }}
            </td>
            <td class="text-muted small">
              {{ inst.api_url }}
            </td>
            <td class="text-muted small">
              {{ inst.token_id }}
            </td>
            <td>
              <span
                v-if="inst.console_configured"
                class="badge bg-success-lt text-success"
              >Configurée</span>
              <span
                v-else
                class="badge bg-secondary-lt text-secondary"
              >Non configurée</span>
            </td>
            <td>{{ inst.node_count }}</td>
            <td>{{ inst.guest_count }}</td>
            <td>
              <span
                v-if="!inst.enabled"
                class="badge bg-secondary-lt text-secondary"
              >Désactivé</span>
              <span
                v-else-if="inst.last_error"
                class="badge bg-danger-lt text-danger"
                :title="inst.last_error"
              >Erreur</span>
              <span
                v-else-if="inst.last_success_at"
                class="badge bg-success-lt text-success"
              >OK</span>
              <span
                v-else
                class="badge bg-warning-lt text-warning"
              >En attente</span>
            </td>
            <td class="text-muted small">
              <span v-if="inst.last_success_at">{{ formatDate(inst.last_success_at) }}</span>
              <span v-else>—</span>
            </td>
            <td
              v-if="authIsAdmin"
              class="text-end"
            >
              <div class="d-flex gap-1 justify-content-end">
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  title="Modifier"
                  @click="openEditForm(inst)"
                >
                  <IconPencil
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  title="Tester"
                  @click="testById(inst)"
                >
                  <IconClock
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-primary"
                  title="Collecter maintenant"
                  @click="pollNow(inst)"
                >
                  <IconRefresh
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  title="Supprimer"
                  @click="remove(inst)"
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

    <div
      v-if="listMsg"
      class="card-footer"
    >
      <span :class="['small', listWarn ? 'text-warning' : listOk ? 'text-success' : 'text-danger']">{{ listMsg }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { IconClock, IconPencil, IconPlus, IconRefresh, IconTrash } from '@tabler/icons-vue'
import api from '../../api/index'
import type { ProxmoxConnection, ProxmoxTestResult } from '../../types/proxmox'
import { getApiErrorMessage } from '../../api/client'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'

const { confirm } = useConfirmDialog()

// Use the shared domain type (the settings card only reads a subset of fields).
type ProxmoxInstance = ProxmoxConnection

interface ProxmoxForm {
  name: string
  api_url: string
  token_id: string
  token_secret: string
  insecure_skip_verify: boolean
  enabled: boolean
  poll_interval_sec: number
  pve_username: string
  pve_password: string
}

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

const instances = ref<ProxmoxInstance[]>([])
const loading = ref(false)
const showForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testing = ref(false)
const formMsg = ref('')
const formOk = ref(false)
// Set alongside formOk only by test-result paths (formatTestResult's
// 'warning' tone: API reachable but console misconfigured/unreachable) —
// every other message path (save/create/delete/poll) leaves this false.
const formWarn = ref(false)
const listMsg = ref('')
const listOk = ref(false)
const listWarn = ref(false)

const emptyForm = (): ProxmoxForm => ({
  name: '',
  api_url: '',
  token_id: '',
  token_secret: '',
  insecure_skip_verify: false,
  enabled: true,
  poll_interval_sec: 60,
  pve_username: '',
  pve_password: '',
})

const form = ref<ProxmoxForm>(emptyForm())

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await api.getProxmoxInstances()
    instances.value = res.data
  } catch {
    // silently ignore
  } finally {
    loading.value = false
  }
}

function openAddForm(): void {
  editingId.value = null
  form.value = emptyForm()
  formMsg.value = ''
  formWarn.value = false
  showForm.value = true
}

function openEditForm(inst: ProxmoxInstance): void {
  editingId.value = inst.id
  form.value = {
    name: inst.name,
    api_url: inst.api_url,
    token_id: inst.token_id,
    token_secret: '',
    insecure_skip_verify: inst.insecure_skip_verify ?? false,
    enabled: inst.enabled ?? true,
    poll_interval_sec: inst.poll_interval_sec ?? 60,
    pve_username: inst.pve_username ?? '',
    pve_password: '',
  }
  formMsg.value = ''
  formWarn.value = false
  showForm.value = true
}

function cancelForm(): void {
  showForm.value = false
  formMsg.value = ''
  formWarn.value = false
  editingId.value = null
}

async function save(): Promise<void> {
  if (!form.value.name || !form.value.api_url || !form.value.token_id) {
    formMsg.value = 'Nom, URL API et Token ID sont obligatoires.'
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
  formWarn.value = false
  try {
    if (editingId.value) {
      await api.updateProxmoxInstance(editingId.value, form.value)
    } else {
      if (!form.value.token_secret) {
        formMsg.value = 'Le token secret est obligatoire à la création.'
        formOk.value = false
        saving.value = false
        return
      }
      await api.createProxmoxInstance(form.value)
    }
    formMsg.value = editingId.value ? 'Connexion mise à jour.' : 'Connexion créée.'
    formOk.value = true
    await load()
    showForm.value = false
    editingId.value = null
  } catch (e: unknown) {
    formMsg.value = getApiErrorMessage(e, 'Erreur lors de l\'enregistrement.')
    formOk.value = false
  } finally {
    saving.value = false
  }
}

// Shared by testForm/testById: turns a ProxmoxTestResult into a single
// human message plus its ok/warning/error tone. Console fields are only
// meaningful when console_configured is true; console_ok only proves the
// PVE login itself succeeded — it can't confirm the VM.Console privilege
// without a specific guest to open a console against (see the backend's
// TestConnection doc comment), so that caveat is spelled out rather than
// implied by a green badge.
function formatTestResult(result: ProxmoxTestResult): { message: string, tone: 'success' | 'warning' | 'danger' } {
  if (!result.success) {
    return { message: result.error || 'Échec de connexion.', tone: 'danger' }
  }
  if (!result.console_configured) {
    return { message: 'Connexion API réussie. Console non configurée (identifiants PVE absents).', tone: 'warning' }
  }
  if (result.console_ok) {
    return { message: 'Connexion API réussie, identifiants console valides. Le rôle VM.Console sera vérifié à l\'ouverture d\'une console sur un conteneur.', tone: 'success' }
  }
  return { message: `Connexion API réussie, mais échec de connexion console : ${result.console_error || 'erreur inconnue'}`, tone: 'warning' }
}

async function testForm(): Promise<void> {
  testing.value = true
  formMsg.value = ''
  formWarn.value = false
  try {
    // Editing: test the connection as actually stored server-side, so
    // fields left blank on purpose ("vide = inchangé") are honored instead
    // of being sent empty and failing the test.
    if (editingId.value) {
      const res = await api.testProxmoxInstanceById(editingId.value)
      const { message, tone } = formatTestResult(res.data)
      formMsg.value = message
      formOk.value = tone !== 'danger'
      formWarn.value = tone === 'warning'
      return
    }
    if (!form.value.api_url || !form.value.token_id || !form.value.token_secret) {
      formMsg.value = 'Renseignez l\'URL, le token ID et le secret pour tester.'
      formOk.value = false
      return
    }
    const res = await api.testProxmoxConnection({
      api_url: form.value.api_url,
      token_id: form.value.token_id,
      token_secret: form.value.token_secret,
      insecure_skip_verify: form.value.insecure_skip_verify,
      pve_username: form.value.pve_username,
      pve_password: form.value.pve_password,
    })
    const { message, tone } = formatTestResult(res.data)
    formMsg.value = message
    formOk.value = tone !== 'danger'
    formWarn.value = tone === 'warning'
  } catch (e: unknown) {
    formMsg.value = getApiErrorMessage(e, 'Erreur réseau.')
    formOk.value = false
  } finally {
    testing.value = false
  }
}

async function testById(inst: ProxmoxInstance): Promise<void> {
  listMsg.value = ''
  listWarn.value = false
  try {
    const res = await api.testProxmoxInstanceById(inst.id)
    const { message, tone } = formatTestResult(res.data)
    listMsg.value = `[${inst.name}] ${message}`
    listOk.value = tone !== 'danger'
    listWarn.value = tone === 'warning'
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, 'Erreur réseau.')
    listOk.value = false
  }
}

async function pollNow(inst: ProxmoxInstance): Promise<void> {
  listWarn.value = false
  try {
    await api.pollProxmoxNow(inst.id)
    listMsg.value = `[${inst.name}] Collecte déclenchée.`
    listOk.value = true
    setTimeout(load, 3000)
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, 'Erreur.')
    listOk.value = false
  }
}

async function remove(inst: ProxmoxInstance): Promise<void> {
  const confirmed = await confirm({
    title: 'Supprimer la connexion Proxmox ?',
    message: `Supprimer la connexion Proxmox « ${inst.name} » ? Toutes les données collectées seront effacées.`,
    variant: 'danger',
  })
  if (!confirmed) return
  listWarn.value = false
  try {
    await api.deleteProxmoxInstance(inst.id)
    await load()
    listMsg.value = 'Connexion supprimée.'
    listOk.value = true
  } catch (e: unknown) {
    listMsg.value = getApiErrorMessage(e, 'Erreur lors de la suppression.')
    listOk.value = false
  }
}

function formatDate(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

onMounted(load)
</script>

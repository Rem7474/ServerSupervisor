<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">Proxmox VE</h3>
      <button v-if="authIsAdmin && !showForm" class="btn btn-sm btn-primary" @click="openAddForm">
        + Ajouter une connexion
      </button>
    </div>

    <!-- Add / Edit form -->
    <div v-if="showForm && authIsAdmin" class="card-body border-bottom">
      <div class="row g-3">
        <div class="col-md-6">
          <label class="form-label">Nom *</label>
          <input v-model="form.name" type="text" class="form-control" placeholder="Mon cluster PVE" />
        </div>
        <div class="col-md-6">
          <label class="form-label">URL API *</label>
          <input v-model="form.api_url" type="text" class="form-control" placeholder="https://pve.example.com:8006/api2/json" />
        </div>
        <div class="col-md-6">
          <label class="form-label">Token ID *</label>
          <input v-model="form.token_id" type="text" class="form-control" placeholder="root@pam!supervision" />
        </div>
        <div class="col-md-6">
          <label class="form-label">Token secret {{ editingId ? '(vide = inchangé)' : '*' }}</label>
          <input v-model="form.token_secret" type="password" class="form-control" autocomplete="new-password" />
        </div>
        <div class="col-md-4">
          <label class="form-label">Intervalle de collecte (s)</label>
          <input v-model.number="form.poll_interval_sec" type="number" class="form-control" min="10" />
        </div>
        <div class="col-md-4 d-flex align-items-end gap-3">
          <label class="form-check form-switch mb-0">
            <input v-model="form.insecure_skip_verify" class="form-check-input" type="checkbox" />
            <span class="form-check-label">Ignorer TLS (self-signed)</span>
          </label>
        </div>
        <div class="col-md-4 d-flex align-items-end gap-3">
          <label class="form-check form-switch mb-0">
            <input v-model="form.enabled" class="form-check-input" type="checkbox" />
            <span class="form-check-label">Activé</span>
          </label>
        </div>
      </div>
      <div class="mt-3 d-flex align-items-center gap-2">
        <button class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? 'Enregistrement...' : (editingId ? 'Mettre à jour' : 'Créer') }}
        </button>
        <button class="btn btn-outline-secondary" @click="cancelForm">Annuler</button>
        <button class="btn btn-outline-info ms-2" :disabled="testing" @click="testForm">
          {{ testing ? 'Test...' : 'Tester la connexion' }}
        </button>
        <span v-if="formMsg" :class="['ms-auto small', formOk ? 'text-success' : 'text-danger']">{{ formMsg }}</span>
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
            <th>Nœuds</th>
            <th>Guests</th>
            <th>Statut</th>
            <th>Dernier contact</th>
            <th v-if="authIsAdmin"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="instances.length === 0">
            <td colspan="8" class="text-center text-muted py-4">
              Aucune connexion Proxmox configurée.
            </td>
          </tr>
          <tr v-for="inst in instances" :key="inst.id">
            <td class="fw-medium">{{ inst.name }}</td>
            <td class="text-muted small">{{ inst.api_url }}</td>
            <td class="text-muted small">{{ inst.token_id }}</td>
            <td>{{ inst.node_count }}</td>
            <td>{{ inst.guest_count }}</td>
            <td>
              <span v-if="!inst.enabled" class="badge bg-secondary-lt text-secondary">Désactivé</span>
              <span v-else-if="inst.last_error" class="badge bg-danger-lt text-danger" :title="inst.last_error">Erreur</span>
              <span v-else-if="inst.last_success_at" class="badge bg-success-lt text-success">OK</span>
              <span v-else class="badge bg-warning-lt text-warning">En attente</span>
            </td>
            <td class="text-muted small">
              <span v-if="inst.last_success_at">{{ formatDate(inst.last_success_at) }}</span>
              <span v-else>—</span>
            </td>
            <td v-if="authIsAdmin" class="text-end">
              <div class="d-flex gap-1 justify-content-end">
                <button class="btn btn-sm btn-outline-secondary" @click="openEditForm(inst)" title="Modifier">
                  <svg class="icon icon-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                    <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                  </svg>
                </button>
                <button class="btn btn-sm btn-outline-info" @click="testById(inst)" title="Tester">
                  <svg class="icon icon-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10"/><polyline points="12 8 12 12 14 14"/>
                  </svg>
                </button>
                <button class="btn btn-sm btn-outline-primary" @click="pollNow(inst)" title="Collecter maintenant">
                  <svg class="icon icon-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 .49-3.86"/>
                  </svg>
                </button>
                <button class="btn btn-sm btn-outline-danger" @click="remove(inst)" title="Supprimer">
                  <svg class="icon icon-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>
                    <path d="M10 11v6M14 11v6M9 6V4h6v2"/>
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="listMsg" class="card-footer">
      <span :class="['small', listOk ? 'text-success' : 'text-danger']">{{ listMsg }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../../api/index.js'

const props = defineProps({
  authIsAdmin: { type: Boolean, default: false },
})

const instances = ref([])
const showForm = ref(false)
const editingId = ref(null)
const saving = ref(false)
const testing = ref(false)
const formMsg = ref('')
const formOk = ref(false)
const listMsg = ref('')
const listOk = ref(false)

const emptyForm = () => ({
  name: '',
  api_url: '',
  token_id: '',
  token_secret: '',
  insecure_skip_verify: false,
  enabled: true,
  poll_interval_sec: 60,
})

const form = ref(emptyForm())

async function load() {
  try {
    const res = await api.getProxmoxInstances()
    instances.value = res.data
  } catch {
    // silently ignore
  }
}

function openAddForm() {
  editingId.value = null
  form.value = emptyForm()
  formMsg.value = ''
  showForm.value = true
}

function openEditForm(inst) {
  editingId.value = inst.id
  form.value = {
    name: inst.name,
    api_url: inst.api_url,
    token_id: inst.token_id,
    token_secret: '',
    insecure_skip_verify: inst.insecure_skip_verify,
    enabled: inst.enabled,
    poll_interval_sec: inst.poll_interval_sec,
  }
  formMsg.value = ''
  showForm.value = true
}

function cancelForm() {
  showForm.value = false
  formMsg.value = ''
  editingId.value = null
}

async function save() {
  if (!form.value.name || !form.value.api_url || !form.value.token_id) {
    formMsg.value = 'Nom, URL API et Token ID sont obligatoires.'
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
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
  } catch (e) {
    formMsg.value = e?.response?.data?.error || 'Erreur lors de l\'enregistrement.'
    formOk.value = false
  } finally {
    saving.value = false
  }
}

async function testForm() {
  if (!form.value.api_url || !form.value.token_id || !form.value.token_secret) {
    formMsg.value = 'Renseignez l\'URL, le token ID et le secret pour tester.'
    formOk.value = false
    return
  }
  testing.value = true
  formMsg.value = ''
  try {
    const res = await api.testProxmoxConnection({
      api_url: form.value.api_url,
      token_id: form.value.token_id,
      token_secret: form.value.token_secret,
      insecure_skip_verify: form.value.insecure_skip_verify,
    })
    if (res.data.success) {
      formMsg.value = 'Connexion réussie !'
      formOk.value = true
    } else {
      formMsg.value = res.data.error || 'Échec de connexion.'
      formOk.value = false
    }
  } catch (e) {
    formMsg.value = e?.response?.data?.error || 'Erreur réseau.'
    formOk.value = false
  } finally {
    testing.value = false
  }
}

async function testById(inst) {
  listMsg.value = ''
  try {
    const res = await api.testProxmoxInstanceById(inst.id)
    if (res.data.success) {
      listMsg.value = `[${inst.name}] Connexion OK.`
      listOk.value = true
    } else {
      listMsg.value = `[${inst.name}] ${res.data.error}`
      listOk.value = false
    }
  } catch (e) {
    listMsg.value = e?.response?.data?.error || 'Erreur réseau.'
    listOk.value = false
  }
}

async function pollNow(inst) {
  try {
    await api.pollProxmoxNow(inst.id)
    listMsg.value = `[${inst.name}] Collecte déclenchée.`
    listOk.value = true
    setTimeout(load, 3000)
  } catch (e) {
    listMsg.value = e?.response?.data?.error || 'Erreur.'
    listOk.value = false
  }
}

async function remove(inst) {
  if (!confirm(`Supprimer la connexion Proxmox « ${inst.name} » ? Toutes les données collectées seront effacées.`)) return
  try {
    await api.deleteProxmoxInstance(inst.id)
    await load()
    listMsg.value = 'Connexion supprimée.'
    listOk.value = true
  } catch (e) {
    listMsg.value = e?.response?.data?.error || 'Erreur lors de la suppression.'
    listOk.value = false
  }
}

function formatDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

onMounted(load)
</script>

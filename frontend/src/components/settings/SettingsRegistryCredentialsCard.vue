<template>
  <div class="card mb-4">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Registres privés
      </h3>
      <button
        v-if="authIsAdmin && !showForm"
        class="btn btn-sm btn-primary"
        @click="openAddForm"
      >
        + Ajouter un identifiant
      </button>
    </div>

    <div class="card-body border-bottom py-2">
      <p class="text-muted small mb-0">
        Identifiants utilisés par les trackers Docker pour interroger des images sur des
        registres privés (GHCR, Harbor, registres d'entreprise…). Le mot de passe n'est
        jamais réaffiché.
      </p>
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
            placeholder="GHCR mon-org"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Hôte du registre *</label>
          <input
            v-model="form.registry_host"
            type="text"
            class="form-control"
            placeholder="ghcr.io"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Utilisateur *</label>
          <input
            v-model="form.username"
            type="text"
            class="form-control"
            autocomplete="off"
          >
        </div>
        <div class="col-md-6">
          <label class="form-label">Mot de passe / token {{ editingId ? '(vide = inchangé)' : '*' }}</label>
          <input
            v-model="form.password"
            type="password"
            class="form-control"
            autocomplete="new-password"
          >
        </div>
      </div>
      <div class="mt-3 d-flex align-items-center gap-2">
        <button
          class="btn btn-primary"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? 'Enregistrement...' : (editingId ? 'Mettre à jour' : 'Créer') }}
        </button>
        <button
          class="btn btn-outline-secondary"
          @click="cancelForm"
        >
          Annuler
        </button>
        <span
          v-if="formMsg"
          :class="['ms-auto small', formOk ? 'text-success' : 'text-danger']"
        >{{ formMsg }}</span>
      </div>
    </div>

    <!-- List -->
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Nom</th>
            <th>Hôte</th>
            <th>Utilisateur</th>
            <th v-if="authIsAdmin" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="credentials.length === 0">
            <td
              colspan="4"
              class="text-center text-muted py-4"
            >
              Aucun identifiant de registre configuré.
            </td>
          </tr>
          <tr
            v-for="cred in credentials"
            :key="cred.id"
          >
            <td class="fw-medium">
              {{ cred.name }}
            </td>
            <td class="text-muted small">
              {{ cred.registry_host }}
            </td>
            <td class="text-muted small">
              {{ cred.username }}
            </td>
            <td
              v-if="authIsAdmin"
              class="text-end"
            >
              <div class="d-flex gap-1 justify-content-end">
                <button
                  class="btn btn-sm btn-outline-secondary"
                  title="Modifier"
                  @click="openEditForm(cred)"
                >
                  Modifier
                </button>
                <button
                  class="btn btn-sm btn-outline-danger"
                  title="Supprimer"
                  @click="remove(cred)"
                >
                  Supprimer
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
import api from '../../api/index'

interface Credential {
  id: string
  name: string
  registry_host: string
  username: string
}

interface CredentialForm {
  name: string
  registry_host: string
  username: string
  password: string
}

withDefaults(defineProps<{
  authIsAdmin?: boolean
}>(), {
  authIsAdmin: false,
})

const credentials = ref<Credential[]>([])
const showForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const formMsg = ref('')
const formOk = ref(false)
const listMsg = ref('')
const listOk = ref(false)

const emptyForm = (): CredentialForm => ({
  name: '',
  registry_host: '',
  username: '',
  password: '',
})

const form = ref<CredentialForm>(emptyForm())

async function load(): Promise<void> {
  try {
    const res = await api.getRegistryCredentials()
    credentials.value = Array.isArray(res.data?.credentials) ? res.data.credentials : []
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

function openEditForm(cred: Credential): void {
  editingId.value = cred.id
  form.value = {
    name: cred.name,
    registry_host: cred.registry_host,
    username: cred.username,
    password: '',
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
  if (!form.value.name || !form.value.registry_host || !form.value.username) {
    formMsg.value = 'Nom, hôte et utilisateur sont obligatoires.'
    formOk.value = false
    return
  }
  if (!editingId.value && !form.value.password) {
    formMsg.value = 'Le mot de passe est obligatoire à la création.'
    formOk.value = false
    return
  }
  saving.value = true
  formMsg.value = ''
  try {
    if (editingId.value) {
      await api.updateRegistryCredential(editingId.value, form.value)
    } else {
      await api.createRegistryCredential(form.value)
    }
    formMsg.value = editingId.value ? 'Identifiant mis à jour.' : 'Identifiant créé.'
    formOk.value = true
    await load()
    showForm.value = false
    editingId.value = null
  } catch (e: any) {
    formMsg.value = e?.response?.data?.error || "Erreur lors de l'enregistrement."
    formOk.value = false
  } finally {
    saving.value = false
  }
}

async function remove(cred: Credential): Promise<void> {
  if (!confirm(`Supprimer l'identifiant « ${cred.name} » ? Les trackers qui l'utilisent repasseront en accès public.`)) return
  try {
    await api.deleteRegistryCredential(cred.id)
    await load()
    listMsg.value = 'Identifiant supprimé.'
    listOk.value = true
  } catch (e: any) {
    listMsg.value = e?.response?.data?.error || 'Erreur lors de la suppression.'
    listOk.value = false
  }
}

onMounted(load)
</script>

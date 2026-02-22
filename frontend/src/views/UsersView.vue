<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Utilisateurs</h2>
        <div class="text-secondary">Gestion des roles (admin / operator / viewer)</div>
      </div>
      <button class="btn btn-outline-secondary" @click="fetchUsers" :disabled="loading">Actualiser</button>
    </div>

    <!-- Create User Form -->
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Ajouter un utilisateur</h3>
      </div>
      <div class="card-body">
        <form @submit.prevent="createUser">
          <div class="row g-3">
            <div class="col-md-4">
              <label class="form-label">Nom d'utilisateur</label>
              <input 
                v-model="newUserForm.username"
                type="text" 
                class="form-control"
                placeholder="john_doe"
                required
                :disabled="creatingUser"
              />
            </div>
            <div class="col-md-4">
              <label class="form-label">Mot de passe</label>
              <input 
                v-model="newUserForm.password"
                type="password" 
                class="form-control"
                placeholder="••••••••"
                required
                :disabled="creatingUser"
              />
            </div>
            <div class="col-md-3">
              <label class="form-label">Role</label>
              <select v-model="newUserForm.role" class="form-select" :disabled="creatingUser">
                <option value="viewer">viewer</option>
                <option value="operator">operator</option>
                <option value="admin">admin</option>
              </select>
            </div>
            <div class="col-md-1 d-flex align-items-end">
              <button type="submit" class="btn btn-primary w-100" :disabled="creatingUser">
                {{ creatingUser ? 'Création...' : 'Ajouter' }}
              </button>
            </div>
          </div>
        </form>
        <div v-if="createMessage" :class="['alert mt-3 mb-0', createSuccess ? 'alert-success' : 'alert-danger']">
          {{ createMessage }}
        </div>
      </div>
    </div>

    <!-- Users List -->
    <div class="card">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Utilisateur</th>
              <th>Role</th>
              <th>Création</th>
              <th style="width: 200px;"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td class="fw-semibold">
                {{ user.username }}
                <span v-if="user.username === auth.username" class="badge bg-blue-lt text-blue ms-2">Vous</span>
              </td>
              <td>
                <select 
                  v-model="user.role" 
                  class="form-select form-select-sm" 
                  :disabled="saving || user.username === auth.username"
                  @change="saveRole(user)"
                  :title="user.username === auth.username ? 'Impossible de modifier votre propre rôle' : ''"
                >
                  <option value="viewer">viewer</option>
                  <option value="operator">operator</option>
                  <option value="admin">admin</option>
                </select>
              </td>
              <td class="text-secondary small">{{ formatDate(user.created_at) }}</td>
              <td class="text-end">
                <button 
                  class="btn btn-sm btn-danger"
                  @click="deleteUser(user)"
                  :disabled="saving || user.username === auth.username || (isLastAdmin(user.id) && user.role === 'admin')"
                  :title="getDeleteButtonTitle(user)"
                >
                  Supprimer
                </button>
              </td>
            </tr>
            <tr v-if="!users.length && !loading">
              <td colspan="4" class="text-center text-secondary py-4">Aucun utilisateur</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'

const auth = useAuthStore()
const users = ref([])
const loading = ref(false)
const saving = ref(false)
const creatingUser = ref(false)

const newUserForm = ref({
  username: '',
  password: '',
  role: 'viewer',
})

const createMessage = ref('')
const createSuccess = ref(false)

dayjs.extend(utc)

function formatDate(date) {
  if (!date) return '-'
  return dayjs.utc(date).local().format('YYYY-MM-DD HH:mm')
}

const adminCount = computed(() => users.value.filter(u => u.role === 'admin').length)

function isLastAdmin(userId) {
  const user = users.value.find(u => u.id === userId)
  return user && user.role === 'admin' && adminCount.value === 1
}

function getDeleteButtonTitle(user) {
  if (user.username === auth.username) return 'Impossible de supprimer votre propre compte'
  if (isLastAdmin(user.id)) return 'Impossible de supprimer le dernier admin'
  return 'Supprimer cet utilisateur'
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = await apiClient.getUsers()
    users.value = res.data || []
  } catch (e) {
    console.error('Erreur lors du chargement des utilisateurs:', e)
    users.value = []
  } finally {
    loading.value = false
  }
}

async function createUser() {
  if (!newUserForm.value.username || !newUserForm.value.password) {
    createMessage.value = 'Veuillez remplir tous les champs'
    createSuccess.value = false
    return
  }

  // Check if username already exists
  if (users.value.find(u => u.username === newUserForm.value.username)) {
    createMessage.value = 'Ce nom d\'utilisateur existe déjà'
    createSuccess.value = false
    return
  }

  creatingUser.value = true
  createMessage.value = ''
  try {
    await apiClient.createUser(
      newUserForm.value.username,
      newUserForm.value.password,
      newUserForm.value.role
    )
    createSuccess.value = true
    createMessage.value = 'Utilisateur créé avec succès'
    newUserForm.value = { username: '', password: '', role: 'viewer' }
    await fetchUsers()
  } catch (e) {
    createSuccess.value = false
    createMessage.value = e.response?.data?.error || 'Erreur lors de la création'
  } finally {
    creatingUser.value = false
  }
}

async function saveRole(user) {
  saving.value = true
  try {
    await apiClient.updateUserRole(user.id, user.role)
  } catch (e) {
    console.error('Erreur lors de la mise à jour du rôle:', e)
    // Reload to revert changes
    await fetchUsers()
  } finally {
    saving.value = false
  }
}

async function deleteUser(user) {
  if (!confirm(`Êtes-vous sûr de vouloir supprimer l'utilisateur "${user.username}" ?`)) {
    return
  }

  saving.value = true
  try {
    await apiClient.deleteUser(user.id)
    await fetchUsers()
  } catch (e) {
    console.error('Erreur lors de la suppression:', e)
  } finally {
    saving.value = false
  }
}

onMounted(fetchUsers)
</script>

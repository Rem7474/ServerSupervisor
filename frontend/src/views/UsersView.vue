<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Utilisateurs</h2>
        <div class="text-secondary">Gestion des roles (admin / operator / viewer)</div>
      </div>
      <button class="btn btn-outline-secondary" @click="fetchUsers" :disabled="loading">Actualiser</button>
    </div>

    <div class="card">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Utilisateur</th>
              <th>Role</th>
              <th>Creation</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td class="fw-semibold">{{ user.username }}</td>
              <td>
                <select v-model="user.role" class="form-select form-select-sm" :disabled="saving">
                  <option value="admin">admin</option>
                  <option value="operator">operator</option>
                  <option value="viewer">viewer</option>
                </select>
              </td>
              <td class="text-secondary small">{{ formatDate(user.created_at) }}</td>
              <td class="text-end">
                <button class="btn btn-sm btn-primary" @click="saveRole(user)" :disabled="saving">
                  Enregistrer
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
import { ref, onMounted } from 'vue'
import apiClient from '../api'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'

const users = ref([])
const loading = ref(false)
const saving = ref(false)

dayjs.extend(utc)

function formatDate(date) {
  if (!date) return '-'
  return dayjs.utc(date).local().format('YYYY-MM-DD HH:mm')
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = await apiClient.getUsers()
    users.value = res.data || []
  } catch (e) {
    users.value = []
  } finally {
    loading.value = false
  }
}

async function saveRole(user) {
  saving.value = true
  try {
    await apiClient.updateUserRole(user.id, user.role)
  } catch (e) {
    // ignore
  } finally {
    saving.value = false
  }
}

onMounted(fetchUsers)
</script>

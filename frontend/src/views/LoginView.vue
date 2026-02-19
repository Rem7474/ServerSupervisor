<template>
  <div class="page page-center">
    <div class="container container-tight py-4">
      <div class="text-center mb-4">
        <span class="h1">ServerSupervisor</span>
        <div class="text-secondary">Connexion au dashboard</div>
      </div>

      <form class="card card-md" @submit.prevent="handleLogin">
        <div class="card-body">
          <h2 class="card-title text-center mb-4">Se connecter</h2>
          <div class="mb-3">
            <label class="form-label">Utilisateur</label>
            <input v-model="username" type="text" class="form-control" placeholder="admin" required />
          </div>
          <div class="mb-3">
            <label class="form-label">Mot de passe</label>
            <input v-model="password" type="password" class="form-control" placeholder="••••••••" required />
          </div>

          <div v-if="error" class="alert alert-danger" role="alert">
            {{ error }}
          </div>

          <div class="form-footer">
            <button type="submit" class="btn btn-primary w-100" :disabled="loading">
              {{ loading ? 'Connexion...' : 'Se connecter' }}
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.login(username.value, password.value)
    auth.setAuth(data, username.value)
    router.push('/')
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur de connexion'
  } finally {
    loading.value = false
  }
}
</script>

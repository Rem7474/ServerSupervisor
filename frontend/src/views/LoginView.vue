<template>
  <div class="min-h-screen flex items-center justify-center bg-dark-950">
    <div class="card w-full max-w-md">
      <div class="text-center mb-8">
        <h1 class="text-2xl font-bold text-primary-400">ServerSupervisor</h1>
        <p class="text-gray-400 mt-2">Connexion au dashboard</p>
      </div>

      <form @submit.prevent="handleLogin" class="space-y-6">
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">Utilisateur</label>
          <input v-model="username" type="text" class="input-field" placeholder="admin" required />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">Mot de passe</label>
          <input v-model="password" type="password" class="input-field" placeholder="••••••••" required />
        </div>

        <div v-if="error" class="bg-red-500/10 border border-red-500/30 rounded-lg p-3 text-red-400 text-sm">
          {{ error }}
        </div>

        <button type="submit" class="btn-primary w-full" :disabled="loading">
          {{ loading ? 'Connexion...' : 'Se connecter' }}
        </button>
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

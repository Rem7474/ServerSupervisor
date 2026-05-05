<template>
  <div class="page page-center">
    <div class="container container-tight py-4">
      <div class="text-center mb-4">
        <span class="h1">ServerSupervisor</span>
        <div class="text-secondary">
          Connexion au dashboard
        </div>
      </div>

      <form
        class="card card-md"
        @submit.prevent="handleLogin"
      >
        <div class="card-body">
          <h2 class="card-title text-center mb-4">
            Se connecter
          </h2>
          <div class="mb-3">
            <label class="form-label">Utilisateur</label>
            <input
              ref="usernameInput"
              v-model="username"
              type="text"
              class="form-control"
              placeholder="admin"
              name="username"
              autocomplete="username"
              required
              :disabled="loading || needsMFA"
            >
          </div>
          <div class="mb-3">
            <label class="form-label">Mot de passe</label>
            <input
              v-model="password"
              type="password"
              class="form-control"
              placeholder="••••••••"
              name="password"
              autocomplete="current-password"
              required
              :disabled="loading || needsMFA"
            >
          </div>

          <Transition name="slide-down">
            <div
              v-if="needsMFA"
              class="mb-3"
            >
              <label class="form-label">Code TOTP</label>
              <input
                ref="totpInput"
                v-model="totpCode"
                type="text"
                class="form-control"
                placeholder="123456"
                inputmode="numeric"
                maxlength="6"
                required
              >
              <div class="text-secondary small mt-1">
                Entrez le code de votre application d'authentification.
              </div>
            </div>
          </Transition>

          <div
            v-if="error"
            class="alert alert-danger"
            role="alert"
          >
            {{ error }}
          </div>

          <div class="form-footer">
            <button
              type="submit"
              class="btn btn-primary w-100"
              :disabled="loading"
            >
              {{ loading ? 'Connexion...' : (needsMFA ? 'Vérifier le code' : 'Se connecter') }}
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const needsMFA = ref(false)
const totpCode = ref('')
const usernameInput = ref(null)
const totpInput = ref(null)

onMounted(() => {
  usernameInput.value?.focus()
})

watch(needsMFA, async (val) => {
  if (val) {
    await nextTick()
    totpInput.value?.focus()
  }
})

async function handleLogin() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.login(username.value, password.value, needsMFA.value ? totpCode.value : '')

    if (data?.require_mfa) {
      needsMFA.value = true
      totpCode.value = ''
      return
    }

    if (data?.token) {
      auth.setAuth(data, username.value)
      if (data.must_change_password) {
        router.push('/account')
      } else {
        router.push('/')
      }
    } else {
      error.value = 'Réponse de connexion invalide.'
    }
  } catch (e) {
    if (needsMFA.value) {
      totpCode.value = ''
      nextTick(() => totpInput.value?.focus())
      error.value = e.response?.data?.error || 'Code invalide ou expiré — générez un nouveau code.'
    } else {
      error.value = e.response?.data?.error || 'Erreur de connexion'
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.slide-down-enter-active,
.slide-down-leave-active {
  transition: all 0.25s ease;
}
.slide-down-enter-from,
.slide-down-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>

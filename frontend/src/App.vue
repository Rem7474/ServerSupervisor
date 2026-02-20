<template>
  <div class="page">
    <!-- Sidebar + Main -->
    <div v-if="auth.isAuthenticated">
      <header class="navbar navbar-expand-md navbar-dark">
        <div class="container-xl">
          <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbar-menu">
            <span class="navbar-toggler-icon"></span>
          </button>
          <router-link to="/" class="navbar-brand navbar-brand-autodark">
            <svg class="icon me-2" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
            </svg>
            ServerSupervisor
          </router-link>

          <div class="collapse navbar-collapse" id="navbar-menu">
            <ul class="navbar-nav">
              <li class="nav-item">
                <router-link to="/" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Dashboard</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/docker" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Docker</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/apt" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">APT Updates</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/repos" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Versions & Repos</span>
                </router-link>
              </li>
              <li v-if="auth.isAdmin" class="nav-item">
                <router-link to="/audit" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6M5 7h14a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V9a2 2 0 012-2z"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Audit</span>
                </router-link>
              </li>
              <li v-if="auth.isAdmin" class="nav-item">
                <router-link to="/users" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a4 4 0 00-4-4h-1" />
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 20H4v-2a4 4 0 014-4h1" />
                      <circle cx="9" cy="7" r="4" />
                      <circle cx="17" cy="9" r="3" />
                    </svg>
                  </span>
                  <span class="nav-link-title">Utilisateurs</span>
                </router-link>
              </li>
            </ul>

            <div class="ms-auto d-flex align-items-center position-relative">
              <button class="btn btn-outline-secondary d-flex align-items-center" @click="toggleUserMenu">
                <span class="avatar avatar-sm bg-secondary-lt me-2">
                  {{ auth.username?.slice(0, 2).toUpperCase() }}
                </span>
                <span class="me-2">{{ auth.username }}</span>
                <span class="caret"></span>
              </button>

              <div v-if="userMenuOpen" class="dropdown-menu dropdown-menu-end show mt-2">
                <div class="dropdown-header">Compte</div>
                <div class="dropdown-item text-secondary small">Role: {{ auth.role || 'inconnu' }}</div>
                <button class="dropdown-item" @click="openChangePassword">
                  Changer le mot de passe
                </button>
                <router-link to="/security" class="dropdown-item" @click="userMenuOpen = false">
                  Securite (MFA)
                </router-link>
                <div class="dropdown-divider"></div>
                <button class="dropdown-item text-danger" @click="handleLogout">Deconnexion</button>
              </div>
            </div>
          </div>
        </div>
      </header>

      <div class="page-wrapper">
        <div class="page-body">
          <div class="container-xl">
            <router-view />
          </div>
        </div>
      </div>

      <div v-if="showPasswordModal" class="modal modal-blur fade show" tabindex="-1" role="dialog" aria-modal="true" style="display: block;">
        <div class="modal-dialog modal-sm modal-dialog-centered" role="document">
          <div class="modal-content">
            <form @submit.prevent="submitChangePassword">
              <div class="modal-header">
                <h3 class="modal-title">Changer le mot de passe</h3>
                <button type="button" class="btn-close" @click="closeChangePassword" aria-label="Fermer"></button>
              </div>
              <div class="modal-body">
                <div class="mb-3">
                  <label class="form-label">Mot de passe actuel</label>
                  <input v-model="passwordForm.current" type="password" class="form-control" required />
                </div>
                <div class="mb-3">
                  <label class="form-label">Nouveau mot de passe</label>
                  <input v-model="passwordForm.next" type="password" class="form-control" required />
                </div>
                <div class="mb-3">
                  <label class="form-label">Confirmer le nouveau mot de passe</label>
                  <input v-model="passwordForm.confirm" type="password" class="form-control" required />
                </div>

                <div v-if="passwordError" class="alert alert-danger" role="alert">
                  {{ passwordError }}
                </div>
                <div v-if="passwordSuccess" class="alert alert-success" role="alert">
                  {{ passwordSuccess }}
                </div>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-outline-secondary" @click="closeChangePassword" :disabled="passwordLoading">Annuler</button>
                <button type="submit" class="btn btn-primary" :disabled="passwordLoading">
                  {{ passwordLoading ? 'Enregistrement...' : 'Mettre a jour' }}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
      <div v-if="showPasswordModal" class="modal-backdrop fade show" @click="closeChangePassword"></div>
    </div>

    <!-- Login page (no sidebar) -->
    <router-view v-else />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'
import apiClient from './api'

const auth = useAuthStore()
const router = useRouter()
const userMenuOpen = ref(false)
const showPasswordModal = ref(false)
const passwordForm = ref({ current: '', next: '', confirm: '' })
const passwordError = ref('')
const passwordSuccess = ref('')
const passwordLoading = ref(false)

function handleLogout() {
  userMenuOpen.value = false
  auth.logout()
  router.push('/login')
}

function toggleUserMenu() {
  userMenuOpen.value = !userMenuOpen.value
}

function openChangePassword() {
  passwordForm.value = { current: '', next: '', confirm: '' }
  passwordError.value = ''
  passwordSuccess.value = ''
  showPasswordModal.value = true
  userMenuOpen.value = false
}

function closeChangePassword() {
  showPasswordModal.value = false
}

async function submitChangePassword() {
  passwordError.value = ''
  passwordSuccess.value = ''

  if (passwordForm.value.next.length < 8) {
    passwordError.value = 'Le mot de passe doit faire au moins 8 caracteres.'
    return
  }
  if (passwordForm.value.next !== passwordForm.value.confirm) {
    passwordError.value = 'La confirmation ne correspond pas.'
    return
  }

  passwordLoading.value = true
  try {
    await apiClient.changePassword(passwordForm.value.current, passwordForm.value.next)
    passwordSuccess.value = 'Mot de passe mis a jour.'
    passwordForm.value = { current: '', next: '', confirm: '' }
  } catch (e) {
    passwordError.value = e.response?.data?.error || 'Erreur lors de la mise a jour.'
  } finally {
    passwordLoading.value = false
  }
}
</script>

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
                <router-link to="/network" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <circle cx="12" cy="12" r="2.5" stroke-width="2"/>
                      <circle cx="5" cy="5" r="2" stroke-width="2"/>
                      <circle cx="19" cy="5" r="2" stroke-width="2"/>
                      <circle cx="12" cy="20" r="2" stroke-width="2"/>
                      <line x1="6.5" y1="6.5" x2="10.5" y2="10.5" stroke-width="1.8" stroke-linecap="round"/>
                      <line x1="17.5" y1="6.5" x2="13.5" y2="10.5" stroke-width="1.8" stroke-linecap="round"/>
                      <line x1="12" y1="14.5" x2="12" y2="18" stroke-width="1.8" stroke-linecap="round"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Network</span>
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
                <router-link to="/alerts" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Alertes</span>
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
              <li class="nav-item">
                <router-link to="/settings" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <svg class="icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                    </svg>
                  </span>
                  <span class="nav-link-title">Paramètres</span>
                </router-link>
              </li>
            </ul>

            <div class="ms-auto d-flex align-items-center position-relative user-menu" ref="userMenuRef">
              <button class="btn btn-outline-secondary d-flex align-items-center" @click="toggleUserMenu">
                <span class="avatar avatar-sm bg-secondary-lt me-2">
                  {{ auth.username?.slice(0, 2).toUpperCase() }}
                </span>
                <span class="me-2">{{ auth.username }}</span>
                <span class="caret"></span>
              </button>

              <div v-if="userMenuOpen" class="dropdown-menu dropdown-menu-end show user-dropdown">
                <div class="dropdown-header">Compte</div>
                <div class="dropdown-item text-secondary small">Role: {{ auth.role || 'inconnu' }}</div>
                <router-link to="/account" class="dropdown-item" @click="userMenuOpen = false">
                  Mon compte
                </router-link>
                <div class="dropdown-divider"></div>
                <button class="dropdown-item text-danger" @click="handleLogout">Déconnexion</button>
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

      <!-- Global Confirm Dialog -->
      <ConfirmDialog />
    </div>

    <!-- Login page (no sidebar) -->
    <router-view v-else />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'
import ConfirmDialog from './components/ConfirmDialog.vue'

const auth = useAuthStore()
const router = useRouter()
const userMenuOpen = ref(false)
const userMenuRef = ref(null)

function handleLogout() {
  userMenuOpen.value = false
  auth.logout()
  router.push('/login')
}

function toggleUserMenu() {
  userMenuOpen.value = !userMenuOpen.value
}

function handleOutsideClick(event) {
  if (!userMenuOpen.value) return
  const el = userMenuRef.value
  if (el && el.contains(event.target)) return
  userMenuOpen.value = false
}

onMounted(() => {
  document.addEventListener('click', handleOutsideClick, true)
})

onUnmounted(() => {
  document.removeEventListener('click', handleOutsideClick, true)
})
</script>

<style scoped>
.user-menu {
  z-index: 2000;
}

.user-dropdown {
  min-width: 240px;
  padding: 8px 0;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.25);
  position: absolute;
  top: calc(100% + 10px);
  right: 0;
  margin: 0;
}

.user-dropdown::before {
  content: '';
  position: absolute;
  top: -6px;
  right: 14px;
  width: 12px;
  height: 12px;
  background: inherit;
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  transform: rotate(45deg);
}

</style>

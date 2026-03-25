<template>
  <div class="page">
    <!-- Skip navigation link for keyboard/screen reader users -->
    <a href="#main-content" class="skip-link visually-hidden-focusable">Aller au contenu principal</a>

    <!-- Sidebar + Main -->
    <div v-if="auth.isAuthenticated">
      <header class="navbar navbar-expand-md navbar-dark">
        <div class="container-xl">
          <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbar-menu" aria-label="Ouvrir le menu de navigation" aria-controls="navbar-menu" aria-expanded="false">
            <span class="navbar-toggler-icon"></span>
          </button>
          <router-link to="/" class="navbar-brand navbar-brand-autodark">
            <AppIcon name="brand" css-class="icon me-2" />
            ServerSupervisor
          </router-link>

          <div class="collapse navbar-collapse" id="navbar-menu">
            <ul class="navbar-nav">
              <!-- Éléments principaux -->
              <li class="nav-item">
                <router-link to="/" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <AppIcon name="dashboard" />
                  </span>
                  <span class="nav-link-title">Dashboard</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/docker" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <AppIcon name="docker" />
                  </span>
                  <span class="nav-link-title">Docker</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/apt" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <AppIcon name="updates" />
                  </span>
                  <span class="nav-link-title">Mises à jour</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/proxmox" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <AppIcon name="proxmox" />
                  </span>
                  <span class="nav-link-title">Proxmox</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link to="/alerts" class="nav-link" active-class="active">
                  <span class="nav-link-icon">
                    <AppIcon name="alerts" />
                  </span>
                  <span class="nav-link-title">Alertes</span>
                </router-link>
              </li>

              <!-- Dropdown "Plus" — éléments secondaires -->
              <li class="nav-item dropdown" :class="{ active: isSecondaryActive }">
                <a class="nav-link dropdown-toggle" href="#" role="button" @click.prevent="toggleSecondaryMenu" :aria-expanded="secondaryMenuOpen" aria-label="Plus d'options" aria-haspopup="menu">
                  <span class="nav-link-icon">
                    <AppIcon name="more" />
                  </span>
                  <span class="nav-link-title">Plus</span>
                </a>
                <div class="dropdown-menu" :class="{ show: secondaryMenuOpen }" role="menu">
                  <router-link v-if="auth.isAdmin" to="/security" class="dropdown-item" role="menuitem" @click="secondaryMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3l7 4v5c0 5-3.5 8.5-7 9-3.5-.5-7-4-7-9V7l7-4z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.5 12.5l2 2 3-3"/></svg>
                    Sécurité hôtes
                  </router-link>
                  <router-link to="/scheduled-tasks" class="dropdown-item" role="menuitem" @click="secondaryMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                    Tâches planifiées
                  </router-link>
                  <router-link to="/network" class="dropdown-item" role="menuitem" @click="secondaryMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="12" r="2.5" stroke-width="2"/><circle cx="5" cy="5" r="2" stroke-width="2"/><circle cx="19" cy="5" r="2" stroke-width="2"/><circle cx="12" cy="20" r="2" stroke-width="2"/><line x1="6.5" y1="6.5" x2="10.5" y2="10.5" stroke-width="1.8" stroke-linecap="round"/><line x1="17.5" y1="6.5" x2="13.5" y2="10.5" stroke-width="1.8" stroke-linecap="round"/><line x1="12" y1="14.5" x2="12" y2="18" stroke-width="1.8" stroke-linecap="round"/></svg>
                    Réseau
                  </router-link>
                  <router-link to="/settings" class="dropdown-item" role="menuitem" @click="secondaryMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>
                    Paramètres
                  </router-link>
                </div>
              </li>

              <!-- Dropdown "Administration" — admin uniquement -->
              <li v-if="auth.isAdmin" class="nav-item dropdown" :class="{ active: isAdminActive }">
                <a class="nav-link dropdown-toggle" href="#" role="button" @click.prevent="toggleAdminMenu" :aria-expanded="adminMenuOpen" aria-label="Options administrateur" aria-haspopup="menu">
                  <span class="nav-link-icon">
                    <AppIcon name="admin" />
                  </span>
                  <span class="nav-link-title">Admin</span>
                </a>
                <div class="dropdown-menu" :class="{ show: adminMenuOpen }" role="menu">
                  <router-link to="/git-webhooks" class="dropdown-item" role="menuitem" @click="adminMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="5" r="3"/><circle cx="5" cy="19" r="3"/><circle cx="19" cy="19" r="3"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"/></svg>
                    Git / Automatisation
                  </router-link>
                  <router-link to="/audit" class="dropdown-item" role="menuitem" @click="adminMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6M5 7h14a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V9a2 2 0 012-2z"/></svg>
                    Audit
                  </router-link>
                  <router-link to="/users" class="dropdown-item" role="menuitem" @click="adminMenuOpen = false">
                    <svg class="icon icon-sm me-2" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a4 4 0 00-4-4h-1"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 20H4v-2a4 4 0 014-4h1"/><circle cx="9" cy="7" r="4"/><circle cx="17" cy="9" r="3"/></svg>
                    Utilisateurs
                  </router-link>
                </div>
              </li>
            </ul>

            <div class="ms-auto d-flex align-items-center gap-2">
            <NotificationBell />
            <div class="position-relative user-menu" ref="userMenuRef">
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
            </div><!-- end ms-auto wrapper -->
          </div>
        </div>
      </header>

      <!-- Offline / server-unreachable banner -->
      <div v-if="!isOnline" class="alert alert-warning alert-dismissible mb-0 rounded-0 border-0 border-bottom" role="alert" style="position:sticky;top:0;z-index:1040;">
        <div class="container-xl d-flex align-items-center gap-2">
          <AppIcon name="warning" :size="20" css-class="icon flex-shrink-0" />
          <span>Connexion au serveur perdue — les données affichées peuvent être obsolètes.</span>
        </div>
      </div>

      <div v-if="httpError" class="alert alert-danger alert-dismissible mb-0 rounded-0 border-0 border-bottom" role="alert" style="position:sticky;top:0;z-index:1039;">
        <div class="container-xl d-flex align-items-center justify-content-between gap-3">
          <span>{{ httpError }}</span>
          <button type="button" class="btn-close" aria-label="Fermer" @click="httpError = ''"></button>
        </div>
      </div>

      <div class="page-wrapper">
        <div id="main-content" class="page-body">
          <div class="container-xl">
            <ErrorBoundary>
              <router-view />
            </ErrorBoundary>
          </div>
        </div>
        <AppFooter />
      </div>

      <!-- Global Confirm Dialog -->
      <ConfirmDialog />
    </div>

    <!-- Login page (no sidebar) -->
    <router-view v-else />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from './stores/auth'
import { useRouter, useRoute } from 'vue-router'
import ConfirmDialog from './components/ConfirmDialog.vue'
import NotificationBell from './components/NotificationBell.vue'
import AppFooter from './components/AppFooter.vue'
import AppIcon from './components/AppIcon.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'
import { subscribeHttpErrors } from './utils/httpErrorBus'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const userMenuOpen = ref(false)
const userMenuRef = ref(null)
const secondaryMenuOpen = ref(false)
const adminMenuOpen = ref(false)
const httpError = ref('')
let unsubscribeHttpErrors = () => {}

// Offline detection — tracks browser connectivity via navigator.onLine events.
// A "false" value means the browser has no network; the server may still be
// reachable on a local network even when this is false, but it's the best
// signal available without polling.
const isOnline = ref(navigator.onLine)
function handleOnline() { isOnline.value = true }
function handleOffline() { isOnline.value = false }

const secondaryRoutes = ['/security', '/scheduled-tasks', '/network', '/settings']
const adminRoutes = ['/git-webhooks', '/audit', '/users']

const isSecondaryActive = computed(() => secondaryRoutes.some(r => route.path.startsWith(r)))
const isAdminActive = computed(() => adminRoutes.some(r => route.path.startsWith(r)))

function handleLogout() {
  userMenuOpen.value = false
  auth.logout()
  router.push('/login')
}

function toggleUserMenu() {
  secondaryMenuOpen.value = false
  adminMenuOpen.value = false
  userMenuOpen.value = !userMenuOpen.value
}

function toggleSecondaryMenu() {
  userMenuOpen.value = false
  adminMenuOpen.value = false
  secondaryMenuOpen.value = !secondaryMenuOpen.value
}

function toggleAdminMenu() {
  userMenuOpen.value = false
  secondaryMenuOpen.value = false
  adminMenuOpen.value = !adminMenuOpen.value
}

function handleOutsideClick(event) {
  if (!userMenuOpen.value && !secondaryMenuOpen.value && !adminMenuOpen.value) return
  const el = userMenuRef.value
  // Close user menu if click is outside
  if (userMenuOpen.value && el && !el.contains(event.target)) {
    userMenuOpen.value = false
  }
  // Close dropdowns if click is outside the navbar
  const navbar = document.getElementById('navbar-menu')
  if (navbar && !navbar.contains(event.target)) {
    secondaryMenuOpen.value = false
    adminMenuOpen.value = false
  }
}

onMounted(() => {
  unsubscribeHttpErrors = subscribeHttpErrors((event) => {
    httpError.value = event.message
  })
  window.addEventListener('online', handleOnline)
  window.addEventListener('offline', handleOffline)
  document.addEventListener('click', handleOutsideClick, true)
  // Auto-close all menus after navigation
  router.afterEach(() => {
    secondaryMenuOpen.value = false
    adminMenuOpen.value = false
    userMenuOpen.value = false
    const el = document.getElementById('navbar-menu')
    if (el?.classList.contains('show')) {
      el.classList.remove('show')
      const toggler = document.querySelector('.navbar-toggler[data-bs-target="#navbar-menu"]')
      if (toggler) toggler.setAttribute('aria-expanded', 'false')
    }
  })
})

onUnmounted(() => {
  unsubscribeHttpErrors()
  window.removeEventListener('online', handleOnline)
  window.removeEventListener('offline', handleOffline)
  document.removeEventListener('click', handleOutsideClick, true)
})
</script>

<style scoped>
.skip-link {
  position: absolute;
  top: -100%;
  left: 1rem;
  z-index: 9999;
  padding: 0.5rem 1rem;
  background: var(--tblr-primary);
  color: #fff;
  border-radius: 0 0 4px 4px;
  font-size: 0.875rem;
  text-decoration: none;
  transition: top 0.1s;
}
.skip-link:focus {
  top: 0;
}

.user-menu {
  z-index: 2000;
}

.nav-item.dropdown .dropdown-menu {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  min-width: 200px;
  z-index: 1050;
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

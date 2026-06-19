<template>
  <div class="page">
    <!-- Skip navigation link for keyboard/screen reader users -->
    <a
      href="#main-content"
      class="skip-link visually-hidden-focusable"
    >Aller au contenu principal</a>

    <!-- Sidebar + Main -->
    <div v-if="auth.isAuthenticated">
      <header class="navbar navbar-expand-md navbar-dark">
        <div class="container-xl">
          <button
            class="navbar-toggler"
            type="button"
            aria-label="Ouvrir le menu de navigation"
            aria-controls="navbar-menu"
            :aria-expanded="navbarOpen"
            @click="navbarOpen = !navbarOpen"
          >
            <span class="navbar-toggler-icon" />
          </button>
          <router-link
            to="/"
            class="navbar-brand navbar-brand-autodark"
          >
            <IconServer class="icon me-2" />
            ServerSupervisor
          </router-link>

          <div
            id="navbar-menu"
            :class="['collapse navbar-collapse', { show: navbarOpen }]"
          >
            <ul class="navbar-nav">
              <!-- Badge hôtes hors ligne -->
              <li class="nav-item">
                <span
                  v-if="hostsDownCount > 0"
                  class="badge bg-red-lt text-red ms-2 py-2 hosts-down-badge"
                >
                  <IconAlertTriangle class="icon icon-sm me-1" />
                  {{ hostsDownCount }} HORS LIGNE
                </span>
              </li>
              <!-- Éléments principaux -->
              <li class="nav-item">
                <router-link
                  to="/"
                  class="nav-link"
                  active-class="active"
                >
                  <span class="nav-link-icon">
                    <IconLayoutDashboard class="icon" />
                  </span>
                  <span class="nav-link-title">Dashboard</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link
                  to="/docker"
                  class="nav-link"
                  active-class="active"
                >
                  <span class="nav-link-icon">
                    <IconBrandDocker class="icon" />
                  </span>
                  <span class="nav-link-title">Docker</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link
                  to="/apt"
                  class="nav-link"
                  active-class="active"
                >
                  <span class="nav-link-icon">
                    <IconRefresh class="icon" />
                  </span>
                  <span class="nav-link-title">Mises à jour</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link
                  to="/proxmox"
                  class="nav-link"
                  active-class="active"
                >
                  <span class="nav-link-icon">
                    <IconServer2 class="icon" />
                  </span>
                  <span class="nav-link-title">Proxmox</span>
                </router-link>
              </li>
              <li class="nav-item">
                <router-link
                  to="/alerts"
                  class="nav-link"
                  active-class="active"
                >
                  <span class="nav-link-icon">
                    <IconBell class="icon" />
                  </span>
                  <span class="nav-link-title">Alertes</span>
                </router-link>
              </li>

              <!-- Dropdown "Plus" — éléments secondaires -->
              <li
                class="nav-item dropdown"
                :class="{ active: isSecondaryActive }"
              >
                <button
                  class="nav-link dropdown-toggle nav-dropdown-toggle"
                  type="button"
                  :aria-expanded="secondaryMenuOpen"
                  aria-label="Plus d'options"
                  aria-haspopup="menu"
                  @click="toggleSecondaryMenu"
                >
                  <span class="nav-link-icon">
                    <IconDots class="icon" />
                  </span>
                  <span class="nav-link-title">Plus</span>
                </button>
                <div
                  class="dropdown-menu"
                  :class="{ show: secondaryMenuOpen }"
                  role="menu"
                >
                  <router-link
                    v-if="auth.isAdmin"
                    to="/threats"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconShieldCheck
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Menaces web
                  </router-link>
                  <router-link
                    v-if="auth.isAdmin"
                    to="/traffic"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconChartLine
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Stats web
                  </router-link>
                  <div
                    v-if="auth.isAdmin"
                    class="dropdown-divider"
                  />
                  <router-link
                    to="/scheduled-tasks"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconClock
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Tâches planifiées
                  </router-link>
                  <router-link
                    to="/commands"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconTerminal2
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Commandes en cours
                  </router-link>
                  <router-link
                    to="/network"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconTopologyStar3
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Réseau
                  </router-link>
                  <router-link
                    to="/monitoring"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconActivity
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Monitoring
                  </router-link>
                  <router-link
                    to="/npm"
                    class="dropdown-item"
                    role="menuitem"
                    @click="secondaryMenuOpen = false"
                  >
                    <IconBox
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    NPM
                  </router-link>
                </div>
              </li>

              <!-- Dropdown "Administration" — admin uniquement -->
              <li
                v-if="auth.isAdmin"
                class="nav-item dropdown"
                :class="{ active: isAdminActive }"
              >
                <button
                  class="nav-link dropdown-toggle nav-dropdown-toggle"
                  type="button"
                  :aria-expanded="adminMenuOpen"
                  aria-label="Options administrateur"
                  aria-haspopup="menu"
                  @click="toggleAdminMenu"
                >
                  <span class="nav-link-icon">
                    <IconShieldLock class="icon" />
                  </span>
                  <span class="nav-link-title">Admin</span>
                </button>
                <div
                  class="dropdown-menu"
                  :class="{ show: adminMenuOpen }"
                  role="menu"
                >
                  <router-link
                    to="/git-webhooks"
                    class="dropdown-item"
                    role="menuitem"
                    @click="adminMenuOpen = false"
                  >
                    <IconGitBranch
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Git / Automatisation
                  </router-link>
                  <router-link
                    to="/audit"
                    class="dropdown-item"
                    role="menuitem"
                    @click="adminMenuOpen = false"
                  >
                    <IconClipboardList
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Audit
                  </router-link>
                  <router-link
                    to="/users"
                    class="dropdown-item"
                    role="menuitem"
                    @click="adminMenuOpen = false"
                  >
                    <IconUsers
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Utilisateurs
                  </router-link>
                  <div class="dropdown-divider" />
                  <router-link
                    to="/settings"
                    class="dropdown-item"
                    role="menuitem"
                    @click="adminMenuOpen = false"
                  >
                    <IconSettings
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    Paramètres
                  </router-link>
                </div>
              </li>
            </ul>

            <div class="ms-auto d-flex align-items-center gap-2">
              <NotificationBell />
              <div
                ref="userMenuRef"
                class="position-relative user-menu"
              >
                <button
                  class="btn btn-outline-secondary d-flex align-items-center"
                  @click="toggleUserMenu"
                >
                  <span class="avatar avatar-sm bg-secondary-lt me-2">
                    {{ auth.username?.slice(0, 2).toUpperCase() }}
                  </span>
                  <span class="me-2">{{ auth.username }}</span>
                  <span class="caret" />
                </button>

                <div
                  v-if="userMenuOpen"
                  class="dropdown-menu dropdown-menu-end show user-dropdown"
                >
                  <div class="dropdown-header">
                    Compte
                  </div>
                  <div class="dropdown-item text-secondary small">
                    Rôle: {{ auth.role || 'inconnu' }}
                  </div>
                  <router-link
                    to="/account"
                    class="dropdown-item"
                    @click="userMenuOpen = false"
                  >
                    Mon compte
                  </router-link>
                  <div class="dropdown-divider" />
                  <button
                    class="dropdown-item text-danger"
                    @click="handleLogout"
                  >
                    Déconnexion
                  </button>
                </div>
              </div>
            </div><!-- end ms-auto wrapper -->
          </div>
        </div>
      </header>

      <!-- Offline / server-unreachable banner -->
      <div
        v-if="!isOnline || serverUnreachable"
        class="alert alert-warning alert-dismissible mb-0 rounded-0 border-0 border-bottom app-network-alert"
        role="alert"
      >
        <div class="container-xl d-flex align-items-center gap-2">
          <IconAlertTriangle
            :size="20"
            class="icon flex-shrink-0"
          />
          <span v-if="!isOnline">Pas de connexion réseau — les données affichées peuvent être obsolètes.</span>
          <span v-else>Serveur injoignable — reconnexion en cours, les données affichées peuvent être obsolètes.</span>
        </div>
      </div>

      <div
        v-if="httpError"
        class="alert alert-danger alert-dismissible mb-0 rounded-0 border-0 border-bottom app-http-alert"
        role="alert"
      >
        <div class="container-xl d-flex align-items-center justify-content-between gap-3">
          <span>{{ httpError }}</span>
          <button
            type="button"
            class="btn-close"
            aria-label="Fermer"
            @click="httpError = ''"
          />
        </div>
      </div>

      <div class="page-wrapper">
        <div
          id="main-content"
          class="page-body"
        >
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
      <!-- Global Toast Notifications -->
      <ToastContainer />
    </div>

    <!-- Login page (no sidebar) -->
    <router-view v-else />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from './stores/auth'
import { useHostsStore } from './stores/hosts'
import { useRouter, useRoute } from 'vue-router'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ToastContainer from './components/ToastContainer.vue'
import NotificationBell from './components/NotificationBell.vue'
import AppFooter from './components/AppFooter.vue'
import {
  IconAlertTriangle, IconLayoutDashboard, IconBrandDocker, IconRefresh,
  IconServer2, IconBell, IconDots, IconShieldLock, IconShieldCheck,
  IconChartLine, IconClock, IconTerminal2, IconTopologyStar3, IconActivity,
  IconBox, IconGitBranch, IconClipboardList, IconUsers, IconSettings, IconServer,
} from '@tabler/icons-vue'
import ErrorBoundary from './components/common/ErrorBoundary.vue'
import { subscribeHttpErrors, subscribeNetworkOk } from './utils/httpErrorBus'
import apiClient from './api'

const auth = useAuthStore()
const hostsStore = useHostsStore()
const router = useRouter()
const route = useRoute()
const navbarOpen = ref(false)
const userMenuOpen = ref(false)
const userMenuRef = ref<HTMLElement | null>(null)
const secondaryMenuOpen = ref(false)
const adminMenuOpen = ref(false)
const httpError = ref('')
// True when the backend is unreachable (network error) even though the browser
// reports it is online — drives the connectivity banner, auto-clears on recovery.
const serverUnreachable = ref(false)
let unsubscribeHttpErrors: () => void = () => {}
let unsubscribeNetworkOk: () => void = () => {}
let resumeDebounceTimer: ReturnType<typeof setTimeout> | null = null

// Computed property: compter les hôtes hors ligne
const hostsDownCount = computed(() => {
  return hostsStore.hosts.filter(
    (h) => h.status === 'offline'
  ).length
})

// Offline detection — tracks browser connectivity via navigator.onLine events.
// A "false" value means the browser has no network; the server may still be
// reachable on a local network even when this is false, but it's the best
// signal available without polling.
const isOnline = ref(navigator.onLine)
function handleOnline(): void { isOnline.value = true }
function handleOffline(): void { isOnline.value = false }

function notifyAppResume(): void {
  if (resumeDebounceTimer) {
    clearTimeout(resumeDebounceTimer)
  }
  resumeDebounceTimer = setTimeout(() => {
    window.dispatchEvent(new CustomEvent('ss:app-resume', { detail: { at: Date.now() } }))
  }, 600)
}

function handleVisibilityResume(): void {
  if (document.visibilityState === 'visible') {
    notifyAppResume()
  }
}

function handlePageShow(event: PageTransitionEvent): void {
  if (event.persisted || document.visibilityState === 'visible') {
    notifyAppResume()
  }
}

const secondaryRoutes = ['/threats', '/traffic', '/scheduled-tasks', '/network', '/monitoring', '/npm']
const adminRoutes = ['/git-webhooks', '/audit', '/users', '/settings']

const isSecondaryActive = computed(() => secondaryRoutes.some(r => route.path.startsWith(r)))
const isAdminActive = computed(() => adminRoutes.some(r => route.path.startsWith(r)))

async function handleLogout(): Promise<void> {
  userMenuOpen.value = false
  // Remove push subscription before clearing auth token so the DELETE /push/subscribe call succeeds
  if ('serviceWorker' in navigator && 'PushManager' in window) {
    try {
      const reg = await navigator.serviceWorker.ready
      const sub = await reg.pushManager.getSubscription()
      if (sub) {
        await apiClient.unsubscribePush(sub.endpoint).catch(() => {})
        await sub.unsubscribe()
      }
    } catch {
      // Non-critical
    }
  }
  localStorage.removeItem('ss_vapid_public_key')
  // Invalidate the refresh token server-side and let the server clear cookies.
  try {
    await apiClient.logout()
  } catch {
    // Server might be unreachable; we still purge local state.
  }
  auth.logout()
  router.push('/login')
}

function toggleUserMenu(): void {
  secondaryMenuOpen.value = false
  adminMenuOpen.value = false
  userMenuOpen.value = !userMenuOpen.value
}

function toggleSecondaryMenu(): void {
  userMenuOpen.value = false
  adminMenuOpen.value = false
  secondaryMenuOpen.value = !secondaryMenuOpen.value
}

function toggleAdminMenu(): void {
  userMenuOpen.value = false
  secondaryMenuOpen.value = false
  adminMenuOpen.value = !adminMenuOpen.value
}

function handleOutsideClick(event: MouseEvent): void {
  if (!userMenuOpen.value && !secondaryMenuOpen.value && !adminMenuOpen.value) return
  const el = userMenuRef.value
  const target = event.target as Node
  if (userMenuOpen.value && el && !el.contains(target)) {
    userMenuOpen.value = false
  }
  const navbar = document.getElementById('navbar-menu')
  if (navbar && !navbar.contains(target)) {
    secondaryMenuOpen.value = false
    adminMenuOpen.value = false
  }
}

onMounted(() => {
  unsubscribeHttpErrors = subscribeHttpErrors((event) => {
    // Network failures (no HTTP status) surface as the connectivity banner;
    // actionable HTTP errors (403/5xx) keep their own dismissible banner.
    if (event.status === null) {
      serverUnreachable.value = true
    } else {
      httpError.value = event.message
    }
  })
  unsubscribeNetworkOk = subscribeNetworkOk(() => {
    serverUnreachable.value = false
  })
  window.addEventListener('online', handleOnline)
  window.addEventListener('offline', handleOffline)
  document.addEventListener('visibilitychange', handleVisibilityResume)
  window.addEventListener('pageshow', handlePageShow)
  window.addEventListener('focus', notifyAppResume)
  document.addEventListener('click', handleOutsideClick, true)
  // Auto-close all menus after navigation
  router.afterEach(() => {
    navbarOpen.value = false
    secondaryMenuOpen.value = false
    adminMenuOpen.value = false
    userMenuOpen.value = false
  })
})

onUnmounted(() => {
  unsubscribeHttpErrors()
  unsubscribeNetworkOk()
  window.removeEventListener('online', handleOnline)
  window.removeEventListener('offline', handleOffline)
  document.removeEventListener('visibilitychange', handleVisibilityResume)
  window.removeEventListener('pageshow', handlePageShow)
  window.removeEventListener('focus', notifyAppResume)
  document.removeEventListener('click', handleOutsideClick, true)
  if (resumeDebounceTimer) {
    clearTimeout(resumeDebounceTimer)
  }
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

.navbar {
  position: relative;
  z-index: 1030;
  overflow: visible;
}

.nav-dropdown-toggle {
  background: transparent;
  border: 0;
  width: 100%;
  text-align: left;
}

.hosts-down-badge {
  line-height: 1.5;
}

.app-network-alert {
  position: sticky;
  top: 0;
  z-index: 1040;
}

.app-http-alert {
  position: sticky;
  top: 0;
  z-index: 1039;
}

#navbar-menu {
  overflow: visible;
}

.user-menu {
  z-index: 1035;
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
  border: 1px solid var(--ss-overlay-light);
  box-shadow: var(--ss-shadow-floating);
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
  border-left: 1px solid var(--ss-overlay-light);
  border-top: 1px solid var(--ss-overlay-light);
  transform: rotate(45deg);
}

@media (max-width: 768px) {
  .ms-auto.d-flex.align-items-center.gap-2 {
    width: 100%;
    justify-content: space-between;
    margin-top: 0.5rem;
  }

  .nav-item.dropdown .dropdown-menu,
  .user-dropdown {
    position: static;
    width: 100%;
    margin-top: 0.35rem;
    box-shadow: none;
    border: 1px solid var(--tblr-border-color);
    border-radius: 0.6rem;
  }

  .user-dropdown::before {
    display: none;
  }

  .user-menu {
    width: 100%;
  }

  .user-menu > .btn {
    width: 100%;
    justify-content: flex-start;
  }
}

</style>

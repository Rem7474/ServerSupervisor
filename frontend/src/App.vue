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
              <li
                v-if="hostsDownCount > 0"
                class="nav-item d-flex align-items-center"
              >
                <span class="badge bg-red-lt text-red ms-2 py-2 hosts-down-badge">
                  <IconAlertTriangle class="icon icon-sm me-1" />
                  {{ hostsDownCount }} HORS LIGNE
                </span>
              </li>

              <!-- 6 sections d'intention (config/navigation.ts) -->
              <li
                v-for="section in visibleSections"
                :key="section.key"
                class="nav-item dropdown"
                :class="{ active: isSectionActive(section) }"
              >
                <button
                  class="nav-link dropdown-toggle nav-dropdown-toggle"
                  type="button"
                  :aria-expanded="openSectionKey === section.key"
                  :aria-label="section.label"
                  aria-haspopup="menu"
                  @click="toggleSection(section.key)"
                >
                  <span class="nav-link-icon">
                    <component
                      :is="section.icon"
                      class="icon"
                    />
                  </span>
                  <span class="nav-link-title">{{ section.label }}</span>
                </button>
                <div
                  class="dropdown-menu"
                  :class="{ show: openSectionKey === section.key }"
                  role="menu"
                >
                  <router-link
                    v-for="item in section.items"
                    :key="item.to"
                    :to="item.to"
                    class="dropdown-item"
                    role="menuitem"
                    @click="openSectionKey = null"
                  >
                    <component
                      :is="item.icon"
                      :size="16"
                      class="icon icon-sm me-2"
                    />
                    {{ item.label }}
                    <span
                      v-if="item.to === '/proxmox' && suggestedProxmoxLinksCount > 0"
                      class="badge bg-azure-lt text-azure ms-1"
                      :title="`${suggestedProxmoxLinksCount} liaison(s) hôte ↔ VM/LXC suggérée(s), à confirmer`"
                    >{{ suggestedProxmoxLinksCount }}</span>
                  </router-link>
                </div>
              </li>
            </ul>

            <div class="ms-auto d-flex align-items-center gap-2">
              <button
                type="button"
                class="btn btn-outline-secondary d-none d-sm-flex align-items-center gap-2 command-palette-trigger"
                title="Rechercher (Ctrl+K)"
                @click="paletteToggle"
              >
                <IconSearch
                  :size="16"
                  class="icon"
                />
                <span class="text-secondary small">Rechercher…</span>
                <kbd class="ms-2">Ctrl K</kbd>
              </button>
              <button
                type="button"
                class="btn btn-icon d-sm-none"
                aria-label="Rechercher"
                @click="paletteToggle"
              >
                <IconSearch :size="18" />
              </button>
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
      <!-- Command palette — mounted only while open so its search input gets
           a fresh onMounted focus every time (see CommandPalette.vue) -->
      <CommandPalette v-if="paletteOpen" />
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
import { IconAlertTriangle, IconServer, IconSearch } from '@tabler/icons-vue'
import ErrorBoundary from './components/common/ErrorBoundary.vue'
import CommandPalette from './components/CommandPalette.vue'
import { subscribeHttpErrors, subscribeNetworkOk } from './utils/httpErrorBus'
import { useCommandPalette } from './composables/useCommandPalette'
import { useAttentionCenter } from './composables/useAttentionCenter'
import apiClient from './api'
import { visibleNavSections, type NavSection } from './config/navigation'

const auth = useAuthStore()
const hostsStore = useHostsStore()
const router = useRouter()
const route = useRoute()
const { isOpen: paletteOpen, toggle: paletteToggle } = useCommandPalette()
const navbarOpen = ref(false)
const userMenuOpen = ref(false)
const userMenuRef = ref<HTMLElement | null>(null)
const openSectionKey = ref<string | null>(null)
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

// Suggested Proxmox links badge: shares useAttentionCenter's module-level
// state (and its own auth-gated poll/watch — see that composable) rather
// than fetching independently, so this badge and DashboardView's "Attention
// requise" card can no longer disagree about the count.
const { items: attentionItems } = useAttentionCenter()
const suggestedProxmoxLinksCount = computed(
  () => attentionItems.value.find((i) => i.key === 'proxmox-links')?.count ?? 0
)

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

const visibleSections = computed(() => visibleNavSections(auth.isAdmin))

function isItemActive(to: string): boolean {
  return to === '/' ? route.path === '/' : route.path.startsWith(to)
}

function isSectionActive(section: NavSection): boolean {
  return section.items.some((item) => isItemActive(item.to))
}

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
  openSectionKey.value = null
  userMenuOpen.value = !userMenuOpen.value
}

function toggleSection(key: string): void {
  userMenuOpen.value = false
  openSectionKey.value = openSectionKey.value === key ? null : key
}

function handleOutsideClick(event: MouseEvent): void {
  if (!userMenuOpen.value && !openSectionKey.value) return
  const el = userMenuRef.value
  const target = event.target as Node
  if (userMenuOpen.value && el && !el.contains(target)) {
    userMenuOpen.value = false
  }
  const navbar = document.getElementById('navbar-menu')
  if (navbar && !navbar.contains(target)) {
    openSectionKey.value = null
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
    openSectionKey.value = null
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

/* Tabler's default <kbd> (--tblr-code-bg, a near-black code-block swatch in
   dark mode + h5-scale font) reads as a mismatched "code snippet" chip next
   to the button's small, muted "Rechercher…" label instead of a subtle
   keyboard-shortcut hint. Sized/toned down to match. */
.command-palette-trigger kbd {
  padding: 0.15rem 0.4rem;
  font-size: 0.7rem;
  line-height: 1.4;
  color: var(--tblr-secondary);
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--tblr-border-color);
  border-radius: 4px;
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

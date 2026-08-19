<template>
  <div class="page">
    <!-- Skip navigation link for keyboard/screen reader users -->
    <a
      href="#main-content"
      class="skip-link visually-hidden-focusable"
    >Aller au contenu principal</a>

    <!-- Sidebar + Main -->
    <div v-if="auth.isAuthenticated">
      <!-- Row 1: brand + global status/actions (always visible, never collapses) -->
      <header class="navbar navbar-expand-md navbar-dark">
        <div class="container-xl flex-nowrap">
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
            class="navbar-brand navbar-brand-autodark text-truncate"
          >
            <IconServer class="icon me-sm-2" />
            <span class="d-none d-sm-inline">ServerSupervisor</span>
          </router-link>

          <span
            v-if="hostsDownCount > 0"
            class="badge bg-danger-lt text-danger ms-2 py-2 hosts-down-badge d-none d-md-inline-flex align-items-center"
          >
            <IconAlertTriangle class="icon icon-sm me-1" />
            {{ hostsDownCount }} HORS LIGNE
          </span>

          <div class="navbar-nav flex-row order-last">
            <div class="nav-item d-flex flex-row align-items-center gap-2">
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
                  class="btn btn-outline-secondary d-flex align-items-center px-2 px-sm-3"
                  @click="toggleUserMenu"
                >
                  <span class="avatar avatar-sm bg-secondary-lt d-none d-sm-flex me-sm-2">
                    {{ auth.username?.slice(0, 2).toUpperCase() }}
                  </span>
                  <span class="d-none d-md-inline me-2">{{ auth.username }}</span>
                  <IconUser
                    :size="18"
                    class="d-sm-none"
                  />
                  <span class="caret d-none d-sm-inline" />
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
            </div>
          </div>
        </div>
      </header>

      <!-- Row 2: the 6 intent sections (config/navigation.ts) — collapses on mobile -->
      <header class="navbar navbar-expand-md navbar-light navbar-secondary">
        <div class="container-xl">
          <div
            id="navbar-menu"
            :class="['collapse navbar-collapse', { show: navbarOpen }]"
          >
            <ul class="navbar-nav">
              <!-- Badge hôtes hors ligne (mobile: row 1's copy is hidden below md) -->
              <li
                v-if="hostsDownCount > 0"
                class="nav-item d-flex d-md-none align-items-center"
              >
                <span class="badge bg-danger-lt text-danger ms-2 py-2 hosts-down-badge">
                  <IconAlertTriangle class="icon icon-sm me-1" />
                  {{ hostsDownCount }} HORS LIGNE
                </span>
              </li>

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
          </div>
        </div>
      </header>

      <!-- Offline / server-unreachable banner -->
      <div
        v-if="!isOnline || serverUnreachable"
        class="alert alert-warning alert-dismissible mb-0 rounded-0 border-0 border-bottom sticky-top app-network-alert"
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
        class="alert alert-danger alert-dismissible mb-0 rounded-0 border-0 border-bottom sticky-top app-http-alert"
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
import { IconAlertTriangle, IconServer, IconSearch, IconUser } from '@tabler/icons-vue'
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

const visibleSections = computed(() => visibleNavSections(auth))

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
  color: var(--tblr-white);
  border-radius: 0 0 4px 4px;
  font-size: 0.875rem;
  text-decoration: none;
  transition: top 0.1s;
}
.skip-link:focus {
  top: 0;
}

/* position: relative already comes from Tabler's own .navbar rule — only
   the stacking level (not a Tabler default) needs setting here. */
.navbar {
  z-index: 1030;
}

/* Row 2 (the 6 intent-section links) — Tabler ships no .navbar-light in this
   dark-only app (body[data-bs-theme=dark] forces dark navbar colors on every
   .navbar regardless of that class), so the two-row split needs its own
   subtle shade step to read as a distinct row rather than one tall bar.
   --tblr-bg-surface-secondary (one step darker than --tblr-bg-surface, which
   row 1/cards use) is Tabler's own token for exactly this "secondary surface"
   role. */
.navbar-secondary {
  background-color: var(--tblr-bg-surface-secondary);
  border-top: 1px solid var(--tblr-border-color);
  /* Lower than row 1's shared .navbar z-index (1030): row 1's dropdowns
     (notification bell, user menu) are absolutely positioned within row 1
     but visually extend down past row 1's own bottom edge, into row 2's
     screen area. Row 2 is a sibling stacking context at the same z-index,
     later in DOM order — without this, it paints over the top of row 1's
     open dropdowns instead of sitting behind them. Confirmed via
     elementFromPoint() during visual QA: the "Compte" dropdown-header text
     was there in the DOM with correct color/position, just painted over. */
  z-index: 1020;
}

/* background/border already come from Tabler's own .nav-link rule (this
   button also carries that class) — only the button-vs-<a> defaults
   (block width, left-aligned text) need resetting here. */
.nav-dropdown-toggle {
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

/* .btn-outline-secondary's hover fills the button background with
   --tblr-secondary (Tabler's own --tblr-btn-hover-bg) — without this, the
   "Rechercher…" label (text-secondary) and the kbd hint above (color:
   --tblr-secondary too) become the same gray as the fill they now sit on,
   i.e. invisible text on hover. Force both to the button's own hover
   foreground token instead. Needs !important: Tabler's own .text-secondary
   utility sets color with !important, so a plain override here is silently
   discarded — verified live that without it the "Rechercher…" label stays
   the same dim gray on hover while only the kbd badge brightens. */
.command-palette-trigger:hover .text-secondary {
  color: var(--tblr-secondary-fg) !important;
}

.command-palette-trigger:hover kbd {
  color: var(--tblr-secondary-fg);
  border-color: rgba(255, 255, 255, 0.4);
  background: rgba(255, 255, 255, 0.12);
}

/* position: sticky; top: 0 now come from Tabler's own .sticky-top utility
   (applied in the template) — only the stacking level (above .navbar/
   .navbar-secondary, below .modal) needs overriding here. */
.app-network-alert {
  z-index: 1040;
}

.app-http-alert {
  z-index: 1039;
}

.user-menu {
  z-index: 1035;
}

/* Same root cause as .command-palette-trigger above: .btn-outline-secondary's
   hover fills the button background with --tblr-secondary, but the avatar's
   .bg-secondary-lt sets its own text color to --tblr-secondary with
   !important — so the "AD" initials stay the same gray as the fill they now
   sit on and disappear on hover. Force it to the button's own hover
   foreground token instead. */
.user-menu > .btn:hover .avatar {
  color: var(--tblr-secondary-fg) !important;
}

/* position: absolute already comes from Tabler's own .dropdown-menu rule.
   top/left are still needed: this app toggles dropdowns via a plain Vue
   :class binding, not Bootstrap's JS/Popper dropdown, so there's no
   Popper-computed placement to rely on. min-width/z-index are deliberate
   overrides of Tabler's own --tblr-dropdown-min-width (11rem) and
   --tblr-dropdown-zindex (1000). */
.nav-item.dropdown .dropdown-menu {
  top: calc(100% + 4px);
  left: 0;
  min-width: 200px;
  z-index: 1050;
}

/* This element also carries .dropdown-menu, whose Tabler rule already sets
   position: absolute — only the placement/decoration overrides below are
   this component's own. */
.user-dropdown {
  min-width: 240px;
  padding: 8px 0;
  border-radius: 12px;
  border: 1px solid var(--ss-overlay-light);
  box-shadow: var(--ss-shadow-floating);
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

/* Row 1's icon cluster (search/bell/user) lives outside #navbar-menu now —
   it's a compact, always-visible header row at every viewport width, same as
   NotificationBell's own dropdown, so it needs none of the old full-width/
   static-position mobile treatment. Only row 2's section dropdowns still
   collapse into a stacked vertical list on mobile. */
@media (max-width: 767.98px) {
  .nav-item.dropdown .dropdown-menu {
    position: static;
    width: 100%;
    margin-top: 0.35rem;
    box-shadow: none;
    border: 1px solid var(--tblr-border-color);
    border-radius: 0.6rem;
  }
}

</style>

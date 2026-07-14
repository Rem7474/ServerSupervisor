import { ref, watch } from 'vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import type { AttentionItem } from '../types/generated'

export type { AttentionItem }

const REFRESH_INTERVAL_MS = 5 * 60 * 1000

// Module-level singleton: App.vue's navbar badge (liaisons Proxmox
// suggérées) and DashboardView's "Attention requise" card both need the
// same numbers — they used to each call getProxmoxLinks('suggested')
// independently and could disagree. Sharing this state (not just hitting
// the same endpoint from two places) is what actually guarantees they
// can't drift apart again. Same "shared composable" pattern as
// useNotifications.ts.
const items = ref<AttentionItem[]>([])
const loading = ref(true)
let refreshTimer: ReturnType<typeof setInterval> | null = null
let watcherReady = false

async function refresh(): Promise<void> {
  loading.value = true
  try {
    const res = await apiClient.getAttention()
    items.value = res.data?.items || []
  } catch {
    // Non-critical — keep the last known items on error.
  } finally {
    loading.value = false
  }
}

/**
 * Aggregates signals the backend already detects but that are otherwise only
 * visible by landing on the exact right page (a suggested Proxmox link only
 * shows on that host's detail page, an NPM host with monitoring off only
 * shows in the NPM list, ...). Backed by GET /v1/dashboard/attention
 * (server/internal/services/dashboard), which computes this server-side —
 * this composable used to fetch five separate list endpoints and filter
 * them client-side; that logic now lives in Go so it can't drift from the
 * navbar badge's own computation.
 *
 * Deliberately excludes CVEs and Proxmox node/storage health — those already
 * have their own banners on the Dashboard (GET /apt/cve-summary and the
 * dashboard WebSocket snapshot's proxmox_summary), duplicating them here
 * would just be noise.
 */
export function useAttentionCenter() {
  const auth = useAuthStore()

  // Gated on the isAuthenticated transition (not a bare onMounted call), for
  // the same reason App.vue's old suggested-Proxmox-links poll was: App.vue
  // mounts this composable's first caller once for the whole SPA lifetime,
  // including on the login page — an unconditional call here 401s on every
  // unauthenticated load, and login is a soft router.push (no remount), so
  // onMounted alone would never fire again once the user actually signs in.
  if (!watcherReady) {
    watcherReady = true
    watch(
      () => auth.isAuthenticated,
      (isAuth) => {
        if (isAuth) {
          refresh()
          if (!refreshTimer) refreshTimer = setInterval(refresh, REFRESH_INTERVAL_MS)
        } else {
          items.value = []
          if (refreshTimer) {
            clearInterval(refreshTimer)
            refreshTimer = null
          }
        }
      },
      { immediate: true }
    )
  }

  return { loading, items, refresh }
}

import { ref, computed, onMounted, watch, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { IconServer, IconBrandDocker, IconBell } from '@tabler/icons-vue'
import { useAuthStore } from '../stores/auth'
import { useHostsStore } from '../stores/hosts'
import { useAlertRulesStore } from '../stores/alertRules'
import apiClient from '../api'
import type { DockerContainer } from '../types/docker'
import { visibleNavSections } from '../config/navigation'
import { getAlertMetricMeta } from '../utils/alertMetrics'

export interface PaletteResult {
  key: string
  label: string
  sublabel: string
  icon: Component
  to: string
  group: 'Navigation' | 'Hôtes' | 'Conteneurs' | 'Alertes'
}

const MAX_RESULTS_PER_GROUP = 6

// Module-level singleton: the Ctrl/Cmd+K listener must work app-wide
// regardless of which page is mounted, and CommandPalette.vue (mounted only
// while open, so it gets a clean focus/lifecycle each time — see its own
// comment) needs to read/close the same isOpen the App.vue navbar button
// toggles. All state lives here rather than inside useCommandPalette()'s
// function body for the same reason — App.vue and CommandPalette.vue each
// call useCommandPalette() independently, so any state declared per-call
// (e.g. containers) would silently fork into two disconnected copies: the
// one App.vue's open() populates, and the empty one CommandPalette.vue's
// results actually read from. Same "shared composable" pattern as
// useNotifications.ts.
const isOpen = ref(false)
const query = ref('')
const activeIndex = ref(0)
const containers = ref<DockerContainer[]>([])
const containersLoaded = ref(false)
const containersLoading = ref(false)
let globalListenerReady = false

export function useCommandPalette() {
  const router = useRouter()
  const auth = useAuthStore()
  const hostsStore = useHostsStore()
  const alertRulesStore = useAlertRulesStore()

  async function ensureContainersLoaded(): Promise<void> {
    if (containersLoaded.value || containersLoading.value) return
    containersLoading.value = true
    try {
      const res = await apiClient.getAllContainers()
      containers.value = res.data?.containers || []
      containersLoaded.value = true
    } catch {
      // Non-critical — container search is a bonus; nav/host results still work.
    } finally {
      containersLoading.value = false
    }
  }

  function open(): void {
    // The Ctrl/Cmd+K listener is global (attached once from App.vue, whose
    // script setup runs regardless of the login-vs-authenticated template
    // branch) — without this guard, pressing it on the login page would
    // still flip isOpen, and CommandPalette.vue (rendered inside the
    // authenticated-only branch) would pop up right after signing in.
    if (!auth.isAuthenticated) return
    isOpen.value = true
    query.value = ''
    activeIndex.value = 0
    hostsStore.fetchHosts()
    void alertRulesStore.fetchRules()
    void ensureContainersLoaded()
  }

  function close(): void {
    isOpen.value = false
  }

  function toggle(): void {
    if (isOpen.value) close()
    else open()
  }

  const navResults = computed<PaletteResult[]>(() => {
    const q = query.value.trim().toLowerCase()
    const results: PaletteResult[] = []
    for (const section of visibleNavSections(auth.isAdmin)) {
      for (const item of section.items) {
        if (!q || item.label.toLowerCase().includes(q) || section.label.toLowerCase().includes(q)) {
          results.push({
            key: `nav:${item.to}`,
            label: item.label,
            sublabel: section.label,
            icon: item.icon,
            to: item.to,
            group: 'Navigation',
          })
        }
      }
    }
    return results.slice(0, MAX_RESULTS_PER_GROUP)
  })

  const hostResults = computed<PaletteResult[]>(() => {
    const q = query.value.trim().toLowerCase()
    if (!q) return []
    return hostsStore.hosts
      .filter((h) =>
        h.name?.toLowerCase().includes(q) ||
        h.hostname?.toLowerCase().includes(q) ||
        h.ip_address?.includes(q)
      )
      .slice(0, MAX_RESULTS_PER_GROUP)
      .map((h) => ({
        key: `host:${h.id}`,
        label: h.name || h.hostname || h.ip_address,
        sublabel: h.hostname && h.hostname !== h.name ? h.hostname : h.ip_address,
        icon: IconServer,
        to: `/hosts/${h.id}`,
        group: 'Hôtes' as const,
      }))
  })

  // Deep-links to the host detail page's Docker tab (?tab=docker, already
  // supported by useHostDetail.ts) rather than /docker — there's no
  // per-container URL, but landing on that host's own Docker tab (with its
  // per-row actions) is the closest useful destination.
  const containerResults = computed<PaletteResult[]>(() => {
    const q = query.value.trim().toLowerCase()
    if (!q) return []
    return containers.value
      .filter((c) => c.name?.toLowerCase().includes(q) || c.image?.toLowerCase().includes(q))
      .slice(0, MAX_RESULTS_PER_GROUP)
      .map((c) => ({
        key: `container:${c.id}`,
        label: c.name,
        sublabel: `${c.hostname} · ${c.image}`,
        icon: IconBrandDocker,
        to: `/hosts/${c.host_id}?tab=docker`,
        group: 'Conteneurs' as const,
      }))
  })

  // No per-rule route exists (editing happens via a modal on /alerts itself,
  // not a dedicated page) — every match deep-links to the Règles tab, where
  // the matched rule is still visible in the list to open from there.
  const alertResults = computed<PaletteResult[]>(() => {
    const q = query.value.trim().toLowerCase()
    if (!q) return []
    return alertRulesStore.rules
      .filter((r) =>
        r.name?.toLowerCase().includes(q) ||
        getAlertMetricMeta(r.metric).label.toLowerCase().includes(q)
      )
      .slice(0, MAX_RESULTS_PER_GROUP)
      .map((r) => ({
        key: `alert-rule:${r.id}`,
        label: r.name || getAlertMetricMeta(r.metric).label,
        sublabel: r.enabled ? getAlertMetricMeta(r.metric).label : 'Désactivée',
        icon: IconBell,
        to: '/alerts?tab=rules',
        group: 'Alertes' as const,
      }))
  })

  const results = computed<PaletteResult[]>(() => [
    ...navResults.value,
    ...hostResults.value,
    ...containerResults.value,
    ...alertResults.value,
  ])

  // Without this, typing a query that narrows the list below the current
  // activeIndex leaves it pointing past the end: the "active" row highlight
  // (CommandPalette.vue's :class="{ active: ... }") disappears entirely, and
  // Enter silently no-ops (selectResult guards on an undefined result) until
  // an arrow key happens to bring the index back in range.
  watch(query, () => {
    activeIndex.value = 0
  })

  // Separately clamps (rather than resets to 0) when the result set itself
  // shrinks without a query change — e.g. containers finish loading async
  // after open() and turn out to match fewer rows than the host/nav results
  // already on screen — so an in-range selection isn't yanked back to the
  // top for a change the user didn't make.
  watch(results, (list) => {
    if (activeIndex.value > list.length - 1) {
      activeIndex.value = Math.max(0, list.length - 1)
    }
  })

  function moveActive(delta: number): void {
    const len = results.value.length
    if (!len) return
    activeIndex.value = (activeIndex.value + delta + len) % len
  }

  function selectResult(result: PaletteResult | undefined): void {
    if (!result) return
    close()
    router.push(result.to)
  }

  function selectActive(): void {
    selectResult(results.value[activeIndex.value])
  }

  // Centralizes all palette keyboard handling in one place (attached once,
  // globally) rather than splitting Ctrl/Cmd+K here and arrow/enter/escape
  // in the search input's own handler — works regardless of DOM focus and
  // there's only ever one palette instance to coordinate with.
  function handleGlobalKeydown(e: KeyboardEvent): void {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      toggle()
      return
    }
    if (!isOpen.value) return
    if (e.key === 'Escape') {
      close()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      moveActive(1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      moveActive(-1)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      selectActive()
    }
  }

  onMounted(() => {
    if (globalListenerReady) return
    globalListenerReady = true
    window.addEventListener('keydown', handleGlobalKeydown)
  })

  return {
    isOpen,
    query,
    activeIndex,
    results,
    open,
    close,
    toggle,
    moveActive,
    selectResult,
    selectActive,
  }
}

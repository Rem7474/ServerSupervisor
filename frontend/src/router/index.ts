import { createRouter, createWebHistory, RouteRecordRaw, NavigationGuardNext, RouteLocationNormalized } from 'vue-router'
import { useAuthStore } from '../stores/auth'

interface RouteMeta {
  requiresAuth?: boolean
  requiresAdmin?: boolean
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/LoginView.vue'),
  },
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('../views/DashboardView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/hosts/:id',
    name: 'HostDetail',
    component: () => import('../views/HostDetailView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/docker',
    name: 'Docker',
    component: () => import('../views/DockerView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/network',
    name: 'Network',
    component: () => import('../views/NetworkView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/apt',
    name: 'APT',
    component: () => import('../views/AptView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/alerts',
    name: 'Alerts',
    component: () => import('../views/AlertsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/alerts/incidents',
    redirect: '/alerts?tab=incidents',
  },
  {
    path: '/hosts/new',
    name: 'AddHost',
    component: () => import('../views/AddHostView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/audit',
    name: 'AuditLogs',
    component: () => import('../views/AuditLogsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/security',
    name: 'Security',
    component: () => import('../views/SecurityView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/users',
    name: 'Users',
    component: () => import('../views/UsersView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/SettingsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/account',
    name: 'Account',
    component: () => import('../views/AccountView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/scheduled-tasks',
    name: 'GlobalScheduledTasks',
    component: () => import('../views/GlobalScheduledTasksView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/hosts/:id/scheduled-tasks',
    redirect: (to) => `/hosts/${to.params.id}`,
  },
  {
    path: '/git-webhooks',
    name: 'GitWebhooks',
    component: () => import('../views/GitWebhooksView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/git-webhooks/:id',
    name: 'GitWebhookDetail',
    component: () => import('../views/GitWebhookDetailView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/release-trackers/:id',
    name: 'ReleaseTrackerDetail',
    component: () => import('../views/ReleaseTrackerDetailView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/proxmox',
    name: 'Proxmox',
    component: () => import('../views/ProxmoxView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/proxmox/nodes/:id',
    name: 'ProxmoxNode',
    component: () => import('../views/ProxmoxNodeView.vue'),
    meta: { requiresAuth: true },
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(
  (to: RouteLocationNormalized, _from: RouteLocationNormalized, next: NavigationGuardNext) => {
    const auth = useAuthStore()
    const meta = to.meta as RouteMeta

    if (meta.requiresAuth && !auth.isAuthenticated) {
      next('/login')
    } else if (auth.isAuthenticated && auth.mustChangePassword && to.path !== '/account') {
      // Force password change before accessing any other page
      next('/account')
    } else if (meta.requiresAdmin && !auth.hasPermission('*')) {
      next('/')
    } else if (to.path === '/login' && auth.isAuthenticated) {
      next('/')
    } else {
      next()
    }
  }
)

// Recover from chunk load failures (network hiccup during lazy route import).
// The browser caches the old chunk references after a deployment; a hard
// reload fetches the new manifest and resolves the mismatch.
router.onError((error: any) => {
  const isChunkError =
    error?.name === 'ChunkLoadError' ||
    /loading chunk/i.test(error?.message || '') ||
    /failed to fetch dynamically imported module/i.test(error?.message || '')
  if (isChunkError) {
    window.location.reload()
  }
})

export default router

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
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
    path: '/repos',
    name: 'Repos',
    component: () => import('../views/ReposView.vue'),
    meta: { requiresAuth: true },
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
    path: '/hosts/:id/system',
    name: 'HostSystem',
    component: () => import('../views/SystemView.vue'),
    meta: { requiresAuth: true },
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    next('/login')
  } else if (auth.isAuthenticated && auth.mustChangePassword && to.path !== '/account') {
    // Force password change before accessing any other page
    next('/account')
  } else if (to.meta.requiresAdmin && !auth.isAdmin) {
    next('/')
  } else if (to.path === '/login' && auth.isAuthenticated) {
    next('/')
  } else {
    next()
  }
})

export default router

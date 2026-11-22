import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../views/LoginView.vue', () => ({ default: { template: '<div />' } }))
vi.mock('../views/DashboardView.vue', () => ({ default: { template: '<div />' } }))
vi.mock('../views/SettingsView.vue', () => ({ default: { template: '<div />' } }))
vi.mock('../views/AuditLogsView.vue', () => ({ default: { template: '<div />' } }))
vi.mock('../views/AccountView.vue', () => ({ default: { template: '<div />' } }))

import router from './index'
import { useAuthStore } from '../stores/auth'

describe('router navigation guards', () => {
  beforeEach(async () => {
    localStorage.clear()
    setActivePinia(createPinia())
    await router.push('/login')
  })

  it('redirects unauthenticated users accessing /settings to /login', async () => {
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('redirects non-admin roles (viewer, operator) away from /settings to /', async () => {
    const auth = useAuthStore()

    auth.setAuth({ role: 'viewer', username: 'bob' }, 'bob')
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/')

    auth.setAuth({ role: 'operator', username: 'carol' }, 'carol')
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('allows admins to access /settings', async () => {
    const auth = useAuthStore()
    auth.setAuth({ role: 'admin', username: 'alice' }, 'alice')

    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/settings')
  })

  it('guards legacy /admin/configuration redirect against non-admins', async () => {
    const auth = useAuthStore()
    auth.setAuth({ role: 'operator', username: 'carol' }, 'carol')

    await router.push('/admin/configuration')
    expect(router.currentRoute.value.path).toBe('/')

    auth.setAuth({ role: 'admin', username: 'alice' }, 'alice')
    await router.push('/admin/configuration')
    expect(router.currentRoute.value.path).toBe('/settings')
    expect(router.currentRoute.value.query.tab).toBe('advanced')
  })

  it('allows operator and admin on /audit while redirecting viewer to /', async () => {
    const auth = useAuthStore()

    auth.setAuth({ role: 'viewer', username: 'bob' }, 'bob')
    await router.push('/audit?tab=commandes')
    expect(router.currentRoute.value.path).toBe('/')

    auth.setAuth({ role: 'operator', username: 'carol' }, 'carol')
    await router.push('/audit?tab=commandes')
    expect(router.currentRoute.value.path).toBe('/audit')

    auth.setAuth({ role: 'admin', username: 'alice' }, 'alice')
    await router.push('/audit?tab=journal')
    expect(router.currentRoute.value.path).toBe('/audit')
  })
})

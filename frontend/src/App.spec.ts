import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { ref } from 'vue'

const { logout, unsubscribePush } = vi.hoisted(() => ({
  logout: vi.fn().mockResolvedValue(undefined),
  unsubscribePush: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('./api', () => ({
  default: { logout, unsubscribePush },
}))

// Both are module-level singletons with their own network/timer side
// effects unrelated to App.vue's own logic under test here (navbar
// dropdowns, connectivity banners, logout) — stubbed to plain static state.
vi.mock('./composables/useCommandPalette', () => ({
  useCommandPalette: () => ({ isOpen: ref(false), toggle: vi.fn() }),
}))
vi.mock('./composables/useAttentionCenter', () => ({
  useAttentionCenter: () => ({ items: ref([]) }),
}))

import App from './App.vue'
import { useAuthStore } from './stores/auth'
import { useHostsStore } from './stores/hosts'
import { emitHttpError, emitNetworkOk } from './utils/httpErrorBus'
import { setLocale } from './i18n'

function mountApp() {
  // This suite's assertions are written against French copy (this app's
  // default for its target audience) — happy-dom's navigator.language
  // doesn't match, so without this the i18n auto-detect falls back to 'en'.
  setLocale('fr')
  setActivePinia(createPinia())
  const auth = useAuthStore()
  auth.setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
  const hostsStore = useHostsStore()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/account', component: { template: '<div />' } },
      { path: '/alerts', component: { template: '<div />' } },
      { path: '/login', component: { template: '<div />' } },
    ],
  })

  const wrapper = mount(App, {
    attachTo: document.body,
    global: {
      plugins: [router],
      stubs: {
        ConfirmDialog: true,
        ToastContainer: true,
        NotificationBell: true,
        AppFooter: true,
        CommandPalette: true,
      },
    },
  })
  return { wrapper, auth, hostsStore, router }
}

describe('App.vue — offline hosts badge', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows the count of offline hosts and hides the badge when none are offline', async () => {
    const { wrapper, hostsStore } = mountApp()
    hostsStore.setHosts([
      { id: '1', status: 'offline' } as never,
      { id: '2', status: 'online' } as never,
      { id: '3', status: 'offline' } as never,
    ])
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('2 HORS LIGNE')

    hostsStore.setHosts([{ id: '1', status: 'online' } as never])
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).not.toContain('HORS LIGNE')
    wrapper.unmount()
  })
})

describe('App.vue — connectivity/HTTP error banners', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows the server-unreachable banner on a network-level error and clears it on network-ok', async () => {
    const { wrapper } = mountApp()

    emitHttpError(null, 'Network Error')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.app-network-alert').exists()).toBe(true)
    expect(wrapper.text()).toContain('Serveur injoignable')

    emitNetworkOk()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.app-network-alert').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows a dismissible HTTP error banner with the error message, separate from the connectivity banner', async () => {
    const { wrapper } = mountApp()

    emitHttpError(403, 'Accès refusé')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.app-http-alert').exists()).toBe(true)
    expect(wrapper.text()).toContain('Accès refusé')

    await wrapper.find('.app-http-alert .btn-close').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.app-http-alert').exists()).toBe(false)
    wrapper.unmount()
  })
})

describe('App.vue — nav section active state', () => {
  it('marks the section containing the current route as active', async () => {
    const { wrapper, router } = mountApp()
    await router.push('/alerts')
    await flushPromises()

    const activeSection = wrapper.find('li.nav-item.dropdown.active')
    expect(activeSection.exists()).toBe(true)
    expect(activeSection.text()).toContain('Centre de contrôle')
    wrapper.unmount()
  })
})

describe('App.vue — user menu / section dropdown outside-click handling', () => {
  it('closes the user menu when clicking outside it, and closes an open section when the user menu is opened', async () => {
    const { wrapper } = mountApp()

    await wrapper.find('.user-menu > .btn').trigger('click')
    expect(wrapper.find('.user-dropdown').exists()).toBe(true)

    // Clicking fully outside both the user menu and the nav closes it.
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.user-dropdown').exists()).toBe(false)

    // Opening a nav section dropdown closes the user menu (mutually exclusive).
    await wrapper.find('.user-menu > .btn').trigger('click')
    expect(wrapper.find('.user-dropdown').exists()).toBe(true)
    await wrapper.find('.nav-dropdown-toggle').trigger('click')
    expect(wrapper.find('.user-dropdown').exists()).toBe(false)
    expect(wrapper.find('.dropdown-menu.show').exists()).toBe(true)

    wrapper.unmount()
  })
})

describe('App.vue — logout', () => {
  it('calls the server logout, clears local auth state, and redirects to /login', async () => {
    const { wrapper, auth, router } = mountApp()
    const pushSpy = vi.spyOn(router, 'push')

    await wrapper.find('.user-menu > .btn').trigger('click')
    await wrapper.find('.dropdown-item.text-danger').trigger('click')
    await flushPromises()

    expect(logout).toHaveBeenCalled()
    expect(auth.isAuthenticated).toBe(false)
    expect(pushSpy).toHaveBeenCalledWith('/login')
    wrapper.unmount()
  })

  it('still logs out locally even if the server logout call fails', async () => {
    logout.mockRejectedValueOnce(new Error('network down'))
    const { wrapper, auth } = mountApp()

    await wrapper.find('.user-menu > .btn').trigger('click')
    await wrapper.find('.dropdown-item.text-danger').trigger('click')
    await flushPromises()

    expect(auth.isAuthenticated).toBe(false)
    wrapper.unmount()
  })
})

describe('App.vue — app-resume debounce', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces multiple resume signals (focus + pageshow) into a single ss:app-resume event', async () => {
    const { wrapper } = mountApp()
    const resumeSpy = vi.fn()
    window.addEventListener('ss:app-resume', resumeSpy)

    window.dispatchEvent(new Event('focus'))
    await vi.advanceTimersByTimeAsync(100)
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))
    await vi.advanceTimersByTimeAsync(600)

    expect(resumeSpy).toHaveBeenCalledTimes(1)

    window.removeEventListener('ss:app-resume', resumeSpy)
    wrapper.unmount()
  })
})

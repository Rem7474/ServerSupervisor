import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'

const { resolveAlertIncident, markNotificationsRead, getNotifications } = vi.hoisted(() => ({
  resolveAlertIncident: vi.fn(),
  markNotificationsRead: vi.fn(),
  getNotifications: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { resolveAlertIncident, markNotificationsRead, getNotifications },
}))

vi.mock('../composables/useWebSocket', () => ({
  useWebSocket: () => ({
    wsStatus: ref('connected'), wsError: ref(''), retryCount: ref(0),
    dataStaleAlert: ref(false), reconnect: vi.fn(), disconnect: vi.fn(), send: vi.fn(),
  }),
  wsEvents: { on: vi.fn(), off: vi.fn() },
}))

const stubs = { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } }

const baseNotification = {
  id: '1', type: 'alert_incident', host_id: 'h1', host_name: 'web-01',
  rule_name: 'CPU haut', metric: 'cpu_percent', value: 92.5,
  triggered_at: '2026-01-01T00:00:00Z', browser_notify: true,
}

async function mountBell() {
  const { default: NotificationBell } = await import('./NotificationBell.vue')
  return mount(NotificationBell, { global: { stubs } })
}

describe('NotificationBell', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.setAuth({ role: 'admin', username: 'u' } as never, 'admin')
    getNotifications.mockResolvedValue({ data: { notifications: [], read_at: null } })
  })

  it('shows the plain "Notifications" title when there are no unread items', async () => {
    const wrapper = await mountBell()
    expect(wrapper.get('.notification-bell-btn').attributes('title')).toBe('Notifications')
  })

  it('shows the pluralized unread count in the bell title and aria-label', async () => {
    getNotifications.mockResolvedValue({
      data: { notifications: [baseNotification, { ...baseNotification, id: '2' }, { ...baseNotification, id: '3' }], read_at: null },
    })
    const wrapper = await mountBell()
    await wrapper.get('.notification-bell-btn').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('.notification-bell-btn').attributes('title')).toBe('3 notifications non lues')
  })

  it('shows the empty state when opened with no notifications', async () => {
    const wrapper = await mountBell()
    await wrapper.get('.notification-bell-btn').trigger('click')
    expect(wrapper.text()).toContain('Aucune notification')
  })

  it('renders a notification row and the mark-all-read action, calling resolveIncident on click', async () => {
    getNotifications.mockResolvedValue({ data: { notifications: [baseNotification], read_at: null } })
    resolveAlertIncident.mockResolvedValue({ data: {} })
    const wrapper = await mountBell()
    await wrapper.get('.notification-bell-btn').trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Tout marquer comme lu')
    expect(wrapper.text()).toContain('Valeur :')
    expect(wrapper.text()).toContain('Voir toutes les notifications')

    const resolveBtn = wrapper.get('.notification-resolve-btn')
    expect(resolveBtn.attributes('title')).toBe('Résoudre')
    await resolveBtn.trigger('click')
    expect(resolveAlertIncident).toHaveBeenCalledWith('1')
  })

  it('shows the version prefix for a release-tracker notification', async () => {
    getNotifications.mockResolvedValue({
      data: { notifications: [{ ...baseNotification, type: 'release_tracker_detected', version: '4.4.1' }], read_at: null },
    })
    const wrapper = await mountBell()
    await wrapper.get('.notification-bell-btn').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Version :')
    expect(wrapper.text()).toContain('4.4.1')
  })

  it('translates chrome to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = await mountBell()
    await wrapper.get('.notification-bell-btn').trigger('click')
    expect(wrapper.text()).toContain('No notifications')
  })
})

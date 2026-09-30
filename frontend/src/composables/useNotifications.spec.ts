import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { setLocale } from '../i18n'

const {
  resolveAlertIncident, markNotificationsRead, getNotifications, getPushVapidPublicKey, subscribePush,
} = vi.hoisted(() => ({
  resolveAlertIncident: vi.fn(),
  markNotificationsRead: vi.fn(),
  getNotifications: vi.fn(),
  getPushVapidPublicKey: vi.fn(),
  subscribePush: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { resolveAlertIncident, markNotificationsRead, getNotifications, getPushVapidPublicKey, subscribePush },
}))

vi.mock('./useWebSocket', () => ({
  useWebSocket: () => ({
    wsStatus: ref('connected'), wsError: ref(''), retryCount: ref(0),
    dataStaleAlert: ref(false), reconnect: vi.fn(), disconnect: vi.fn(), send: vi.fn(),
  }),
  wsEvents: { on: vi.fn(), off: vi.fn() },
}))

async function mountHost() {
  const { useNotifications } = await import('./useNotifications')
  const { useGlobalToast } = await import('./useGlobalToast')
  let api!: ReturnType<typeof useNotifications>
  mount({
    setup() {
      api = useNotifications()
      return () => null
    },
  })
  return { api, toasts: useGlobalToast().toasts }
}

describe('useNotifications — resolveIncident toasts', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    setLocale('fr')
    getNotifications.mockResolvedValue({ data: { notifications: [], read_at: null } })
  })

  it('shows a translated success toast when resolving succeeds', async () => {
    resolveAlertIncident.mockResolvedValueOnce({ data: {} })
    const { api, toasts } = await mountHost()
    toasts.splice(0, toasts.length)

    await api.resolveIncident({ id: 'alert:1', type: 'alert_incident' } as never)

    expect(toasts.some((t) => t.message === 'Incident résolu' && t.type === 'success')).toBe(true)
  })

  it('shows a translated error toast when resolving fails', async () => {
    resolveAlertIncident.mockRejectedValueOnce({})
    const { api, toasts } = await mountHost()
    toasts.splice(0, toasts.length)

    await api.resolveIncident({ id: 'alert:1', type: 'alert_incident' } as never)

    expect(toasts.some((t) => t.message === 'Impossible de résoudre' && t.type === 'error')).toBe(true)
  })

  it('translates the resolve toasts to English when the locale is switched', async () => {
    resolveAlertIncident.mockResolvedValueOnce({ data: {} })
    setLocale('en')
    const { api, toasts } = await mountHost()
    toasts.splice(0, toasts.length)

    await api.resolveIncident({ id: 'alert:1', type: 'alert_incident' } as never)

    expect(toasts.some((t) => t.message === 'Incident resolved' && t.type === 'success')).toBe(true)
  })
})

describe('useNotifications — browser notification permission', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    getNotifications.mockResolvedValue({ data: { notifications: [], read_at: null } })
  })

  it('does not prompt for permission when the app loads', async () => {
    const { api } = await mountHost()
    await flushPromises()
    expect(Notification.requestPermission).not.toHaveBeenCalled()
    expect(api.browserPermission.value).toBe('default')
  })

  it('prompts only when the user asks, and remembers the answer', async () => {
    vi.mocked(Notification.requestPermission).mockResolvedValueOnce('denied')
    const { api } = await mountHost()
    await flushPromises()

    await api.enableBrowserNotifications()

    expect(Notification.requestPermission).toHaveBeenCalledTimes(1)
    expect(api.browserPermission.value).toBe('denied')
    expect(getPushVapidPublicKey).not.toHaveBeenCalled()
  })

  describe('with push support', () => {
    const subscribe = vi.fn()

    beforeEach(() => {
      subscribe.mockResolvedValue({ toJSON: () => ({ endpoint: 'https://push.example/1' }) })
      getPushVapidPublicKey.mockResolvedValue({ data: { public_key: 'AAAA' } })
      subscribePush.mockResolvedValue({ data: {} })
      vi.stubGlobal('PushManager', class {})
      Object.defineProperty(navigator, 'serviceWorker', {
        configurable: true,
        value: { ready: Promise.resolve({ pushManager: { getSubscription: async () => null, subscribe } }) },
      })
    })

    afterEach(() => {
      vi.unstubAllGlobals()
      Reflect.deleteProperty(navigator, 'serviceWorker')
      Object.assign(Notification, { permission: 'default' })
    })

    it('subscribes to push once the user grants permission', async () => {
      vi.mocked(Notification.requestPermission).mockImplementationOnce(async () => {
        Object.assign(Notification, { permission: 'granted' })
        return 'granted'
      })
      const { api } = await mountHost()
      await flushPromises()
      expect(subscribePush).not.toHaveBeenCalled()

      await api.enableBrowserNotifications()

      expect(api.browserPermission.value).toBe('granted')
      expect(subscribe).toHaveBeenCalledTimes(1)
      expect(subscribePush).toHaveBeenCalledWith({ endpoint: 'https://push.example/1' })
    })
  })

  it('follows a permission change made in the browser settings', async () => {
    const status = { state: 'prompt', onchange: null as (() => void) | null }
    Object.defineProperty(navigator, 'permissions', {
      configurable: true,
      value: { query: async () => status },
    })
    try {
      const { api } = await mountHost()
      await flushPromises()
      expect(api.browserPermission.value).toBe('default')

      Object.assign(Notification, { permission: 'denied' })
      status.state = 'denied'
      status.onchange?.()

      expect(api.browserPermission.value).toBe('denied')
    } finally {
      Reflect.deleteProperty(navigator, 'permissions')
      Object.assign(Notification, { permission: 'default' })
    }
  })

  it('reports an unsupported browser and never prompts there', async () => {
    const original = Notification
    vi.stubGlobal('Notification', undefined)
    try {
      const { api } = await mountHost()
      await flushPromises()
      expect(api.browserPermission.value).toBe('unsupported')

      await api.enableBrowserNotifications()

      expect(api.browserPermission.value).toBe('unsupported')
      expect(original.requestPermission).not.toHaveBeenCalled()
    } finally {
      vi.unstubAllGlobals()
    }
  })
})

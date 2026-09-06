import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { setLocale } from '../i18n'

const { resolveAlertIncident, markNotificationsRead, getNotifications } = vi.hoisted(() => ({
  resolveAlertIncident: vi.fn(),
  markNotificationsRead: vi.fn(),
  getNotifications: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { resolveAlertIncident, markNotificationsRead, getNotifications },
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

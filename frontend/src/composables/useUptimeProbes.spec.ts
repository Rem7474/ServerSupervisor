import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'

const {
  getUptimeProbes, getUptimeStats, getUptimeHistory, checkUptimeProbeNow,
  createUptimeProbe, updateUptimeProbe, deleteUptimeProbe,
} = vi.hoisted(() => ({
  getUptimeProbes: vi.fn(),
  getUptimeStats: vi.fn(),
  getUptimeHistory: vi.fn(),
  checkUptimeProbeNow: vi.fn(),
  createUptimeProbe: vi.fn(),
  updateUptimeProbe: vi.fn(),
  deleteUptimeProbe: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getUptimeProbes, getUptimeStats, getUptimeHistory, checkUptimeProbeNow,
    createUptimeProbe, updateUptimeProbe, deleteUptimeProbe,
  },
}))

vi.mock('../api/npm', () => ({
  npmApi: { updateProxyHost: vi.fn() },
}))

import { useUptimeProbes } from './useUptimeProbes'

const probe = {
  id: 'p1', name: 'API prod', type: 'http', target: 'https://api.example.com',
  interval_sec: 60, timeout_sec: 10, expected_status: 200, expected_body_regex: '',
  follow_redirects: true, verify_tls: true, enabled: true, last_status: 'pending',
  consecutive_failures: 0,
}

// useI18n()/useConfirmDialog() both need an active component instance.
function mountHost() {
  let api: ReturnType<typeof useUptimeProbes> | undefined
  const wrapper = mount(defineComponent({
    setup() {
      api = useUptimeProbes()
      return () => h('div')
    },
  }))
  return { wrapper, api: api! }
}

describe('useUptimeProbes', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getUptimeProbes.mockResolvedValue({ data: { probes: [probe] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 100 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
  })

  it('falls back to the translated "unknown" status label for a probe with neither up nor down status', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.probes.value).toHaveLength(1))
    expect(api.probeStatusLabel(probe as never)).toBe('Inconnue')
  })

  it('surfaces the translated fallback error when a manual check fails', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.probes.value).toHaveLength(1))
    checkUptimeProbeNow.mockRejectedValue(new Error('boom'))

    await api.checkProbeNow(probe as never)

    expect(api.error.value).toBe('Échec de la vérification')
  })

  it('surfaces the translated fallback error when saving a probe fails', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.probes.value).toHaveLength(1))
    createUptimeProbe.mockRejectedValue(new Error('boom'))
    api.openCreateProbe()

    await api.saveProbe()

    expect(api.probeFormError.value).toBe("Erreur lors de l'enregistrement")
    expect(api.savingProbe.value).toBe(false)
  })

  it('surfaces the translated fallback error when deleting a probe fails, after confirming', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.probes.value).toHaveLength(1))
    deleteUptimeProbe.mockRejectedValue(new Error('boom'))
    const dialog = useConfirmDialog()

    const deletePromise = api.confirmDeleteProbe(probe as never)
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    dialog.onConfirm()
    await deletePromise

    expect(api.error.value).toBe('Suppression impossible')
  })
})

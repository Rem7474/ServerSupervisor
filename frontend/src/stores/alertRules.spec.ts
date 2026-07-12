import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { AlertRule } from '../types/alert'

vi.mock('../api', () => ({
  default: {
    getAlertRules: vi.fn(),
  },
}))

import apiClient from '../api'
import { useAlertRulesStore } from './alertRules'

function makeRule(overrides: Partial<AlertRule> = {}): AlertRule {
  return { id: 'r1', name: 'CPU high', enabled: true, ...overrides } as AlertRule
}

describe('stores/alertRules', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('fetches rules and marks the cache fresh', async () => {
    const rules = [makeRule()]
    vi.mocked(apiClient.getAlertRules).mockResolvedValue({ data: rules } as never)
    const store = useAlertRulesStore()

    await store.fetchRules()

    expect(apiClient.getAlertRules).toHaveBeenCalledTimes(1)
    expect(store.rules).toEqual(rules)
    expect(store.fetched).toBe(true)
    expect(store.error).toBe('')
  })

  it('skips the network call when the TTL (30s) has not expired', async () => {
    vi.mocked(apiClient.getAlertRules).mockResolvedValue({ data: [makeRule()] } as never)
    const store = useAlertRulesStore()

    await store.fetchRules()
    await store.fetchRules()

    expect(apiClient.getAlertRules).toHaveBeenCalledTimes(1)
  })

  it('re-fetches once the TTL has expired', async () => {
    vi.useFakeTimers()
    vi.mocked(apiClient.getAlertRules).mockResolvedValue({ data: [makeRule()] } as never)
    const store = useAlertRulesStore()

    await store.fetchRules()
    vi.advanceTimersByTime(30_001)
    await store.fetchRules()

    expect(apiClient.getAlertRules).toHaveBeenCalledTimes(2)
  })

  it('force-refetches even when the cache is still fresh', async () => {
    vi.mocked(apiClient.getAlertRules).mockResolvedValue({ data: [makeRule()] } as never)
    const store = useAlertRulesStore()

    await store.fetchRules()
    await store.fetchRules(true)

    expect(apiClient.getAlertRules).toHaveBeenCalledTimes(2)
  })

  it('keeps the stale rules and surfaces the server-provided error message on failure', async () => {
    const rules = [makeRule()]
    vi.mocked(apiClient.getAlertRules)
      .mockResolvedValueOnce({ data: rules } as never)
      .mockRejectedValueOnce({ response: { data: { error: 'quota exceeded' } } })
    const store = useAlertRulesStore()

    await store.fetchRules()
    await store.fetchRules(true)

    expect(store.rules).toEqual(rules)
    expect(store.error).toBe('quota exceeded')
    expect(store.loading).toBe(false)
    expect(store.fetched).toBe(true)
  })

  it('falls back to a generic error message for a non-axios-shaped failure', async () => {
    vi.mocked(apiClient.getAlertRules).mockRejectedValue('boom')
    const store = useAlertRulesStore()

    await store.fetchRules()

    expect(store.error).toBe('Erreur de chargement')
  })

  it('falls back to the thrown Error.message when there is no response.data.error', async () => {
    vi.mocked(apiClient.getAlertRules).mockRejectedValue(new Error('socket hang up'))
    const store = useAlertRulesStore()

    await store.fetchRules()

    expect(store.error).toBe('socket hang up')
  })

  it('invalidate forces the next fetchRules to hit the network again', async () => {
    vi.mocked(apiClient.getAlertRules).mockResolvedValue({ data: [makeRule()] } as never)
    const store = useAlertRulesStore()

    await store.fetchRules()
    store.invalidate()
    expect(store.fetched).toBe(false)
    await store.fetchRules()

    expect(apiClient.getAlertRules).toHaveBeenCalledTimes(2)
  })
})

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  default: {
    getAlertRules: vi.fn(),
    createAlertRule: vi.fn(),
    updateAlertRule: vi.fn(),
    deleteAlertRule: vi.fn(),
    getReleaseTrackers: vi.fn(),
  },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { message?: string })?.message || fallback || 'erreur',
}))

let confirmAnswer = true
vi.mock('./useConfirmDialog', () => ({
  useConfirmDialog: () => ({ confirm: vi.fn(async () => confirmAnswer) }),
}))

import apiClient from '../api'
import { useAlertsPage } from './useAlertsPage'
import type { AlertRule } from '../types/alert'

const rule = { id: 1, name: 'CPU high', enabled: true } as AlertRule

describe('useAlertsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    confirmAnswer = true
    setLocale('fr')
    vi.mocked(apiClient.getAlertRules).mockResolvedValue({ data: [] } as never)
  })

  describe('deleteAlert', () => {
    it('does nothing when the confirmation is declined', async () => {
      confirmAnswer = false
      const p = useAlertsPage()

      await p.deleteAlert(rule)

      expect(apiClient.deleteAlertRule).not.toHaveBeenCalled()
      expect(p.saveError.value).toBe('')
    })

    it('deletes and refetches the rule list on confirmation', async () => {
      vi.mocked(apiClient.deleteAlertRule).mockResolvedValue({} as never)
      const p = useAlertsPage()

      await p.deleteAlert(rule)

      expect(apiClient.deleteAlertRule).toHaveBeenCalledWith(1)
      expect(apiClient.getAlertRules).toHaveBeenCalled()
      expect(p.saveError.value).toBe('')
    })

    it('surfaces a translated, prefixed error when the delete fails', async () => {
      vi.mocked(apiClient.deleteAlertRule).mockRejectedValue({ message: 'rule in use' })
      const p = useAlertsPage()

      await p.deleteAlert(rule)

      expect(p.saveError.value).toBe('Erreur lors de la suppression : rule in use')
    })
  })

  describe('saveAlert', () => {
    it('creates when no rule is being edited, then closes the modal', async () => {
      vi.mocked(apiClient.createAlertRule).mockResolvedValue({} as never)
      const p = useAlertsPage()
      p.startAddAlert()

      await p.saveAlert({ metric: 'cpu' })

      expect(apiClient.createAlertRule).toHaveBeenCalledWith({ metric: 'cpu' })
      expect(apiClient.updateAlertRule).not.toHaveBeenCalled()
      expect(p.showModal.value).toBe(false)
      expect(p.saving.value).toBe(false)
    })

    it('updates the rule being edited instead of creating a second one', async () => {
      vi.mocked(apiClient.updateAlertRule).mockResolvedValue({} as never)
      const p = useAlertsPage()
      p.startEditAlert(rule)

      await p.saveAlert({ metric: 'cpu' })

      expect(apiClient.updateAlertRule).toHaveBeenCalledWith(1, { metric: 'cpu' })
      expect(apiClient.createAlertRule).not.toHaveBeenCalled()
    })

    it('keeps the modal open and shows a translated error on failure', async () => {
      vi.mocked(apiClient.createAlertRule).mockRejectedValue({ message: 'seuil invalide' })
      const p = useAlertsPage()
      p.startAddAlert()

      await p.saveAlert({})

      expect(p.saveError.value).toBe('Erreur : seuil invalide')
      expect(p.showModal.value).toBe(true)
      expect(p.saving.value).toBe(false)
    })
  })

  describe('loadTrackers', () => {
    it('records a translated error and clears the loading flag on failure', async () => {
      vi.mocked(apiClient.getReleaseTrackers).mockRejectedValue({})
      const p = useAlertsPage()

      await p.loadTrackers()

      expect(p.trackersError.value).toBe('Impossible de charger les trackers de versions')
      expect(p.trackersLoading.value).toBe(false)
    })

    it('switchToTrackers fetches once and not again on a second switch', async () => {
      vi.mocked(apiClient.getReleaseTrackers).mockResolvedValue({ data: { trackers: [] } } as never)
      const p = useAlertsPage()

      await p.switchToTrackers()
      await p.switchToTrackers()

      expect(p.alertsTab.value).toBe('releases')
      expect(apiClient.getReleaseTrackers).toHaveBeenCalledTimes(1)
    })
  })
})

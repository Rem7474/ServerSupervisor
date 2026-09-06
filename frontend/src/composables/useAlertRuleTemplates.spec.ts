import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  default: {
    getAlertRuleTemplates: vi.fn(),
    createAlertRuleTemplate: vi.fn(),
    updateAlertRuleTemplate: vi.fn(),
    deleteAlertRuleTemplate: vi.fn(),
    applyAlertRuleTemplate: vi.fn(),
  },
  getApiErrorMessage: (e: unknown, fallback: string) =>
    (e as { message?: string })?.message || fallback,
}))

const toasts: { message: string; type: string }[] = []
vi.mock('./useGlobalToast', () => ({
  addToast: (message: string, type: string) => toasts.push({ message, type }),
}))

let confirmAnswer = true
vi.mock('./useConfirmDialog', () => ({
  useConfirmDialog: () => ({ confirm: vi.fn(async () => confirmAnswer) }),
}))

import apiClient from '../api'
import { useAlertRuleTemplates } from './useAlertRuleTemplates'

const template = { id: 7, name: 'CPU high' } as never

/** `Array.prototype.at` is above this project's TS lib target. */
const lastToast = () => toasts[toasts.length - 1]

describe('useAlertRuleTemplates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    toasts.length = 0
    confirmAnswer = true
    setLocale('fr')
  })

  describe('loadTemplates', () => {
    it('never leaves templates nil when the API returns no body', async () => {
      vi.mocked(apiClient.getAlertRuleTemplates).mockResolvedValue({ data: null } as never)
      const t = useAlertRuleTemplates()

      await t.loadTemplates()

      expect(t.templates.value).toEqual([])
      expect(t.fetched.value).toBe(true)
      expect(t.loading.value).toBe(false)
    })

    it('surfaces a translated error and clears the loading flag on failure', async () => {
      vi.mocked(apiClient.getAlertRuleTemplates).mockRejectedValue({})
      const t = useAlertRuleTemplates()

      await t.loadTemplates()

      expect(t.error.value).toBe('Impossible de charger les modèles de règles')
      expect(t.loading.value).toBe(false)
      expect(t.fetched.value).toBe(false)
    })
  })

  describe('createTemplate / updateTemplate', () => {
    it('reports success and refetches the list', async () => {
      vi.mocked(apiClient.createAlertRuleTemplate).mockResolvedValue({} as never)
      vi.mocked(apiClient.getAlertRuleTemplates).mockResolvedValue({ data: [template] } as never)
      const t = useAlertRuleTemplates()

      expect(await t.createTemplate({} as never)).toBe(true)
      expect(apiClient.getAlertRuleTemplates).toHaveBeenCalled()
      expect(t.saveError.value).toBe('')
    })

    it('returns false and records a translated saveError on create failure', async () => {
      vi.mocked(apiClient.createAlertRuleTemplate).mockRejectedValue({})
      const t = useAlertRuleTemplates()

      expect(await t.createTemplate({} as never)).toBe(false)
      expect(t.saveError.value).toBe('Impossible de créer le modèle')
      expect(t.saving.value).toBe(false)
    })

    it('returns false and records a translated saveError on update failure', async () => {
      vi.mocked(apiClient.updateAlertRuleTemplate).mockRejectedValue({})
      const t = useAlertRuleTemplates()

      expect(await t.updateTemplate(7, {} as never)).toBe(false)
      expect(t.saveError.value).toBe('Impossible de modifier le modèle')
    })
  })

  describe('deleteTemplate', () => {
    it('does nothing when the confirmation is declined', async () => {
      confirmAnswer = false
      const t = useAlertRuleTemplates()

      await t.deleteTemplate(template)

      expect(apiClient.deleteAlertRuleTemplate).not.toHaveBeenCalled()
      expect(toasts).toHaveLength(0)
    })

    it('removes the row locally on success instead of refetching', async () => {
      vi.mocked(apiClient.deleteAlertRuleTemplate).mockResolvedValue({} as never)
      vi.mocked(apiClient.getAlertRuleTemplates).mockResolvedValue({ data: [template] } as never)
      const t = useAlertRuleTemplates()
      await t.loadTemplates()
      vi.mocked(apiClient.getAlertRuleTemplates).mockClear()

      await t.deleteTemplate(template)

      expect(t.templates.value).toEqual([])
      expect(apiClient.getAlertRuleTemplates).not.toHaveBeenCalled()
      expect(toasts).toEqual([{ message: 'Modèle supprimé', type: 'success' }])
    })

    it('keeps the row and toasts an error when the delete fails', async () => {
      vi.mocked(apiClient.deleteAlertRuleTemplate).mockRejectedValue({})
      vi.mocked(apiClient.getAlertRuleTemplates).mockResolvedValue({ data: [template] } as never)
      const t = useAlertRuleTemplates()
      await t.loadTemplates()

      await t.deleteTemplate(template)

      expect(t.templates.value).toHaveLength(1)
      expect(toasts).toEqual([{ message: 'Impossible de supprimer le modèle', type: 'error' }])
    })
  })

  describe('applyTemplate', () => {
    it('pluralizes the success toast on the created count', async () => {
      vi.mocked(apiClient.applyAlertRuleTemplate).mockResolvedValue({
        data: { created_rule_ids: [1], errors: {} },
      } as never)
      const t = useAlertRuleTemplates()

      await t.applyTemplate(7, ['h1'], true)
      expect(lastToast()).toEqual({ message: '1 règle créée', type: 'success' })

      vi.mocked(apiClient.applyAlertRuleTemplate).mockResolvedValue({
        data: { created_rule_ids: [1, 2, 3], errors: {} },
      } as never)
      await t.applyTemplate(7, ['h1', 'h2', 'h3'], true)
      expect(lastToast()).toEqual({ message: '3 règles créées', type: 'success' })
    })

    it('pluralizes each half of a partial apply independently', async () => {
      // The apply endpoint is intentionally not all-or-nothing: one host
      // failing must not hide the rules that were created. vue-i18n resolves
      // one plural index per message, so a single "{created}…{failed}" string
      // would inflect the failure count off the created count — "1 échecs".
      vi.mocked(apiClient.applyAlertRuleTemplate).mockResolvedValue({
        data: { created_rule_ids: [1, 2], errors: { 'h3': 'boom' } },
      } as never)
      const t = useAlertRuleTemplates()

      await t.applyTemplate(7, ['h1', 'h2', 'h3'], true)

      expect(lastToast()).toEqual({ message: '2 règles créées, 1 échec', type: 'error' })
      expect(t.applyResult.value).not.toBeNull()
    })

    it('records a translated applyError and returns false on failure', async () => {
      vi.mocked(apiClient.applyAlertRuleTemplate).mockRejectedValue({})
      const t = useAlertRuleTemplates()

      expect(await t.applyTemplate(7, ['h1'], true)).toBe(false)
      expect(t.applyError.value).toBe("Impossible d'appliquer le modèle")
      expect(t.applying.value).toBe(false)
    })

    it('clearApplyResult drops the previous result', async () => {
      vi.mocked(apiClient.applyAlertRuleTemplate).mockResolvedValue({
        data: { created_rule_ids: [1], errors: {} },
      } as never)
      const t = useAlertRuleTemplates()
      await t.applyTemplate(7, ['h1'], true)

      t.clearApplyResult()

      expect(t.applyResult.value).toBeNull()
    })

    it('toasts in the active language', async () => {
      setLocale('en')
      vi.mocked(apiClient.applyAlertRuleTemplate).mockResolvedValue({
        data: { created_rule_ids: [1, 2], errors: {} },
      } as never)
      const t = useAlertRuleTemplates()

      await t.applyTemplate(7, ['h1', 'h2'], true)

      expect(lastToast()).toEqual({ message: '2 rules created', type: 'success' })
    })
  })
})

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  default: {
    getDomainDetails: vi.fn(),
    blockCrowdSecIP: vi.fn(),
  },
}))

vi.mock('../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { message?: string })?.message || fallback || 'erreur',
}))

const toasts: { message: string; type: string }[] = []
vi.mock('./useGlobalToast', () => ({
  addToast: (message: string, type: string) => toasts.push({ message, type }),
}))

let confirmAnswer = true
vi.mock('./useConfirmDialog', () => ({
  useConfirmDialog: () => ({ confirm: vi.fn(async () => confirmAnswer) }),
}))

import api from '../api'
import { useDomainDetails } from './useDomainDetails'

const lastToast = () => toasts[toasts.length - 1]

describe('useDomainDetails', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    toasts.length = 0
    confirmAnswer = true
    setLocale('fr')
  })

  describe('blockIP', () => {
    it('refuses without a host and never calls the API', async () => {
      // The CrowdSec ban is dispatched to a specific agent — with no host
      // resolved there is nothing to send it to.
      const d = useDomainDetails()

      await d.blockIP('1.2.3.4', '')

      expect(api.blockCrowdSecIP).not.toHaveBeenCalled()
      expect(lastToast()).toEqual({ message: 'Hôte introuvable pour bloquer 1.2.3.4', type: 'error' })
    })

    it('does nothing when the confirmation is declined', async () => {
      confirmAnswer = false
      const d = useDomainDetails()

      await d.blockIP('1.2.3.4', 'host-1')

      expect(api.blockCrowdSecIP).not.toHaveBeenCalled()
      expect(toasts).toHaveLength(0)
      expect(d.blockState['1.2.3.4']).toBeUndefined()
    })

    it('marks matching rows blocked locally rather than refetching', async () => {
      vi.mocked(api.blockCrowdSecIP).mockResolvedValue({} as never)
      vi.mocked(api.getDomainDetails).mockResolvedValue({
        data: {
          details: {
            top_clients: [{ ip: '1.2.3.4' }, { ip: '5.6.7.8' }],
            requests: [{ ip: '1.2.3.4' }],
          },
        },
      } as never)
      const d = useDomainDetails()
      await d.open('example.com')
      vi.mocked(api.getDomainDetails).mockClear()

      await d.blockIP('1.2.3.4', 'host-1', '8h')

      expect(api.blockCrowdSecIP).toHaveBeenCalledWith('1.2.3.4', 'host-1', '8h')
      expect(api.getDomainDetails).not.toHaveBeenCalled()
      expect(d.details.value.top_clients[0].blocked).toBe(true)
      // A different IP in the same table must be left alone.
      expect(d.details.value.top_clients[1].blocked).toBeUndefined()
      expect(d.details.value.requests[0].blocked).toBe(true)
      // The pending marker is cleared, not left spinning.
      expect(d.blockState['1.2.3.4']).toBeUndefined()
      expect(lastToast()).toEqual({ message: 'IP 1.2.3.4 bloquée par CrowdSec (8h)', type: 'success' })
    })

    it('leaves an error marker and toasts the reason on failure', async () => {
      vi.mocked(api.blockCrowdSecIP).mockRejectedValue({ message: 'agent hors ligne' })
      const d = useDomainDetails()

      await d.blockIP('1.2.3.4', 'host-1')

      expect(d.blockState['1.2.3.4']).toBe('error')
      expect(lastToast()).toEqual({
        message: 'Impossible de bloquer 1.2.3.4 : agent hors ligne',
        type: 'error',
      })
    })

    it('defaults the ban duration to 4h', async () => {
      vi.mocked(api.blockCrowdSecIP).mockResolvedValue({} as never)
      const d = useDomainDetails()

      await d.blockIP('1.2.3.4', 'host-1')

      expect(api.blockCrowdSecIP).toHaveBeenCalledWith('1.2.3.4', 'host-1', '4h')
    })
  })

  describe('open', () => {
    it('records a translated error when the fetch fails and empties the details', async () => {
      vi.mocked(api.getDomainDetails).mockRejectedValue({})
      const d = useDomainDetails()

      await d.open('example.com')

      expect(d.error.value).toBe('Erreur de chargement')
      expect(d.details.value).toEqual({})
      expect(d.loading.value).toBe(false)
    })
  })
})

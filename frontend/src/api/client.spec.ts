import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import axios, { type AxiosError } from 'axios'
import { setLocale } from '../i18n'
import { subscribeHttpErrors, type HttpErrorEvent } from '../utils/httpErrorBus'
import { api, getApiErrorMessage } from './client'

/** Drives the response interceptor's error branch directly. */
function rejectWith(error: unknown): Promise<unknown> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const handlers = (api.interceptors.response as any).handlers as {
    rejected: (e: unknown) => Promise<unknown>
  }[]
  const rejected = handlers.find((h) => h?.rejected)?.rejected
  if (!rejected) throw new Error('no response-error interceptor registered')
  return rejected(error).catch((e: unknown) => e)
}

function axiosErrorWithStatus(status: number | null): AxiosError {
  return {
    isAxiosError: true,
    name: 'AxiosError',
    message: 'boom',
    toJSON: () => ({}),
    response: status === null ? undefined : ({ status } as AxiosError['response']),
  } as AxiosError
}

describe('api/client', () => {
  let events: HttpErrorEvent[]
  let unsubscribe: () => void

  beforeEach(() => {
    setActivePinia(createPinia())
    setLocale('fr')
    events = []
    unsubscribe = subscribeHttpErrors((e) => events.push(e))
  })

  afterEach(() => {
    unsubscribe()
    vi.restoreAllMocks()
  })

  describe('getApiErrorMessage', () => {
    it('prefers the response payload error over the thrown message', () => {
      const error = { response: { data: { error: 'host unreachable' } }, message: 'Request failed' }
      expect(getApiErrorMessage(error)).toBe('host unreachable')
    })

    it('falls back to a translated generic message', () => {
      expect(getApiErrorMessage({})).toBe('Une erreur est survenue')
      setLocale('en')
      expect(getApiErrorMessage({})).toBe('Something went wrong')
    })

    it('honours an explicit fallback over the generic one', () => {
      expect(getApiErrorMessage({}, 'Impossible de charger')).toBe('Impossible de charger')
    })
  })

  describe('response error interceptor', () => {
    it.each([
      [403, 'common.httpForbidden'],
      [502, 'common.httpBadGateway'],
      [500, 'common.httpServerError'],
    ])('emits a translated message for HTTP %i', async (status) => {
      await rejectWith(axiosErrorWithStatus(status))

      expect(events).toHaveLength(1)
      expect(events[0].status).toBe(status)
      expect(events[0].message).not.toMatch(/^common\./)
      expect(events[0].message.length).toBeGreaterThan(0)
    })

    it('reports a transport failure (no response) with a null status', async () => {
      await rejectWith(axiosErrorWithStatus(null))

      expect(events).toHaveLength(1)
      expect(events[0].status).toBeNull()
      expect(events[0].message).toBe('Erreur réseau : impossible de joindre le serveur')
    })

    it('emits in the active language, not the build-time default', async () => {
      setLocale('en')
      await rejectWith(axiosErrorWithStatus(403))

      expect(events[0].message).toBe("You don't have permission to perform this action")
    })

    it('does not emit for a 401 — that path logs out instead', async () => {
      await rejectWith(axiosErrorWithStatus(401))

      expect(events).toHaveLength(0)
    })

    it('stays silent on a cancelled request', async () => {
      // axios.isCancel() keys off the __CANCEL__ marker CanceledError sets,
      // not the error code — a hand-rolled lookalike would fall through.
      await rejectWith(new axios.CanceledError('canceled'))

      expect(events).toHaveLength(0)
    })

    it('always rejects, never swallows the error', async () => {
      const original = axiosErrorWithStatus(500)
      const returned = await rejectWith(original)

      expect(returned).toBe(original)
    })
  })
})

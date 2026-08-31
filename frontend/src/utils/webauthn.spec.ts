import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  base64urlToBuffer, bufferToBase64url, isWebAuthnSupported,
  createWebAuthnCredential, getWebAuthnAssertion,
} from './webauthn'
import { setLocale } from '../i18n'

function toBuffer(bytes: number[]): ArrayBuffer {
  return new Uint8Array(bytes).buffer
}

describe('base64url <-> ArrayBuffer round-trip', () => {
  it('round-trips arbitrary bytes, including values needing "+"/"/" in standard base64', () => {
    // 251/252/253 -> standard base64 would need '+'/'/' here; base64url must use '-'/'_' instead.
    const bytes = [0, 1, 2, 251, 252, 253, 254, 255]
    const encoded = bufferToBase64url(toBuffer(bytes))
    expect(encoded).not.toMatch(/[+/=]/)
    const decoded = new Uint8Array(base64urlToBuffer(encoded))
    expect(Array.from(decoded)).toEqual(bytes)
  })

  it('round-trips an empty buffer', () => {
    const encoded = bufferToBase64url(toBuffer([]))
    expect(encoded).toBe('')
    expect(new Uint8Array(base64urlToBuffer(encoded)).length).toBe(0)
  })

  it('decodes a value whose length is not a multiple of 4 (no explicit padding)', () => {
    // 5 raw bytes -> unpadded base64url is 7 chars, not a multiple of 4.
    const bytes = [10, 20, 30, 40, 50]
    const encoded = bufferToBase64url(toBuffer(bytes))
    expect(encoded.length % 4).not.toBe(0)
    expect(Array.from(new Uint8Array(base64urlToBuffer(encoded)))).toEqual(bytes)
  })

  it('produces no standard-base64 padding characters', () => {
    const encoded = bufferToBase64url(toBuffer([1, 2, 3]))
    expect(encoded.endsWith('=')).toBe(false)
  })
})

describe('isWebAuthnSupported', () => {
  it('reflects window.PublicKeyCredential presence', () => {
    expect(isWebAuthnSupported()).toBe(typeof window !== 'undefined' && !!window.PublicKeyCredential)
  })
})

describe('WebAuthn ceremonies cancelled by the user (navigator.credentials returns null)', () => {
  afterEach(() => {
    setLocale('fr')
    vi.unstubAllGlobals()
  })

  it('createWebAuthnCredential throws a translated error when creation is cancelled', async () => {
    vi.stubGlobal('navigator', { credentials: { create: vi.fn().mockResolvedValue(null) } })
    setLocale('fr')

    await expect(createWebAuthnCredential({
      publicKey: { challenge: 'YWJj', user: { id: 'YWJj' } },
    })).rejects.toThrow('La création de la clé de sécurité a été annulée.')

    setLocale('en')
    await expect(createWebAuthnCredential({
      publicKey: { challenge: 'YWJj', user: { id: 'YWJj' } },
    })).rejects.toThrow('Security key creation was cancelled.')
  })

  it('getWebAuthnAssertion throws a translated error when verification is cancelled', async () => {
    vi.stubGlobal('navigator', { credentials: { get: vi.fn().mockResolvedValue(null) } })
    setLocale('fr')

    await expect(getWebAuthnAssertion({
      publicKey: { challenge: 'YWJj' },
    })).rejects.toThrow('La vérification de la clé de sécurité a été annulée.')

    setLocale('en')
    await expect(getWebAuthnAssertion({
      publicKey: { challenge: 'YWJj' },
    })).rejects.toThrow('Security key verification was cancelled.')
  })
})

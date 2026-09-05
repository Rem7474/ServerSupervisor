import { describe, it, expect, vi } from 'vitest'

const { get, post, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
}))

vi.mock('./client', () => ({
  api: { get, post, delete: del },
  rangeParams: vi.fn(() => ({})),
}))

import { authApi } from './auth'

describe('api/auth', () => {
  it('calls /auth/oidc/status on getOIDCStatus', () => {
    authApi.getOIDCStatus()
    expect(get).toHaveBeenCalledWith('/auth/oidc/status')
  })

  it('calls /auth/login with credentials and optional TOTP', () => {
    authApi.login('admin', 'password', '123456')
    expect(post).toHaveBeenCalledWith('/auth/login', {
      username: 'admin',
      password: 'password',
      totp_code: '123456',
    })
  })

  it('calls /auth/logout and /auth/refresh', () => {
    authApi.logout()
    expect(post).toHaveBeenCalledWith('/auth/logout', {})
    authApi.refreshSession()
    expect(post).toHaveBeenCalledWith('/auth/refresh', {})
  })
})

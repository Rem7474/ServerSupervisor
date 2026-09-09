import { describe, it, expect, vi } from 'vitest'

const { get, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}))

vi.mock('./client', () => ({
  api: { get, put, delete: del },
}))

import { configApi } from './config'

describe('configApi', () => {
  it('calls GET /v1/config without reveal by default', () => {
    configApi.getConfig()
    expect(get).toHaveBeenCalledWith('/v1/config', {
      params: undefined,
      signal: undefined,
    })
  })

  it('calls GET /v1/config with reveal=true when requested', () => {
    configApi.getConfig(true)
    expect(get).toHaveBeenCalledWith('/v1/config', {
      params: { reveal: 'true' },
      signal: undefined,
    })
  })

  it('calls PUT /v1/config/:key with value payload', () => {
    configApi.updateParam('SERVER_PORT', '8081')
    expect(put).toHaveBeenCalledWith('/v1/config/SERVER_PORT', {
      value: '8081',
    })
  })

  it('calls DELETE /v1/config/:key when resetting', () => {
    configApi.resetParam('SMTP_HOST')
    expect(del).toHaveBeenCalledWith('/v1/config/SMTP_HOST')
  })

  it('calls PUT /v1/config with bulk settings', () => {
    configApi.updateBulk({ SMTP_HOST: 'smtp.test.corp', SMTP_PORT: '587' })
    expect(put).toHaveBeenCalledWith('/v1/config', {
      settings: { SMTP_HOST: 'smtp.test.corp', SMTP_PORT: '587' },
    })
  })
})

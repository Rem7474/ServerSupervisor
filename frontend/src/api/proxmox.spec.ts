import { describe, it, expect, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}))

vi.mock('./client', () => ({
  api: { get, post, put, delete: del },
}))

import { proxmoxApi } from './proxmox'

describe('proxmoxApi — connection test endpoints', () => {
  it('testProxmoxConnection posts the draft connection payload to the stateless test endpoint', () => {
    const payload = { host: 'pve.example.com', token_id: 'root@pam!test' }
    proxmoxApi.testProxmoxConnection(payload)

    expect(post).toHaveBeenCalledWith('/v1/proxmox/instances/test', payload)
  })

  it('testProxmoxInstanceById re-tests an already-saved connection by id', () => {
    proxmoxApi.testProxmoxInstanceById('conn-42')

    expect(post).toHaveBeenCalledWith('/v1/proxmox/instances/conn-42/test')
  })
})

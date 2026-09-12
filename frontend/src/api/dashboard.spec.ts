import { describe, it, expect, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('./client', () => ({
  api: { get },
}))

import { dashboardApi } from './dashboard'

describe('api/dashboard', () => {
  it('calls /v1/dashboard/attention on getAttention', () => {
    dashboardApi.getAttention()
    expect(get).toHaveBeenCalledWith('/v1/dashboard/attention')
  })

  it('calls /v1/dashboard/init on getDashboardInit', () => {
    dashboardApi.getDashboardInit()
    expect(get).toHaveBeenCalledWith('/v1/dashboard/init')
  })
})

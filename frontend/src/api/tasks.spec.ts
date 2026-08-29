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

import { tasksApi } from './tasks'

describe('tasksApi — runCustomTask', () => {
  it('dispatches a run for the given host/task pair', () => {
    tasksApi.runCustomTask('host-1', 'task-9')

    expect(post).toHaveBeenCalledWith('/v1/hosts/host-1/custom-tasks/task-9/run')
  })
})

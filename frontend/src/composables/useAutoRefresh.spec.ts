import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h, ref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useAutoRefresh } from './useAutoRefresh'

function mountRefresh(tick: () => unknown, options: { enabled?: Ref<boolean>; skip?: () => boolean } = {}) {
  let api!: ReturnType<typeof useAutoRefresh>
  const wrapper = mount(defineComponent({
    setup() {
      api = useAutoRefresh(tick, { intervalSec: 30, ...options })
      return () => h('div')
    },
  }))
  return { api, wrapper }
}

describe('useAutoRefresh', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('ticks every interval while enabled, not on mount', () => {
    const tick = vi.fn()
    mountRefresh(tick)
    expect(tick).not.toHaveBeenCalled()
    vi.advanceTimersByTime(60_000)
    expect(tick).toHaveBeenCalledTimes(2)
  })

  it('pauses while the toggle is off', () => {
    const tick = vi.fn()
    const { api } = mountRefresh(tick)
    api.autoRefresh.value = false
    vi.advanceTimersByTime(60_000)
    expect(tick).not.toHaveBeenCalled()
    api.autoRefresh.value = true
    vi.advanceTimersByTime(30_000)
    expect(tick).toHaveBeenCalledTimes(1)
  })

  it('follows an externally owned toggle', () => {
    const tick = vi.fn()
    const enabled = ref(false)
    const { api } = mountRefresh(tick, { enabled })
    expect(api.autoRefresh).toBe(enabled)
    vi.advanceTimersByTime(30_000)
    expect(tick).not.toHaveBeenCalled()
  })

  it('skips a tick while the skip condition holds', () => {
    const tick = vi.fn()
    let busy = true
    mountRefresh(tick, { skip: () => busy })
    vi.advanceTimersByTime(30_000)
    busy = false
    vi.advanceTimersByTime(30_000)
    expect(tick).toHaveBeenCalledTimes(1)
  })

  it('stops on unmount', () => {
    const tick = vi.fn()
    const { wrapper } = mountRefresh(tick)
    wrapper.unmount()
    vi.advanceTimersByTime(120_000)
    expect(tick).not.toHaveBeenCalled()
  })
})

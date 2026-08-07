import { describe, it, expect, vi, beforeEach } from 'vitest'

const collectCommandOutput = vi.fn()
vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({ collectCommandOutput }),
}))

import { usePendingCommand } from './usePendingCommand'

describe('usePendingCommand', () => {
  beforeEach(() => {
    collectCommandOutput.mockReset()
  })

  it('is pending while the command is in flight, and stops on completion', async () => {
    let resolveOutput: (v: string) => void = () => {}
    collectCommandOutput.mockReturnValue(new Promise<string>((resolve) => { resolveOutput = resolve }))

    const { isPending, track } = usePendingCommand()
    expect(isPending('cmd-1')).toBe(false)

    const promise = track('cmd-1')
    // track() adds to the pending set synchronously, before awaiting the stream.
    expect(isPending('cmd-1')).toBe(true)

    resolveOutput('done')
    await promise

    expect(isPending('cmd-1')).toBe(false)
  })

  it('stops being pending even when the command fails or times out', async () => {
    collectCommandOutput.mockReturnValue(Promise.reject(new Error('failed')))

    const { isPending, track } = usePendingCommand()
    await track('cmd-2')

    expect(isPending('cmd-2')).toBe(false)
  })

  it('is a no-op for a null/undefined command id', async () => {
    const { isPending, track } = usePendingCommand()
    await track(undefined)
    await track(null)

    expect(collectCommandOutput).not.toHaveBeenCalled()
    expect(isPending(undefined)).toBe(false)
  })

  it('tracks multiple in-flight commands independently', async () => {
    let resolveA: (v: string) => void = () => {}
    let resolveB: (v: string) => void = () => {}
    collectCommandOutput.mockImplementation((id: string) =>
      new Promise<string>((resolve) => {
        if (id === 'a') resolveA = resolve
        else resolveB = resolve
      })
    )

    const { isPending, track } = usePendingCommand()
    const pA = track('a')
    const pB = track('b')
    expect(isPending('a')).toBe(true)
    expect(isPending('b')).toBe(true)

    resolveA('a-done')
    await pA
    expect(isPending('a')).toBe(false)
    expect(isPending('b')).toBe(true)

    resolveB('b-done')
    await pB
    expect(isPending('b')).toBe(false)
  })
})

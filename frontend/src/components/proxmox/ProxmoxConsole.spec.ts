import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { openMock, closeMock, resizeMock } = vi.hoisted(() => ({
  openMock: vi.fn(),
  closeMock: vi.fn(),
  resizeMock: vi.fn(),
}))

// status/errorMessage must be real Vue refs (not plain { value } objects)
// for `<script setup>`'s template auto-unwrap to see them as refs, matching
// how the real composable returns them. vi.importActual (rather than a
// top-level `import { ref } from 'vue'`) avoids referencing an outer binding
// from inside the factory, which vi.mock forbids (it's hoisted above
// this file's own imports). useProxmoxConsole always returns the same
// closed-over refs, so calling it once below yields a shared handle the
// tests can drive directly.
vi.mock('../../composables/useProxmoxConsole', async () => {
  const { ref } = await vi.importActual<typeof import('vue')>('vue')
  const status = ref('idle')
  const errorMessage = ref('')
  return {
    useProxmoxConsole: () => ({
      status,
      errorMessage,
      open: openMock,
      resize: resizeMock,
      close: closeMock,
    }),
  }
})

import { useProxmoxConsole } from '../../composables/useProxmoxConsole'

const { status: statusRef, errorMessage: errorRef } = useProxmoxConsole()

const { termOpenMock, termDisposeMock, fitMock } = vi.hoisted(() => ({
  termOpenMock: vi.fn(),
  termDisposeMock: vi.fn(),
  fitMock: vi.fn(),
}))

vi.mock('@xterm/xterm', () => ({
  Terminal: vi.fn().mockImplementation(function TerminalMock() {
    return {
      open: termOpenMock,
      dispose: termDisposeMock,
      loadAddon: vi.fn(),
      cols: 80,
      rows: 24,
    }
  }),
}))

vi.mock('@xterm/addon-fit', () => ({
  FitAddon: vi.fn().mockImplementation(function FitAddonMock() {
    return { fit: fitMock }
  }),
}))

import ProxmoxConsole from './ProxmoxConsole.vue'

class FakeResizeObserver {
  observe(): void {}
  disconnect(): void {}
}

describe('ProxmoxConsole', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    statusRef.value = 'idle'
    errorRef.value = ''
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
  })

  it('does not construct a Terminal while hidden', () => {
    mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: false } })
    expect(termOpenMock).not.toHaveBeenCalled()
    expect(openMock).not.toHaveBeenCalled()
  })

  it('mounts the terminal and opens the console session once shown', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    expect(termOpenMock).toHaveBeenCalledTimes(1)
    expect(fitMock).toHaveBeenCalled()
    expect(openMock).toHaveBeenCalledTimes(1)
    expect(openMock.mock.calls[0][0]).toBe('g1')
    wrapper.unmount()
  })

  it('disposes the terminal and closes the socket when hidden again', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    await wrapper.setProps({ show: false })

    expect(closeMock).toHaveBeenCalled()
    expect(termDisposeMock).toHaveBeenCalledTimes(1)
  })

  it('disposes the terminal on unmount while still shown', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    wrapper.unmount()

    expect(closeMock).toHaveBeenCalled()
    expect(termDisposeMock).toHaveBeenCalledTimes(1)
  })

  it('emits close when the close button is clicked', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    await wrapper.find('.btn-close').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('shows a "Rouvrir" button and reconnects when the session is disconnected', async () => {
    statusRef.value = 'disconnected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    const reopen = wrapper.findAll('button').find((b) => b.text() === 'Rouvrir')
    expect(reopen).toBeTruthy()
    await reopen!.trigger('click')

    expect(openMock).toHaveBeenCalledTimes(2) // once on mount, once on reopen
  })

  it('surfaces the error message from the composable', async () => {
    statusRef.value = 'error'
    errorRef.value = 'console PVE non configurée pour cette connexion'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('console PVE non configurée pour cette connexion')
  })
})

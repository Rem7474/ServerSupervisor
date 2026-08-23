import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { openMock, closeMock, resizeMock, sendInputMock } = vi.hoisted(() => ({
  openMock: vi.fn(),
  closeMock: vi.fn(),
  resizeMock: vi.fn(),
  sendInputMock: vi.fn(),
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
      sendInput: sendInputMock,
      resize: resizeMock,
      close: closeMock,
    }),
  }
})

import { useProxmoxConsole } from '../../composables/useProxmoxConsole'

const { status: statusRef, errorMessage: errorRef } = useProxmoxConsole()

const { termOpenMock, termDisposeMock, termFocusMock, fitMock } = vi.hoisted(() => ({
  termOpenMock: vi.fn(),
  termDisposeMock: vi.fn(),
  termFocusMock: vi.fn(),
  fitMock: vi.fn(),
}))

vi.mock('@xterm/xterm', () => ({
  Terminal: vi.fn().mockImplementation(function TerminalMock() {
    return {
      open: termOpenMock,
      dispose: termDisposeMock,
      focus: termFocusMock,
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
    // fit() is deferred to requestAnimationFrame so a ResizeObserver
    // notification never does layout work synchronously inside its own
    // callback (the fix for the "ResizeObserver loop completed with
    // undelivered notifications" crash) — run it synchronously in tests.
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback): number => {
      cb(0)
      return 0
    })
    // happy-dom does no real layout (offsetWidth/offsetHeight are always 0),
    // but the component intentionally skips fit() on a zero-size container
    // (fitting a hidden panel produces meaningless cols/rows) — stub a
    // real-looking size so that guard doesn't mask fit() being called.
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 800 })
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 400 })
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

  it('a plain show-prop toggle (not the close button) does not close the socket', async () => {
    statusRef.value = 'connected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    await wrapper.setProps({ show: false })

    expect(closeMock).not.toHaveBeenCalled()
    expect(termDisposeMock).not.toHaveBeenCalled()
  })

  it('re-fits without recreating or reconnecting the terminal when re-shown while still connected', async () => {
    statusRef.value = 'connected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()
    fitMock.mockClear()
    openMock.mockClear()

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(termOpenMock).toHaveBeenCalledTimes(1) // still only constructed once
    expect(fitMock).toHaveBeenCalled()
    expect(openMock).not.toHaveBeenCalled() // already connected, no redundant reconnect
  })

  it('reconnects automatically when re-shown while idle/disconnected', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()
    openMock.mockClear()

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(openMock).toHaveBeenCalledTimes(1)
  })

  it('disposes the terminal and closes the socket on unmount', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    wrapper.unmount()

    expect(closeMock).toHaveBeenCalled()
    expect(termDisposeMock).toHaveBeenCalledTimes(1)
  })

  it('closing (X) ends the remote shell, not just hides the panel', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    await wrapper.find('button[title="Fermer"]').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
    expect(closeMock).toHaveBeenCalled()
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

  // --- Mobile touch bar: a soft keyboard has no Ctrl/Esc/Tab/arrows ---

  it('sends the raw control byte for ^C without going through the terminal', async () => {
    statusRef.value = 'connected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    await wrapper.find('button[title="Interrompre (Ctrl+C)"]').trigger('click')

    expect(sendInputMock).toHaveBeenCalledWith('\u0003')
    // Refocused so the soft keyboard doesn't close after every tap.
    expect(termFocusMock).toHaveBeenCalled()
  })

  it('sends the escape sequences for Échap / Tab / arrows', async () => {
    statusRef.value = 'connected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    await wrapper.find('button[title="Échap"]').trigger('click')
    await wrapper.find('button[title="Tabulation (complétion)"]').trigger('click')
    await wrapper.find('button[title="Haut (historique)"]').trigger('click')
    await wrapper.find('button[title="Droite"]').trigger('click')

    expect(sendInputMock.mock.calls.map((c) => c[0])).toEqual(['\u001b', '\t', '\u001b[A', '\u001b[C'])
  })

  it('the sticky Ctrl key rewrites the next typed character into its control code', async () => {
    statusRef.value = 'connected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    const transformInput = openMock.mock.calls[0][2].transformInput as (data: string) => string
    expect(transformInput('c')).toBe('c') // not armed yet

    await wrapper.find('button[title^="Touche Ctrl"]').trigger('click')

    expect(transformInput('c')).toBe('\u0003')
    expect(transformInput('c')).toBe('c') // sticky for exactly one keystroke
  })

  it('leaves input with no control form untouched while Ctrl is armed', async () => {
    statusRef.value = 'connected'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()
    const transformInput = openMock.mock.calls[0][2].transformInput as (data: string) => string

    await wrapper.find('button[title^="Touche Ctrl"]').trigger('click')
    expect(transformInput('ls -la\n')).toBe('ls -la\n') // a paste, not a single key
  })

  it('disables the touch keys while the session is not connected', async () => {
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    expect(wrapper.find('button[title="Interrompre (Ctrl+C)"]').attributes('disabled')).toBeDefined()
  })

  it('surfaces the error message from the composable', async () => {
    statusRef.value = 'error'
    errorRef.value = 'console PVE non configurée pour cette connexion'
    const wrapper = mount(ProxmoxConsole, { props: { guestId: 'g1', guestName: 'web1', show: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('console PVE non configurée pour cette connexion')
  })
})

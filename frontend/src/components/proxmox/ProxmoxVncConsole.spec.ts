import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import { FakeRFB, FakeVncSocket, resetFakeNoVnc } from '../../test/fakeNoVnc'

vi.mock('@novnc/novnc', () => ({ default: FakeRFB }))

import ProxmoxVncConsole from './ProxmoxVncConsole.vue'

enableAutoUnmount(afterEach)

function buttonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find((b) => b.text().includes(text))
}

describe('ProxmoxVncConsole', () => {
  beforeEach(() => {
    setLocale('fr')
    resetFakeNoVnc()
    vi.stubGlobal('WebSocket', FakeVncSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.style.overflow = ''
  })

  async function mountOpen() {
    const wrapper = mount(ProxmoxVncConsole, {
      props: { guestId: 'g1', guestName: 'win11', show: true },
      attachTo: document.body,
    })
    await flushPromises()
    return { wrapper, rfb: FakeRFB.instances[FakeRFB.instances.length - 1], socket: FakeVncSocket.instances[FakeVncSocket.instances.length - 1] }
  }

  it('renders nothing and opens no session while hidden', async () => {
    const wrapper = mount(ProxmoxVncConsole, { props: { guestId: 'g1', show: false } })
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(FakeRFB.instances).toHaveLength(0)
  })

  it('is an accessible modal dialog that mounts noVNC into its screen area', async () => {
    const { wrapper, rfb } = await mountOpen()
    const dialog = wrapper.find('[role="dialog"]')
    expect(dialog.attributes('aria-modal')).toBe('true')
    const title = wrapper.find(`#${dialog.attributes('aria-labelledby')}`)
    expect(title.text()).toContain('Console VNC — win11')
    expect(rfb.target).toBe(wrapper.find('.vnc-screen').element)
    expect(wrapper.find('.vnc-screen').attributes('aria-label')).toBe('Écran de la VM')
    // The page behind must not scroll while the dialog is open.
    expect(document.body.style.overflow).toBe('hidden')
  })

  it('shows the connecting state, then the connected badge with Ctrl+Alt+Suppr enabled', async () => {
    const { wrapper, rfb } = await mountOpen()
    expect(wrapper.text()).toContain("Connexion à l'affichage de la VM…")
    expect(buttonByText(wrapper, 'Ctrl+Alt+Suppr')!.attributes('disabled')).toBeDefined()

    rfb.emit('connect')
    await flushPromises()
    expect(wrapper.find('.vnc-overlay').exists()).toBe(false)
    expect(wrapper.find('.badge').text()).toBe('Connecté')
    const cad = buttonByText(wrapper, 'Ctrl+Alt+Suppr')!
    expect(cad.attributes('disabled')).toBeUndefined()
    await cad.trigger('click')
    expect(rfb.ctrlAltDelCalls).toBe(1)
    expect(rfb.focusCalls).toBe(1)
  })

  it('shows the server\'s translated error and reconnects with a fresh session', async () => {
    const { wrapper, rfb, socket } = await mountOpen()
    expect(buttonByText(wrapper, 'Reconnecter')).toBeUndefined()

    socket.serverClose(4403, 'PROXMOX_CONSOLE_ADMIN_ONLY')
    rfb.emit('disconnect', { clean: false })
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toBe('la console Proxmox est réservée aux administrateurs')
    expect(wrapper.find('.badge').text()).toBe('Erreur')

    await buttonByText(wrapper, 'Reconnecter')!.trigger('click')
    await flushPromises()
    expect(FakeRFB.instances).toHaveLength(2)
    expect(wrapper.text()).toContain("Connexion à l'affichage de la VM…")
  })

  it('tells the user the session ended after a clean disconnect', async () => {
    const { wrapper, rfb } = await mountOpen()
    rfb.emit('connect')
    rfb.emit('disconnect', { clean: true })
    await flushPromises()
    expect(wrapper.text()).toContain('Session VNC terminée.')
    expect(wrapper.find('.badge').text()).toBe('Déconnecté')
    expect(buttonByText(wrapper, 'Reconnecter')).toBeDefined()
  })

  it('ends the session and emits close from the close button, Escape staying a VM key', async () => {
    const { wrapper, rfb } = await mountOpen()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(wrapper.emitted('close')).toBeUndefined()

    await wrapper.find('.btn-close').trigger('click')
    expect(rfb.disconnectCalls).toBe(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('closes the session when the parent hides the dialog, and opens a new one when shown again', async () => {
    const { wrapper, rfb } = await mountOpen()
    await wrapper.setProps({ show: false })
    expect(rfb.disconnectCalls).toBe(1)
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(FakeRFB.instances).toHaveLength(2)
  })

  it('toggles full screen on the dialog content and follows fullscreenchange', async () => {
    const { wrapper, rfb } = await mountOpen()
    const content = wrapper.find('.vnc-content').element as HTMLElement
    let fullscreenElement: Element | null = null
    Object.defineProperty(document, 'fullscreenElement', { configurable: true, get: () => fullscreenElement })
    content.requestFullscreen = vi.fn(async () => {
      fullscreenElement = content
      document.dispatchEvent(new Event('fullscreenchange'))
    })
    document.exitFullscreen = vi.fn(async () => {
      fullscreenElement = null
      document.dispatchEvent(new Event('fullscreenchange'))
    })

    await buttonByText(wrapper, 'Plein écran')!.trigger('click')
    await flushPromises()
    expect(content.requestFullscreen).toHaveBeenCalled()
    const exit = buttonByText(wrapper, 'Quitter le plein écran')!
    expect(exit.attributes('aria-pressed')).toBe('true')
    expect(rfb.focusCalls).toBe(1)

    await exit.trigger('click')
    await flushPromises()
    expect(document.exitFullscreen).toHaveBeenCalled()
    expect(buttonByText(wrapper, 'Plein écran')!.attributes('aria-pressed')).toBe('false')

    // Closing while in full screen leaves it first.
    fullscreenElement = content
    await wrapper.find('.btn-close').trigger('click')
    expect(document.exitFullscreen).toHaveBeenCalledTimes(2)
    Reflect.deleteProperty(document, 'fullscreenElement')
  })

  it('keeps the dialog usable when the browser refuses full screen', async () => {
    const { wrapper } = await mountOpen()
    const content = wrapper.find('.vnc-content').element as HTMLElement
    content.requestFullscreen = vi.fn(async () => { throw new Error('denied') })
    await buttonByText(wrapper, 'Plein écran')!.trigger('click')
    await flushPromises()
    expect(buttonByText(wrapper, 'Plein écran')!.attributes('aria-pressed')).toBe('false')
  })
})

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { FakeRFB, FakeVncSocket, resetFakeNoVnc } from '../test/fakeNoVnc'

const { importFails } = vi.hoisted(() => ({ importFails: { value: false } }))
vi.mock('@novnc/novnc', () => ({
  get default() {
    if (importFails.value) throw new Error('chunk load failed')
    return FakeRFB
  },
}))

import { useProxmoxVnc } from './useProxmoxVnc'

describe('useProxmoxVnc', () => {
  let target: HTMLElement

  beforeEach(() => {
    setLocale('en')
    resetFakeNoVnc()
    importFails.value = false
    vi.stubGlobal('WebSocket', FakeVncSocket)
    target = document.createElement('div')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  async function openSession(guestId = 'g/1') {
    const api = useProxmoxVnc()
    const pending = api.open(guestId, target)
    expect(api.status.value).toBe('connecting')
    await pending
    return { api, rfb: FakeRFB.instances[FakeRFB.instances.length - 1], socket: FakeVncSocket.instances[FakeVncSocket.instances.length - 1] }
  }

  it('hands noVNC its own WebSocket to this app\'s proxy, scaled and shared', async () => {
    const { api, rfb, socket } = await openSession('g/1')
    expect(socket.url).toBe(`ws://${window.location.host}/api/v1/ws/proxmox/vnc/g%2F1`)
    expect(socket.binaryType).toBe('arraybuffer')
    expect(rfb.channel).toBe(socket)
    expect(rfb.target).toBe(target)
    expect(rfb.options).toEqual({ shared: true })
    expect(rfb.scaleViewport).toBe(true)
    expect(rfb.resizeSession).toBe(false)

    rfb.emit('connect')
    expect(api.status.value).toBe('connected')
  })

  it('translates a catalog key from the server\'s close reason and keeps it over noVNC\'s disconnect', async () => {
    const { api, rfb, socket } = await openSession()
    socket.serverClose(4400, 'PROXMOX_CONSOLE_NOT_CONFIGURED')
    rfb.emit('disconnect', { clean: false })
    expect(api.status.value).toBe('error')
    expect(api.errorMessage.value).toBe('PVE console is not configured for this connection: credentials required')
  })

  it('shows a non-catalog close reason (PVE\'s own message) as is, and a generic one when empty', async () => {
    const first = await openSession()
    first.socket.serverClose(4502, 'create vncproxy: HTTP 403: Permission check failed')
    expect(first.api.errorMessage.value).toBe('create vncproxy: HTTP 403: Permission check failed')

    const second = await openSession()
    second.socket.serverClose(4500, '')
    expect(second.api.errorMessage.value).toBe('Could not connect to VNC.')
  })

  it('ignores ordinary close codes and reports noVNC\'s clean/unclean disconnect', async () => {
    const clean = await openSession()
    clean.rfb.emit('connect')
    clean.socket.serverClose(1000)
    clean.rfb.emit('disconnect', { clean: true })
    expect(clean.api.status.value).toBe('disconnected')
    expect(clean.api.errorMessage.value).toBe('')

    const unclean = await openSession()
    unclean.socket.serverClose(1006)
    unclean.rfb.emit('disconnect', { clean: false })
    expect(unclean.api.status.value).toBe('error')
    expect(unclean.api.errorMessage.value).toBe('Could not connect to VNC.')
  })

  it('reports a security failure with noVNC\'s reason or a generic message', async () => {
    const withReason = await openSession()
    withReason.rfb.emit('securityfailure', { reason: 'Authentication failure' })
    expect(withReason.api.errorMessage.value).toBe('Authentication failure')

    const withoutReason = await openSession()
    withoutReason.rfb.emit('securityfailure', {})
    expect(withoutReason.api.errorMessage.value).toBe('Could not connect to VNC.')
  })

  it('sends Ctrl+Alt+Del only while connected, and forwards focus', async () => {
    const { api, rfb } = await openSession()
    api.sendCtrlAltDel()
    expect(rfb.ctrlAltDelCalls).toBe(0)
    rfb.emit('connect')
    api.sendCtrlAltDel()
    api.focus()
    expect(rfb.ctrlAltDelCalls).toBe(1)
    expect(rfb.focusCalls).toBe(1)
  })

  it('disconnects on close and ignores events from the closed session', async () => {
    const { api, rfb, socket } = await openSession()
    api.close()
    expect(rfb.disconnectCalls).toBe(1)
    expect(api.status.value).toBe('idle')

    rfb.emit('connect')
    socket.serverClose(4403, 'PROXMOX_CONSOLE_ADMIN_ONLY')
    rfb.emit('disconnect', { clean: false })
    rfb.emit('securityfailure', { reason: 'x' })
    expect(api.status.value).toBe('idle')
    expect(api.errorMessage.value).toBe('')
  })

  it('reopening replaces the previous session', async () => {
    const { api } = await openSession()
    const first = FakeRFB.instances[0]
    await api.open('g2', target)
    expect(first.disconnectCalls).toBe(1)
    expect(FakeRFB.instances).toHaveLength(2)
  })

  it('drops an open superseded while noVNC was still loading', async () => {
    const api = useProxmoxVnc()
    const pending = api.open('g1', target)
    api.close()
    await pending
    await flushPromises()
    expect(FakeRFB.instances).toHaveLength(0)
    expect(FakeVncSocket.instances).toHaveLength(0)
    expect(api.status.value).toBe('idle')
  })

  it('reports a failed noVNC load', async () => {
    importFails.value = true
    const api = useProxmoxVnc()
    await api.open('g1', target)
    expect(api.status.value).toBe('error')
    expect(api.errorMessage.value).toBe('Could not load the VNC client.')
    expect(FakeVncSocket.instances).toHaveLength(0)
  })

  it('stays silent when a load failure belongs to a superseded open', async () => {
    importFails.value = true
    const api = useProxmoxVnc()
    const pending = api.open('g1', target)
    api.close()
    await pending
    expect(api.status.value).toBe('idle')
  })
})

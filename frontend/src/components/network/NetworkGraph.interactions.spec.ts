import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setLocale } from '../../i18n'

type Handler = (event: unknown) => void
interface FakeNode { id: () => string; position: (arg?: string) => { x: number; y: number } | number; _pos: { x: number; y: number } }

// Fake cytoscape Core that records the registered handlers so tests can fire
// the tap / hover / drag events the component wires up.
const { cytoscapeMock, core, handlers } = vi.hoisted(() => {
  const handlers: { event: string; selector: string; fn: (event: unknown) => void }[] = []
  const core = {
    nodes: vi.fn(),
    on: vi.fn((event: string, selectorOrFn: string | ((e: unknown) => void), fn?: (e: unknown) => void) => {
      if (typeof selectorOrFn === 'function') handlers.push({ event, selector: '', fn: selectorOrFn })
      else handlers.push({ event, selector: selectorOrFn, fn: fn! })
    }),
    destroy: vi.fn(),
    fit: vi.fn(),
    zoom: vi.fn(() => 2),
    width: vi.fn(() => 200),
    height: vi.fn(() => 100),
    resize: vi.fn(),
    elements: vi.fn(() => ({ remove: vi.fn() })),
    add: vi.fn(),
    layout: vi.fn(() => ({ run: vi.fn() })),
  }
  const fn = Object.assign(vi.fn((_opts: unknown) => core), { use: vi.fn() })
  return { cytoscapeMock: fn, core, handlers }
})

vi.mock('cytoscape', () => ({ default: cytoscapeMock }))
vi.mock('cytoscape-fcose', () => ({ default: {} }))

import NetworkGraph from './NetworkGraph.vue'

function makeNode(id: string, pos?: { x: number; y: number }): FakeNode {
  const node = {
    _pos: pos ?? { x: 0, y: 0 },
    id: () => id,
    position: (arg?: string) => {
      if (arg === 'x') return node._pos.x
      if (arg === 'y') return node._pos.y
      return node._pos
    },
  }
  return node as unknown as FakeNode
}

function setNodes(nodes: FakeNode[]) {
  core.nodes.mockReturnValue({ forEach: (cb: (n: FakeNode) => void) => nodes.forEach(cb) })
}

function find(event: string, selector: string): Handler {
  const h = handlers.find((x) => x.event === event && x.selector === selector)
  if (!h) throw new Error(`handler ${event} ${selector} not registered`)
  return h.fn
}

function ev(data: Record<string, unknown>, pageX = 10, pageY = 20) {
  return { target: { data: (key?: string) => (key ? data[key] : data) }, originalEvent: { pageX, pageY } }
}

const baseProps = { data: [{ id: 'h1', name: 'host-1', status: 'online', ports: [] }] }

describe('NetworkGraph — cytoscape interactions', () => {
  beforeEach(() => {
    setLocale('fr')
    handlers.length = 0
    vi.clearAllMocks()
    setNodes([])
    cytoscapeMock.use.mockClear()
  })
  afterEach(() => { vi.useRealTimers() })

  it('emits node-select with a copy of the node data on tap', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    find('tap', 'node')(ev({ id: 'n1', type: 'service' }))
    expect(wrapper.emitted('node-select')![0][0]).toEqual({ id: 'n1', type: 'service' })
  })

  it('emits host-click only when the tapped host node has a hostId', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    const tap = find('tap', 'node[type="host"]')
    tap(ev({ type: 'host', hostId: 'h1' }))
    tap(ev({ type: 'host' }))
    expect(wrapper.emitted('host-click')).toEqual([['h1']])
  })

  it('builds a host tooltip with IP and status, falling back to the unknown status', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    const tooltip = wrapper.find('.cy-tooltip').element as HTMLElement
    find('mouseover', 'node')(ev({ type: 'host', label: 'srv', sublabel: '10.0.0.1', status: 'online' }))
    expect(tooltip.style.display).toBe('block')
    expect(tooltip.textContent).toContain('srv')
    expect(tooltip.textContent).toContain('IP : 10.0.0.1')
    expect(tooltip.textContent).toContain('Statut : online')
    expect(tooltip.style.left).toBe('22px')
    expect(tooltip.style.top).toBe('32px')

    find('mouseover', 'node')(ev({ type: 'host', label: 'srv2' }))
    expect(tooltip.textContent).not.toContain('IP :')
    expect(tooltip.textContent).toContain('Statut : ')
    expect(tooltip.textContent).not.toContain('Statut : undefined')
  })

  it('builds tooltips per node type', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    const tooltip = wrapper.find('.cy-tooltip').element as HTMLElement
    const over = find('mouseover', 'node')

    over(ev({ type: 'proxmox_guest', label: 'vm', sublabel: 'pve1', status: 'running' }))
    expect(tooltip.textContent).toContain('VM Proxmox sans agent')
    expect(tooltip.textContent).toContain('pve1')

    over(ev({ type: 'service', label: 'app', sublabel: 'app.fr', internalPort: 8080, externalPort: 443, tags: 'proxy' }))
    expect(tooltip.textContent).toContain('Port interne : 8080')
    expect(tooltip.textContent).toContain('Port externe : 443')
    expect(tooltip.textContent).toContain('Tags : proxy')

    over(ev({ type: 'service', label: 'app' }))
    expect(tooltip.textContent).toContain('Port interne : -')
    expect(tooltip.textContent).not.toContain('Port externe')
    expect(tooltip.textContent).not.toContain('Tags')

    over(ev({ type: 'port', label: ':80', containers: ['a', 'b'] }))
    expect(tooltip.textContent).toContain('Conteneurs : a, b')
    over(ev({ type: 'port', label: ':81', containers: [] }))
    expect(tooltip.textContent).not.toContain('Conteneurs')

    over(ev({ type: 'root', label: 'root', sublabel: '1.1.1.1' }))
    expect(tooltip.textContent).toContain('IP : 1.1.1.1')

    over(ev({ type: 'authelia', label: 'Auth', sublabel: 'auth.fr' }))
    expect(tooltip.textContent).toContain('auth.fr')
    over(ev({ type: 'internet', label: 'Net' }))
    expect(tooltip.textContent).toBe('Net')
  })

  it('does not show a tooltip for an unknown node type and escapes HTML in labels', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    const tooltip = wrapper.find('.cy-tooltip').element as HTMLElement
    find('mouseover', 'node')(ev({ type: 'mystery', label: 'x' }))
    expect(tooltip.style.display).not.toBe('block')

    find('mouseover', 'node')(ev({ type: 'root', label: '<img src=x onerror=alert(1)>' }))
    expect(tooltip.querySelector('img')).toBeNull()
    expect(tooltip.textContent).toContain('<img')
  })

  it('builds edge tooltips per edge type', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    const tooltip = wrapper.find('.cy-tooltip').element as HTMLElement
    const over = find('mouseover', 'edge')

    over(ev({ edgeType: 'internet-proxy', ports: [80, 443] }))
    expect(tooltip.textContent).toContain('Trafic Internet → Proxy')
    expect(tooltip.textContent).toContain('Ports exposés : :80, :443')
    over(ev({ edgeType: 'internet-proxy' }))
    expect(tooltip.textContent).not.toContain('Ports exposés')

    over(ev({ edgeType: 'proxy-authelia' }))
    expect(tooltip.textContent).toContain('Proxy → Authelia')
    over(ev({ edgeType: 'internet', externalPort: 8443 }))
    expect(tooltip.textContent).toContain('port 8443')
    over(ev({ edgeType: 'proxy' }))
    expect(tooltip.textContent).toContain('Proxy → Service')
    over(ev({ edgeType: 'authelia' }))
    expect(tooltip.textContent).toContain('Authelia → Service')

    tooltip.style.display = 'none'
    over(ev({ edgeType: 'internet' }))
    expect(tooltip.style.display).toBe('none')
  })

  it('moves the tooltip only while visible, and hides it on mouseout', () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    const tooltip = wrapper.find('.cy-tooltip').element as HTMLElement
    const move = find('mousemove', 'node')

    move(ev({}, 100, 200))
    expect(tooltip.style.left).toBe('')

    find('mouseover', 'node')(ev({ type: 'root', label: 'r' }))
    move(ev({}, 100, 200))
    expect(tooltip.style.left).toBe('112px')
    expect(tooltip.style.top).toBe('212px')

    find('mouseout', 'node')(ev({}))
    expect(tooltip.style.display).toBe('none')
    find('mouseout', 'edge')(ev({}))
    expect(tooltip.style.display).toBe('none')
  })

  it('emits rounded node positions after a debounced drag', () => {
    vi.useFakeTimers()
    const wrapper = mount(NetworkGraph, { props: baseProps })
    setNodes([makeNode('a', { x: 10.4, y: 20.6 }), makeNode('b', { x: -3.5, y: 0 })])
    const drag = find('dragfree', 'node')
    drag(ev({}))
    drag(ev({}))
    expect(wrapper.emitted('update:nodePositions')).toBeUndefined()
    vi.advanceTimersByTime(600)
    expect(wrapper.emitted('update:nodePositions')).toHaveLength(1)
    expect(wrapper.emitted('update:nodePositions')![0][0]).toEqual({ a: { x: 10, y: 21 }, b: { x: -3, y: 0 } })
  })

  it('uses the fcose layout without saved positions and the preset layout with them', () => {
    mount(NetworkGraph, { props: baseProps })
    let opts = (cytoscapeMock.mock.calls[0] as unknown[])[0] as { layout: { name: string } }
    expect(opts.layout.name).toBe('fcose')

    cytoscapeMock.mockClear()
    setNodes([makeNode('a'), makeNode('b')])
    const position = vi.fn()
    core.nodes.mockReturnValue({ forEach: (cb: (n: unknown) => void) => {
      cb({ id: () => 'a', position })
      cb({ id: () => 'b', position })
    } })
    mount(NetworkGraph, { props: { ...baseProps, nodePositions: { a: { x: 1, y: 2 } } } })
    opts = (cytoscapeMock.mock.calls[0] as unknown[])[0] as { layout: { name: string } }
    expect(opts.layout.name).toBe('preset')
    expect(position).toHaveBeenCalledTimes(1)
    expect(position).toHaveBeenCalledWith({ x: 1, y: 2 })
    expect(core.fit).toHaveBeenCalledWith(undefined, 48)
  })

  it('re-runs the full auto-layout when data changes without saved positions', async () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    core.layout.mockClear()
    await wrapper.setProps({ data: [...baseProps.data, { id: 'h2', name: 'host-2', status: 'online', ports: [] }] })
    await nextTick()
    expect(core.add).toHaveBeenCalled()
    expect(core.layout).toHaveBeenCalledTimes(1)
    const layoutOpts = (core.layout.mock.calls[0] as unknown[])[0] as Record<string, unknown>
    expect(layoutOpts.name).toBe('fcose')
    expect(layoutOpts.randomize).not.toBe(false)
  })

  it('keeps existing positions on rebuild and only lays out when a node is new', async () => {
    const wrapper = mount(NetworkGraph, { props: { ...baseProps, nodePositions: { known: { x: 5, y: 6 } } } })
    core.layout.mockClear()

    const each = (nodes: unknown[]) => ({ forEach: (cb: (n: unknown) => void) => nodes.forEach(cb) })
    const positioned = vi.fn()

    // snapshot: node known in memory; rebuild: same node back -> no layout
    core.nodes
      .mockReturnValueOnce(each([makeNode('known', { x: 7, y: 8 })]))
      .mockReturnValueOnce(each([{ id: () => 'known', position: positioned }]))
    await wrapper.setProps({ rootLabel: 'renamed' })
    expect(positioned).toHaveBeenCalledWith({ x: 7, y: 8 })
    expect(core.layout).not.toHaveBeenCalled()

    // the rebuilt graph has a node with neither in-memory nor saved position
    core.nodes
      .mockReturnValueOnce(each([]))
      .mockReturnValueOnce(each([{ id: () => 'fresh', position: vi.fn() }]))
    await wrapper.setProps({ rootLabel: 'renamed-again' })
    expect(core.layout).toHaveBeenCalledTimes(1)
    const opts = (core.layout.mock.calls[0] as unknown[])[0] as Record<string, unknown>
    expect(opts.randomize).toBe(false)
    expect(opts.fit).toBe(false)
  })

  it('exposes zoom, fit and reset layout through the control buttons', async () => {
    const wrapper = mount(NetworkGraph, { props: baseProps })
    await wrapper.find('button[title="Zoom +"]').trigger('click')
    expect(core.zoom).toHaveBeenCalledWith({ level: 2 * 1.3, renderedPosition: { x: 100, y: 50 } })
    await wrapper.find('button[title="Zoom −"]').trigger('click')
    expect(core.zoom).toHaveBeenCalledWith({ level: 2 / 1.3, renderedPosition: { x: 100, y: 50 } })
    await wrapper.find('button[title="Ajuster à l\'écran"]').trigger('click')
    expect(core.fit).toHaveBeenCalledWith(undefined, 40)
    core.layout.mockClear()
    await wrapper.find('button[title="Réinitialiser la disposition"]').trigger('click')
    expect(core.layout).toHaveBeenCalledTimes(1)
  })
})

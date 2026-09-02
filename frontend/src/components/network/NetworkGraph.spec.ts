import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'

// cytoscape needs a real 2D canvas context to construct its renderer, which
// happy-dom doesn't provide — mock it with a minimal fake Core exposing only
// the chainable API this component actually calls, so the legend/controls/
// empty-state chrome (the only thing under test here) mounts without error.
const { cytoscapeMock } = vi.hoisted(() => {
  const fakeCore = {
    nodes: vi.fn(() => ({ forEach: vi.fn() })),
    on: vi.fn(),
    destroy: vi.fn(),
    fit: vi.fn(),
    zoom: vi.fn(() => 1),
    width: vi.fn(() => 100),
    height: vi.fn(() => 100),
    elements: vi.fn(() => ({ remove: vi.fn() })),
    add: vi.fn(),
    layout: vi.fn(() => ({ run: vi.fn() })),
  }
  const fn = vi.fn(() => fakeCore) as unknown as { (opts: unknown): typeof fakeCore; use: ReturnType<typeof vi.fn> }
  fn.use = vi.fn()
  return { cytoscapeMock: fn }
})

vi.mock('cytoscape', () => ({ default: cytoscapeMock }))
vi.mock('cytoscape-fcose', () => ({ default: {} }))

import NetworkGraph from './NetworkGraph.vue'

describe('NetworkGraph', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('shows the empty state when there is no data', () => {
    const wrapper = mount(NetworkGraph, { props: { data: [] } })
    expect(wrapper.text()).toContain('Aucune topologie disponible')
  })

  it('renders the legend with the base entries, and hides Authelia/Internet items when unused', () => {
    const wrapper = mount(NetworkGraph, { props: { data: [] } })
    expect(wrapper.text()).toContain('Légende')
    expect(wrapper.text()).toContain('Reverse proxy')
    expect(wrapper.text()).toContain('Hôte')
    expect(wrapper.text()).toContain('En ligne')
    expect(wrapper.text()).toContain('Hors ligne')
    expect(wrapper.text()).not.toContain('Internet → Proxy')
  })

  it('shows the Authelia legend entries with the custom label when a service links to it', () => {
    const wrapper = mount(NetworkGraph, {
      props: {
        data: [],
        services: [{ id: 's1', hostId: 'h1', internalPort: 80, linkToAuthelia: true }],
        autheliaLabel: 'MyAuth',
      },
    })
    expect(wrapper.text()).toContain('Proxy → MyAuth')
    expect(wrapper.text()).toContain('MyAuth → service')
  })

  it('shows the Internet legend entry when a service is exposed to the Internet', () => {
    const wrapper = mount(NetworkGraph, {
      props: {
        data: [],
        services: [{ id: 's1', hostId: 'h1', internalPort: 80, exposedToInternet: true }],
      },
    })
    expect(wrapper.text()).toContain('Internet → Proxy')
  })

  it('exposes the graph control tooltips', () => {
    const wrapper = mount(NetworkGraph, { props: { data: [] } })
    expect(wrapper.find('button[title="Zoom +"]').exists()).toBe(true)
    expect(wrapper.find('button[title="Zoom −"]').exists()).toBe(true)
    expect(wrapper.find('button[title="Ajuster à l\'écran"]').exists()).toBe(true)
    expect(wrapper.find('button[title="Réinitialiser la disposition"]').exists()).toBe(true)
  })
})

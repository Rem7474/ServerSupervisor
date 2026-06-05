import { describe, it, expect } from 'vitest'
import type { ElementDefinition } from 'cytoscape'
import { buildNetworkElements, type BuildNetworkElementsInput } from './buildNetworkElements'

function makeInput(over: Partial<BuildNetworkElementsInput> = {}): BuildNetworkElementsInput {
  return {
    data: [],
    services: [],
    hostPortOverrides: {},
    rootLabel: 'root',
    rootIp: '',
    autheliaLabel: 'Authelia',
    autheliaIp: '',
    internetLabel: 'Internet',
    internetIp: '',
    rootHostId: '',
    autheliaHostId: '',
    rootPortId: '',
    autheliaPortId: '',
    statusColors: { online: '#0f0', offline: '#f00', warning: '#ff0', unknown: '#888' },
    ...over,
  }
}

const nodes = (els: ElementDefinition[]) => els.filter(e => e.group === 'nodes')
const edges = (els: ElementDefinition[]) => els.filter(e => e.group === 'edges')
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const d = (e: ElementDefinition) => e.data as any
const nodeById = (els: ElementDefinition[], id: string) => nodes(els).find(e => d(e).id === id)
const edgeById = (els: ElementDefinition[], id: string) => edges(els).find(e => d(e).id === id)

describe('buildNetworkElements — base structure', () => {
  it('emits only an abstract root node for empty data', () => {
    const els = buildNetworkElements(makeInput())
    expect(nodes(els)).toHaveLength(1)
    expect(d(nodes(els)[0]).id).toBe('root')
    expect(edges(els)).toHaveLength(0)
  })

  it('emits a host node carrying the resolved status colour, no edges for an unlinked port', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', name: 'web', status: 'offline', ports: [{ port: 443, protocol: 'tcp' }] }],
    }))
    const host = nodeById(els, 'host-h1')
    expect(host).toBeTruthy()
    expect(d(host!).statusColor).toBe('#f00')
    expect(nodeById(els, 'port-h1-443-tcp')).toBeTruthy()
    expect(edges(els)).toHaveLength(0)
  })

  it('skips excluded ports and ports already covered by a service', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', ports: [{ port: 22, protocol: 'tcp' }, { port: 3000, protocol: 'tcp' }] }],
      services: [{ id: 's1', hostId: 'h1', name: 'app', internalPort: 3000 }],
      hostPortOverrides: { h1: { excludedPorts: [22] } },
    }))
    expect(nodeById(els, 'port-h1-22-tcp')).toBeFalsy() // excluded
    expect(nodeById(els, 'port-h1-3000-tcp')).toBeFalsy() // covered by service
    expect(nodeById(els, 'svc-h1-s1')).toBeTruthy()
  })
})

describe('buildNetworkElements — routing edges', () => {
  it('links a proxy-linked port from the abstract root', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', ports: [{ port: 443, protocol: 'tcp' }] }],
      hostPortOverrides: { h1: { proxyPorts: new Set([443]) } },
    }))
    const edge = edgeById(els, 'e-proxy-port-h1-443-tcp')
    expect(edge).toBeTruthy()
    expect(d(edge!).source).toBe('root')
    expect(d(edge!).edgeType).toBe('proxy')
  })

  it('creates the Authelia node + proxy→authelia edge for an Authelia-linked port', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', ports: [{ port: 443, protocol: 'tcp' }] }],
      hostPortOverrides: { h1: { autheliaPortNumbers: new Set([443]) } },
    }))
    expect(nodeById(els, 'authelia')).toBeTruthy()
    expect(edgeById(els, 'e-auth-port-h1-443-tcp')).toBeTruthy()
    const link = edgeById(els, 'e-proxy-to-authelia')
    expect(link).toBeTruthy()
    expect(d(link!).source).toBe('root')
    expect(d(link!).target).toBe('authelia')
  })

  it('aggregates internet→proxy exposure into a single labelled edge', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', ports: [{ port: 443, protocol: 'tcp' }, { port: 8080, protocol: 'tcp' }] }],
      hostPortOverrides: { h1: {
        proxyPorts: new Set([443, 8080]),
        internetExposedPorts: { 443: 8443, 8080: 80 },
      } },
    }))
    expect(nodeById(els, 'internet')).toBeTruthy()
    const agg = edgeById(els, 'e-inet-to-proxy')
    expect(agg).toBeTruthy()
    expect(d(agg!).source).toBe('internet')
    expect(d(agg!).target).toBe('root')
    expect(d(agg!).ports).toEqual([80, 8443]) // sorted, unique
  })

  it('uses a direct internet edge when a port is exposed without the proxy', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', ports: [{ port: 8080, protocol: 'tcp' }] }],
      hostPortOverrides: { h1: { internetExposedPorts: { 8080: null } } },
    }))
    const edge = edgeById(els, 'e-inet-port-h1-8080-tcp')
    expect(edge).toBeTruthy()
    expect(d(edge!).source).toBe('internet')
    expect(d(edge!).edgeType).toBe('internet')
  })

  it('links a proxy-linked service node', () => {
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1' }],
      services: [{ id: 's1', hostId: 'h1', name: 'web', internalPort: 3000, linkToProxy: true }],
    }))
    expect(nodeById(els, 'svc-h1-s1')).toBeTruthy()
    const edge = edgeById(els, 'e-proxy-svc-h1-s1')
    expect(edge).toBeTruthy()
    expect(d(edge!).source).toBe('root')
  })
})

describe('buildNetworkElements — robustness (phantom edges)', () => {
  it('drops edges whose proxy node was never emitted (pinned port excluded)', () => {
    // h1 is the proxy, pinned on port 80 — but port 80 is excluded, so the
    // port-h1-80-tcp node is never created. The proxy edge for port 443 would
    // otherwise reference a non-existent source and crash Cytoscape.
    const els = buildNetworkElements(makeInput({
      data: [{ id: 'h1', ports: [{ port: 80, protocol: 'tcp' }, { port: 443, protocol: 'tcp' }] }],
      hostPortOverrides: { h1: { excludedPorts: [80], proxyPorts: new Set([443]) } },
      rootHostId: 'h1',
      rootPortId: '80-tcp',
    }))

    // The pinned proxy node does not exist...
    expect(nodeById(els, 'port-h1-80-tcp')).toBeFalsy()
    // ...so no edge may reference it.
    const dangling = edges(els).filter(e => d(e).source === 'port-h1-80-tcp' || d(e).target === 'port-h1-80-tcp')
    expect(dangling).toHaveLength(0)
    // every remaining edge points at real nodes
    const nodeIds = new Set(nodes(els).map(e => d(e).id))
    for (const e of edges(els)) {
      expect(nodeIds.has(d(e).source)).toBe(true)
      expect(nodeIds.has(d(e).target)).toBe(true)
    }
  })
})

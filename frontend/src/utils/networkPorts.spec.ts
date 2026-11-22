import { describe, it, expect } from 'vitest'
import {
  buildDiscoveredPortsByHost, createDefaultPortSetting, ensureHostPortConfig, getHostPortEntry,
  type HostPortEntry,
} from './networkPorts'

describe('buildDiscoveredPortsByHost', () => {
  it('prefers the host port, merges containers sharing a port and flags container-only ports', () => {
    const map = buildDiscoveredPortsByHost([
      { host_id: 'h1', name: 'web', port_mappings: [{ host_port: 8080, container_port: 80, protocol: 'TCP' }] },
      { host_id: 'h1', name: 'web-2', port_mappings: [{ host_port: 8080, container_port: 80 }] },
      { host_id: 'h1', name: 'db', port_mappings: [{ container_port: 5432 }] },
      { host_id: 'h1', name: 'dns', port_mappings: [{ host_port: 53, protocol: 'udp' }] },
    ], [{ id: 'h1' }])

    expect(map.h1).toEqual([
      { key: '8080-tcp', port: 8080, protocol: 'tcp', internal: false, containers: ['web', 'web-2'] },
      { key: '5432-tcp', port: 5432, protocol: 'tcp', internal: true, containers: ['db'] },
      { key: '53-udp', port: 53, protocol: 'udp', internal: false, containers: ['dns'] },
    ])
  })

  it('keeps the same port number apart per protocol', () => {
    const map = buildDiscoveredPortsByHost([
      { host_id: 'h1', name: 'a', port_mappings: [{ host_port: 53, protocol: 'tcp' }, { host_port: 53, protocol: 'udp' }] },
    ], [])
    expect(map.h1.map((p) => p.key)).toEqual(['53-tcp', '53-udp'])
  })

  it('skips mappings without a port or container without host, and lists every host', () => {
    const map = buildDiscoveredPortsByHost([
      { host_id: '', name: 'orphan', port_mappings: [{ host_port: 80 }] },
      { host_id: 'h1', name: 'x', port_mappings: [{}] },
    ], [{ id: 'h1' }, { id: 'h2' }])
    expect(map).toEqual({ h1: [], h2: [] })
  })
})

describe('ensureHostPortConfig', () => {
  it('adds missing hosts and ports without overwriting existing settings', () => {
    const config: HostPortEntry[] = [
      { hostId: 'h1', ports: { 80: { ...createDefaultPortSetting(), name: 'site', enabled: false } } },
    ]
    ensureHostPortConfig(config, [{ id: 'h1' }, { id: 'h2' }], {
      h1: [{ key: '80-tcp', port: 80, protocol: 'tcp', internal: false, containers: [] },
        { key: '443-tcp', port: 443, protocol: 'tcp', internal: false, containers: [] }],
    })

    expect(config.map((e) => e.hostId)).toEqual(['h1', 'h2'])
    expect(config[0].ports['80']).toMatchObject({ name: 'site', enabled: false })
    expect(config[0].ports['443']).toEqual(createDefaultPortSetting())
    expect(config[1].ports).toEqual({})
  })

  it('getHostPortEntry repairs an entry saved without a ports map', () => {
    const config = [{ hostId: 'h1' } as unknown as HostPortEntry]
    expect(getHostPortEntry(config, 'h1').ports).toEqual({})
    expect(config).toHaveLength(1)
  })
})

// Port discovery and per-host port configuration for the network topology
// (useNetwork.ts and NetworkTopologyConfig.vue both edit the same
// hostPortConfig array, shared through v-model).

/** User settings for one port of one host, as saved in the topology config. */
export interface PortSetting {
  name: string
  domain: string
  path: string
  enabled: boolean
  linkToProxy: boolean
  linkToAuthelia: boolean
  exposedToInternet: boolean
  externalPort: number | null
}

export interface HostPortEntry {
  hostId: string
  ports: Record<string, PortSetting>
}

/** A port seen on a host's containers, deduplicated by number + protocol. */
export interface DiscoveredPort {
  key: string
  port: number
  protocol: string
  /** Only published inside the container (no host port). */
  internal: boolean
  containers: string[]
}

interface PortMappingLike {
  host_port?: number | null
  container_port?: number | null
  protocol?: string | null
}

interface ContainerLike {
  host_id?: string | null
  name?: string | null
  port_mappings?: PortMappingLike[] | null
}

/**
 * Groups container port mappings by host. A host port is preferred over the
 * container port; the same port/protocol published by several containers is
 * listed once with every container name. Every known host gets an entry,
 * empty if nothing is published.
 */
export function buildDiscoveredPortsByHost(
  containers: ContainerLike[],
  hosts: { id: string }[],
): Record<string, DiscoveredPort[]> {
  const map: Record<string, DiscoveredPort[]> = {}
  for (const container of containers) {
    const hostId = container.host_id
    if (!hostId) continue
    for (const mapping of container.port_mappings || []) {
      const hostPort = mapping.host_port || 0
      const portNumber = hostPort || mapping.container_port || 0
      if (!portNumber) continue
      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      const key = `${portNumber}-${protocol}`
      const list = (map[hostId] ||= [])
      const existing = list.find((entry) => entry.key === key)
      if (existing) {
        if (container.name && !existing.containers.includes(container.name)) existing.containers.push(container.name)
        continue
      }
      list.push({ key, port: portNumber, protocol, internal: hostPort === 0, containers: container.name ? [container.name] : [] })
    }
  }
  for (const host of hosts) {
    map[host.id] ||= []
  }
  return map
}

export function createDefaultPortSetting(): PortSetting {
  return { name: '', domain: '', path: '/', enabled: true, linkToProxy: false, linkToAuthelia: false, exposedToInternet: false, externalPort: null }
}

/** The config entry for a host, created (and appended) if missing. */
export function getHostPortEntry(config: HostPortEntry[], hostId: string): HostPortEntry {
  let entry = config.find((item) => item.hostId === hostId)
  if (!entry) {
    entry = { hostId, ports: {} }
    config.push(entry)
  }
  if (!entry.ports) entry.ports = {}
  return entry
}

/**
 * Makes sure every host and every discovered port has a config entry,
 * without touching settings the user already has.
 */
export function ensureHostPortConfig(
  config: HostPortEntry[],
  hosts: { id: string }[],
  discovered: Record<string, DiscoveredPort[]>,
): void {
  for (const host of hosts) getHostPortEntry(config, host.id)
  for (const [hostId, ports] of Object.entries(discovered)) {
    const entry = getHostPortEntry(config, hostId)
    for (const port of ports) {
      entry.ports[String(port.port)] ||= createDefaultPortSetting()
    }
  }
}

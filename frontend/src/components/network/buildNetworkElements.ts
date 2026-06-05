import type { ElementDefinition } from 'cytoscape'

export interface HostPort {
  port: number | string
  protocol?: string
  containers?: string[]
}

export interface NetworkHost {
  id: string
  name?: string
  hostname?: string
  ip_address?: string
  status?: string
  ports?: HostPort[]
}

export interface NetworkService {
  id: string | number
  hostId?: string
  name?: string
  domain?: string
  path?: string
  internalPort?: number | string
  externalPort?: number | null
  linkToProxy?: boolean
  linkToAuthelia?: boolean
  exposedToInternet?: boolean
  tags?: string
}

export interface HostPortOverride {
  excludedPorts?: number[]
  portMap?: Record<number, string>
  proxyPorts?: Set<number>
  autheliaPortNumbers?: Set<number>
  internetExposedPorts?: Record<number, number | null>
}

export interface BuildNetworkElementsInput {
  data: NetworkHost[]
  services: NetworkService[]
  hostPortOverrides: Record<string, HostPortOverride>
  rootLabel: string
  rootIp: string
  autheliaLabel: string
  autheliaIp: string
  internetLabel: string
  internetIp: string
  rootHostId: string
  autheliaHostId: string
  rootPortId: string
  autheliaPortId: string
  /** status -> hex colour map (canvas can't read CSS vars). */
  statusColors: Record<string, string>
}

/**
 * buildNetworkElements turns the topology props into a flat Cytoscape element
 * list (compound host nodes with port/service children, plus proxy / Authelia /
 * Internet routing edges). It is a pure function so the routing logic can be
 * unit-tested in isolation.
 *
 * Edges referencing a node that was not emitted (e.g. a pinned proxy/Authelia
 * port that turned out excluded or covered by a service) are dropped at the end,
 * since Cytoscape throws on an edge with a non-existent source/target.
 */
export function buildNetworkElements(input: BuildNetworkElementsInput): ElementDefinition[] {
  const elements: ElementDefinition[] = []
  const statusColors = input.statusColors

  // === Root node (abstract, only when not linked to a real host) ===
  const rootLabel = input.rootLabel || 'Infrastructure'
  if (!input.rootHostId) {
    elements.push({
      group: 'nodes',
      data: { id: 'root', label: rootLabel, sublabel: input.rootIp || '', type: 'root' },
    })
  }

  // === Services by host ===
  const servicesByHost = new Map<string, NetworkService[]>()
  for (const svc of input.services || []) {
    if (!svc?.hostId) continue
    if (!servicesByHost.has(svc.hostId)) servicesByHost.set(svc.hostId, [])
    servicesByHost.get(svc.hostId)!.push(svc)
  }

  // Resolve effective node IDs: specific port > host > abstract node
  const proxyNodeId = input.rootHostId
    ? (input.rootPortId ? `port-${input.rootHostId}-${input.rootPortId}` : `host-${input.rootHostId}`)
    : 'root'
  const autheliaNodeId = input.autheliaHostId
    ? (input.autheliaPortId ? `port-${input.autheliaHostId}-${input.autheliaPortId}` : `host-${input.autheliaHostId}`)
    : 'authelia'

  // Collect external ports exposed via proxy (for the single aggregated internet→proxy edge)
  const internetViaProxyPorts: number[] = []

  // === Host nodes (compound parents) ===
  for (const host of input.data) {
    const hostStatus = host.status || 'unknown'
    const statusColor = statusColors[hostStatus] || statusColors.unknown
    // Role goes to the host node only when no specific port is pinned
    const role = (host.id === input.rootHostId && !input.rootPortId) ? 'proxy'
      : (host.id === input.autheliaHostId && !input.autheliaPortId) ? 'authelia'
        : null
    elements.push({
      group: 'nodes',
      data: {
        id: `host-${host.id}`,
        label: host.name || host.hostname || host.id,
        sublabel: host.ip_address || '',
        type: 'host',
        status: hostStatus,
        statusColor,
        hostId: host.id,
        ...(role ? { role } : {}),
      },
    })

    const override: HostPortOverride = input.hostPortOverrides?.[host.id] || {}
    const hostExcluded = new Set((override.excludedPorts || []).map(Number).filter(Boolean))
    const portMap: Record<number, string> = override.portMap || {}
    const proxyPorts: Set<number> = override.proxyPorts || new Set<number>()
    const autheliaPortNumbers: Set<number> = override.autheliaPortNumbers || new Set<number>()
    const internetExposedPorts: Record<number, number | null> = override.internetExposedPorts || {}

    // Port nodes
    const rawPorts = host.ports || []
    for (const port of rawPorts) {
      const portNumber = Number(port.port || 0)
      if (!portNumber || hostExcluded.has(portNumber)) continue

      // Skip if a service already covers this port
      const hostServices = servicesByHost.get(host.id) || []
      if (hostServices.some(s => Number(s.internalPort) === portNumber)) continue

      const protocol = (port.protocol || 'tcp').toLowerCase()
      const serviceName = portMap?.[portNumber]
      const label = serviceName
        ? `${serviceName}\n${portNumber}/${protocol.toUpperCase()}`
        : `${portNumber}/${protocol.toUpperCase()}`

      const isProxyLinked = proxyPorts.has(portNumber)
      const isAutheliaLinked = autheliaPortNumbers.has(portNumber)
      const isInternetExposed = portNumber in internetExposedPorts
      const externalPort = internetExposedPorts[portNumber] || null

      const nodeId = `port-${host.id}-${portNumber}-${protocol}`
      const portKey = `${portNumber}-${protocol}`
      const portRole = (host.id === input.rootHostId && portKey === input.rootPortId) ? 'proxy'
        : (host.id === input.autheliaHostId && portKey === input.autheliaPortId) ? 'authelia'
          : null
      elements.push({
        group: 'nodes',
        data: {
          id: nodeId,
          label,
          type: 'port',
          parent: `host-${host.id}`,
          protocol,
          portNumber,
          hostId: host.id,
          isProxyLinked,
          isAutheliaLinked,
          isInternetExposed,
          externalPort,
          containers: port.containers || [],
          ...(portRole ? { role: portRole } : {}),
        },
      })

      // proxy → service (direct, only when not going through Authelia)
      if (isProxyLinked && !isAutheliaLinked) {
        elements.push({ group: 'edges', data: { id: `e-proxy-${nodeId}`, source: proxyNodeId, target: nodeId, edgeType: 'proxy' } })
      }
      // authelia → service (Authelia routes to the service)
      if (isAutheliaLinked) {
        elements.push({ group: 'edges', data: { id: `e-auth-${nodeId}`, source: autheliaNodeId, target: nodeId, edgeType: 'authelia' } })
      }
      // internet routing: via proxy (aggregated) or direct
      if (isInternetExposed && isProxyLinked) {
        internetViaProxyPorts.push(externalPort || portNumber)
      } else if (isInternetExposed && !isProxyLinked) {
        elements.push({ group: 'edges', data: { id: `e-inet-${nodeId}`, source: 'internet', target: nodeId, edgeType: 'internet', externalPort } })
      }
    }

    // Service nodes
    for (const svc of servicesByHost.get(host.id) || []) {
      const internalPort = Number(svc.internalPort || 0)
      const domain = svc.domain || ''
      const path = svc.path || '/'
      const sublabel = domain ? `${domain}${path}` : path

      const nodeId = `svc-${host.id}-${svc.id}`
      elements.push({
        group: 'nodes',
        data: {
          id: nodeId,
          label: svc.name || 'Service',
          sublabel,
          type: 'service',
          parent: `host-${host.id}`,
          hostId: host.id,
          internalPort,
          externalPort: svc.externalPort || null,
          isProxyLinked: svc.linkToProxy || false,
          isAutheliaLinked: svc.linkToAuthelia || false,
          isInternetExposed: svc.exposedToInternet || false,
          tags: svc.tags || '',
        },
      })

      // proxy → service (direct, only when not going through Authelia)
      if (svc.linkToProxy && !svc.linkToAuthelia) {
        elements.push({ group: 'edges', data: { id: `e-proxy-${nodeId}`, source: proxyNodeId, target: nodeId, edgeType: 'proxy' } })
      }
      // authelia → service
      if (svc.linkToAuthelia) {
        elements.push({ group: 'edges', data: { id: `e-auth-${nodeId}`, source: autheliaNodeId, target: nodeId, edgeType: 'authelia' } })
      }
      // internet routing: via proxy (aggregated) or direct
      if (svc.exposedToInternet && svc.linkToProxy) {
        internetViaProxyPorts.push(svc.externalPort || 443)
      } else if (svc.exposedToInternet && !svc.linkToProxy) {
        elements.push({ group: 'edges', data: { id: `e-inet-${nodeId}`, source: 'internet', target: nodeId, edgeType: 'internet', externalPort: svc.externalPort } })
      }
    }
  }

  // === Authelia node (only if abstract, i.e. not linked to a real host) ===
  const needsAuthelia = elements.some(e => e.group === 'edges' && (e.data.edgeType === 'authelia' || e.data.edgeType === 'proxy-authelia'))
  if (needsAuthelia) {
    if (!input.autheliaHostId && input.autheliaLabel) {
      elements.push({
        group: 'nodes',
        data: { id: 'authelia', label: input.autheliaLabel || 'Authelia', sublabel: input.autheliaIp || '', type: 'authelia' },
      })
    }
    // Single proxy→authelia edge (between the two real IDs, whatever they resolve to)
    const autheliaEdgeExists = elements.some(e => e.data.id === 'e-proxy-to-authelia')
    if (!autheliaEdgeExists) {
      elements.push({
        group: 'edges',
        data: { id: 'e-proxy-to-authelia', source: proxyNodeId, target: autheliaNodeId, edgeType: 'proxy-authelia' },
      })
    }
  }

  // === Internet node + aggregated internet→proxy edge ===
  const hasDirectInternetEdges = elements.some(e => e.group === 'edges' && e.data.edgeType === 'internet')
  const needsInternet = hasDirectInternetEdges || internetViaProxyPorts.length > 0
  if (needsInternet) {
    elements.push({
      group: 'nodes',
      data: { id: 'internet', label: input.internetLabel || 'Internet', sublabel: input.internetIp || '', type: 'internet' },
    })
    // Single aggregated internet→proxy edge
    if (internetViaProxyPorts.length > 0) {
      const uniquePorts = [...new Set(internetViaProxyPorts)].sort((a, b) => a - b)
      const label = uniquePorts.map(p => `:${p}`).join('  ')
      elements.push({
        group: 'edges',
        data: { id: 'e-inet-to-proxy', source: 'internet', target: proxyNodeId, edgeType: 'internet-proxy', label, ports: uniquePorts },
      })
    }
  }

  // === Drop edges referencing a non-existent node (robustness) ===
  const nodeIds = new Set<string>()
  for (const e of elements) {
    if (e.group === 'nodes' && e.data.id) nodeIds.add(String(e.data.id))
  }
  return elements.filter((e) => {
    if (e.group !== 'edges') return true
    const d = e.data as { source?: string, target?: string }
    return !!d.source && !!d.target && nodeIds.has(d.source) && nodeIds.has(d.target)
  })
}

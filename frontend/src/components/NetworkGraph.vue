<template>
  <div class="d3-graph-container">
    <div class="graph-legend">
      <div class="legend-title">Légende</div>
      <div class="legend-item">
        <span style="display:inline-block; width:20px; height:12px; border:1.5px solid rgba(148,163,184,0.4); border-radius:3px; background:rgba(15,23,42,0.5); flex-shrink:0;"></span>
        Hôte
      </div>
      <div class="legend-item">
        <span class="legend-dot" style="background:#22c55e;"></span>
        En ligne
      </div>
      <div class="legend-item">
        <span class="legend-dot" style="background:#ef4444;"></span>
        Hors ligne
      </div>
      <div class="legend-item">
        <span class="legend-dot service-node"></span>
        Service proxy
      </div>
      <div class="legend-item">
        <span class="legend-dot port-tcp"></span>
        Port TCP
      </div>
      <div class="legend-item">
        <span class="legend-dot port-udp"></span>
        Port UDP
      </div>
      <div v-if="autheliaLabel" class="legend-item">
        <span style="display:inline-block; width:18px; height:3px; background:#8b5cf6; border-radius:2px; flex-shrink:0;"></span>
        {{ autheliaLabel }}
      </div>
      <div v-if="internetLabel" class="legend-item">
        <span style="display:inline-block; width:18px; height:3px; background:#fb923c; border-radius:2px; flex-shrink:0;"></span>
        {{ internetLabel }}
      </div>
    </div>
    <div v-if="!hasData" class="graph-empty">
      <div class="empty-title">Aucune topologie disponible</div>
      <div class="empty-subtitle">Les hotes actifs apparaitront ici des que les donnees remontent.</div>
    </div>
    <div ref="tooltipRef" class="d3-tooltip"></div>
    <svg ref="svgRef" class="d3-graph"></svg>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import * as d3 from 'd3'

const props = defineProps({
  data: {
    type: Array, // Array of host objects with containers
    required: true
  },
  rootLabel: {
    type: String,
    default: 'root'
  },
  rootIp: {
    type: String,
    default: ''
  },
  serviceMap: {
    type: Object,
    default: () => ({})
  },
  excludedPorts: {
    type: Array,
    default: () => []
  },
  services: {
    type: Array,
    default: () => []
  },
  hostPortOverrides: {
    type: Object,
    default: () => ({})
  },
  autheliaLabel: {
    type: String,
    default: 'Authelia'
  },
  autheliaIp: {
    type: String,
    default: ''
  },
  internetLabel: {
    type: String,
    default: 'Internet'
  },
  internetIp: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['host-click'])

const svgRef = ref(null)
const tooltipRef = ref(null)
const hasData = computed(() => Array.isArray(props.data) && props.data.length > 0)

function escapeHtml(str) {
  if (str == null) return ''
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}
const statusColors = {
  online: '#22c55e',
  warning: '#f59e0b',
  offline: '#ef4444',
  unknown: '#94a3b8'
}

const protocolColors = {
  tcp: '#60a5fa',
  udp: '#fb923c',
  other: '#34d399'
}

const hierarchyRoot = computed(() => {
  const globalExcluded = (props.excludedPorts || []).map(value => Number(value)).filter(Boolean)
  const servicesByHost = new Map()
  for (const service of props.services || []) {
    if (!service?.hostId) continue
    if (!servicesByHost.has(service.hostId)) servicesByHost.set(service.hostId, [])
    servicesByHost.get(service.hostId).push(service)
  }

  // Hiérarchie plate : root → services/ports (l'hôte n'est plus un nœud D3)
  const rootChildren = []

  for (const host of props.data) {
    const hostOverride = props.hostPortOverrides?.[host.id]
    const hostExcluded = new Set([
      ...globalExcluded,
      ...(hostOverride?.excludedPorts || [])
    ].map(value => Number(value)).filter(Boolean))
    const hostPortMap = hostOverride?.portMap || {}
    const proxyPorts = hostOverride?.proxyPorts || new Set()
    const autheliaPortNumbers = hostOverride?.autheliaPortNumbers || new Set()
    const internetExposedPorts = hostOverride?.internetExposedPorts || {}
    const rawPorts = host.ports || []
    const filteredPorts = rawPorts.filter(port => {
      const portNumber = Number(port.port || 0)
      return portNumber && !hostExcluded.has(portNumber)
    })
    const hostServices = servicesByHost.get(host.id) || []

    // Métadonnées hôte portées directement par chaque nœud
    const hostMeta = {
      hostId: host.id,
      hostName: host.name || host.hostname || host.id,
      hostIp: host.ip_address || '',
      hostStatus: host.status || 'unknown'
    }

    // Services liés au proxy
    for (const service of hostServices) {
      const internalPort = Number(service.internalPort || 0)
      const externalPort = Number(service.externalPort || 0)
      const path = service.path || '/'
      const domain = service.domain || ''
      const name = service.name || 'Service'
      const label = domain ? `${domain}${path}` : path
      rootChildren.push({
        id: `service-${host.id}-${service.id}`,
        name,
        subtitle: label,
        internalPort,
        externalPort,
        type: 'service',
        protocol: 'tcp',
        port: internalPort,
        tags: service.tags || '',
        isProxyLinked: service.linkToProxy || false,
        isAutheliaLinked: service.linkToAuthelia || autheliaPortNumbers.has(Number(internalPort)),
        isInternetExposed: service.exposedToInternet || (internalPort in internetExposedPorts),
        externalPort: service.externalPort || internetExposedPorts[internalPort] || null,
        ...hostMeta
      })
    }

    // Ports non représentés comme service
    for (const port of filteredPorts) {
      const portNumber = Number(port.port || 0)
      if (hostServices.some(s => Number(s.internalPort) === portNumber)) continue
      const protocol = (port.protocol || 'tcp').toLowerCase()
      const serviceName = hostPortMap?.[portNumber] || props.serviceMap?.[portNumber]
      const label = serviceName
        ? `${serviceName} (${portNumber}/${protocol.toUpperCase()})`
        : `${portNumber}/${protocol.toUpperCase()}`
      rootChildren.push({
        id: `port-${host.id}-${port.port}-${protocol}`,
        name: label,
        type: 'port',
        protocol,
        port: portNumber,
        containers: port.containers || [],
        isProxyLinked: proxyPorts.has(portNumber),
        isAutheliaLinked: autheliaPortNumbers.has(portNumber),
        isInternetExposed: portNumber in internetExposedPorts,
        externalPort: internetExposedPorts[portNumber] || null,
        ...hostMeta
      })
    }
  }

  return d3.hierarchy({
    id: 'root',
    name: props.rootLabel || 'root',
    type: 'root',
    children: rootChildren
  })
})

const render = () => {
  if (!svgRef.value) return

  const width = svgRef.value.clientWidth || 1000
  const height = svgRef.value.clientHeight || 600

  d3.select(svgRef.value).selectAll('*').remove()
  if (!hasData.value) return

  const root = hierarchyRoot.value
  if (!root || !root.children || root.children.length === 0) return

  const svg = d3.select(svgRef.value)
    .attr('width', width)
    .attr('height', height)

  const defs = svg.append('defs')
  defs.append('filter')
    .attr('id', 'node-shadow')
    .append('feDropShadow')
    .attr('dx', 0)
    .attr('dy', 10)
    .attr('stdDeviation', 8)
    .attr('flood-opacity', 0.45)

  const g = svg.append('g').attr('class', 'graph-root')

  const zoom = d3.zoom()
    .scaleExtent([0.6, 2.2])
    .on('zoom', (event) => {
      g.attr('transform', event.transform)
    })

  svg.call(zoom)

  const treeLayout = d3.tree().size([height - 100, width - 260])
  treeLayout.separation((a, b) => (a.parent === b.parent ? 1.2 : 2))

  const treeData = treeLayout(root)

  const clusterGroup = g.append('g').attr('class', 'service-clusters')
  const autheliaLinkGroup = g.append('g').attr('class', 'authelia-links')
  const internetLinkGroup = g.append('g').attr('class', 'internet-links')
  const linkGroup = g.append('g').attr('class', 'links')
  const nodeGroup = g.append('g').attr('class', 'nodes')
  const specialNodeGroup = g.append('g').attr('class', 'special-nodes')

  // Centrage du proxy (root) avec marge pour Internet
  const centerX = width * 0.55;

  // Tous les nœuds feuilles (hiérarchie plate : root → service/port)
  const allLeafNodes = treeData.descendants().filter(
    d => d.data.type === 'service' || d.data.type === 'port'
  )
  // Infos hôte depuis props.data (plus de nœud hôte dans l'arbre D3)
  const hostInfoById = new Map(props.data.map(h => [
    h.id,
    { name: h.name || h.hostname || h.id, ip: h.ip_address || '', status: h.status || 'unknown' }
  ]))
  const clustersByHost = d3.group(allLeafNodes, d => d.data.hostId)

  // Render host clusters
  for (const [hostId, nodes] of clustersByHost.entries()) {
    if (nodes.length === 0) continue
    const hostInfo = hostInfoById.get(hostId) || { name: hostId, ip: '', status: 'unknown' }
    const hostStatus = hostInfo.status
    const hostName = hostInfo.name
    const hostIp = hostInfo.ip
    const statusColor = statusColors[hostStatus] || statusColors.unknown

    const positions = nodes.map(node => ({
      x: node.y + centerX,
      y: node.x + 40
    }))
    const minX = d3.min(positions, pos => pos.x) ?? 0
    const maxX = d3.max(positions, pos => pos.x) ?? 0
    const minY = d3.min(positions, pos => pos.y) ?? 0
    const maxY = d3.max(positions, pos => pos.y) ?? 0

    const headerH = 32
    const clusterPadding = { x: 28, y: 16 }

    const rectX = minX - 130 - clusterPadding.x
    const rectY = minY - 22 - clusterPadding.y - headerH
    const rectW = (maxX - minX) + 260 + clusterPadding.x * 2
    const rectH = (maxY - minY) + 44 + clusterPadding.y * 2 + headerH

    const cluster = clusterGroup.append('g')
      .attr('class', 'service-cluster')
      .style('cursor', 'pointer')
      .on('click', () => emit('host-click', hostId))
      .on('mouseover', (event) => {
        if (!tooltipRef.value) return
        const lines = [
          hostName,
          hostIp ? `IP : ${hostIp}` : null,
          `Statut : ${hostStatus}`,
          `${nodes.length} service(s)`
        ].filter(Boolean)
        tooltipRef.value.innerHTML = lines.map(l => `<div>${escapeHtml(l)}</div>`).join('')
        tooltipRef.value.style.display = 'block'
        tooltipRef.value.style.left = `${event.pageX + 8}px`
        tooltipRef.value.style.top = `${event.pageY - 48}px`
      })
      .on('mousemove', (event) => {
        if (!tooltipRef.value || tooltipRef.value.style.display !== 'block') return
        tooltipRef.value.style.left = `${event.pageX + 8}px`
        tooltipRef.value.style.top = `${event.pageY - 48}px`
      })
      .on('mouseout', () => {
        if (tooltipRef.value) tooltipRef.value.style.display = 'none'
      })

    // Rectangle principal du cluster
    cluster.append('rect')
      .attr('x', rectX)
      .attr('y', rectY)
      .attr('width', rectW)
      .attr('height', rectH)
      .attr('rx', 14)
      .attr('ry', 14)

    // Ligne de séparation en-tête / contenu
    cluster.append('line')
      .attr('x1', rectX + 12)
      .attr('y1', rectY + headerH)
      .attr('x2', rectX + rectW - 12)
      .attr('y2', rectY + headerH)
      .attr('stroke', 'rgba(148, 163, 184, 0.2)')
      .attr('stroke-width', 1)

    // Pastille de statut
    cluster.append('circle')
      .attr('cx', rectX + 18)
      .attr('cy', rectY + headerH / 2)
      .attr('r', 5)
      .attr('fill', statusColor)

    // Nom de l'hôte
    cluster.append('text')
      .attr('x', rectX + 32)
      .attr('y', rectY + headerH / 2 - 4)
      .style('font-size', '12px')
      .style('font-weight', '700')
      .style('fill', '#e2e8f0')
      .style('pointer-events', 'none')
      .text(hostName)

    // IP en sous-titre
    if (hostIp) {
      cluster.append('text')
        .attr('x', rectX + 32)
        .attr('y', rectY + headerH / 2 + 10)
        .style('font-size', '10px')
        .style('fill', '#94a3b8')
        .style('pointer-events', 'none')
        .text(hostIp)
    }
  }

  // Draw tree links only when proxy links are enabled
  // Proxy links: draw from root to all visible leaf nodes (always shown, protocol-colored)
  // NOTE: all SVG styles are set via .attr()/.style() rather than CSS classes because
  // Vue's <style scoped> does not apply to D3-created elements (they lack the data-v-* attr).
  linkGroup
    .selectAll('path')
    .data(treeData.links().filter(d => d.target.data.isProxyLinked))
    .enter()
    .append('path')
    .attr('fill', 'none')
    .attr('d', (d) => {
      const startX = d.source.y + 100 + 120
      const startY = d.source.x + 40
      const targetLeftOffset = d.target.data.type === 'service' ? -130 : -90
      const endX = d.target.y + 100 + targetLeftOffset
      const endY = d.target.x + 40
      const midX = (startX + endX) / 2
      return `M${startX},${startY} C${midX},${startY} ${midX},${endY} ${endX},${endY}`
    })
    .attr('stroke', d => {
      const protocol = d.target.data.protocol
      return protocolColors[protocol] || protocolColors.other
    })
    .attr('stroke-width', 1.4)
    .attr('opacity', 0.65)

  const nodes = nodeGroup
    .selectAll('g')
    .data(treeData.descendants())
    .enter()
    .append('g')
    .attr('class', d => `node ${d.data.type}`)
    .attr('transform', d => `translate(${d.y + 100},${d.x + 40})`)
    .style('cursor', 'default')

  const rootNodes = nodes.filter(d => d.data.type === 'root')
  const serviceNodes = nodes.filter(d => d.data.type === 'service')
  const portNodes = nodes.filter(d => d.data.type === 'port')

  rootNodes
    .append('rect')
    .attr('rx', 12)
    .attr('ry', 12)
    .attr('x', -120)
    .attr('y', -22)
    .attr('width', 240)
    .attr('height', 44)
    .attr('fill', 'rgba(15, 23, 42, 0.9)')
    .attr('stroke', '#94a3b8')
    .attr('stroke-width', 1.4)
    .attr('filter', 'url(#node-shadow)')

  rootNodes
    .append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', '-0.1em')
    .style('font-size', '13px')
    .style('font-weight', '700')
    .style('fill', '#e2e8f0')
    .each(function (d) {
      const text = d3.select(this)
      text.append('tspan').text(d.data.name)
      if (props.rootIp) {
        text.append('tspan')
          .attr('x', 0)
          .attr('dy', '1.2em')
          .style('font-size', '10px')
          .style('fill', '#94a3b8')
          .text(props.rootIp)
      }
    })

  serviceNodes
    .append('rect')
    .attr('rx', 10)
    .attr('ry', 10)
    .attr('x', -130)
    .attr('y', -30)
    .attr('width', 260)
    .attr('height', 60)
    .attr('fill', 'rgba(15, 23, 42, 0.86)')
    .attr('stroke', '#38bdf8')
    .attr('stroke-width', 1.3)
    .attr('filter', 'url(#node-shadow)')
    .on('mouseover', (event, d) => {
      if (!tooltipRef.value) return
      const lines = [d.data.name, d.data.subtitle, `Interne: ${d.data.internalPort || '-'} / Externe: ${d.data.externalPort || '-'}`]
      if (d.data.tags) lines.push(`Tags: ${d.data.tags}`)
      tooltipRef.value.innerHTML = lines.map(line => `<div>${escapeHtml(line)}</div>`).join('')
      tooltipRef.value.style.display = 'block'
      tooltipRef.value.style.left = `${event.pageX + 12}px`
      tooltipRef.value.style.top = `${event.pageY + 12}px`
    })
    .on('mousemove', (event) => {
      if (!tooltipRef.value || tooltipRef.value.style.display !== 'block') return
      tooltipRef.value.style.left = `${event.pageX + 12}px`
      tooltipRef.value.style.top = `${event.pageY + 12}px`
    })
    .on('mouseout', () => {
      if (tooltipRef.value) tooltipRef.value.style.display = 'none'
    })

  serviceNodes
    .append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', '-0.7em')
    .style('font-size', '11px')
    .style('font-weight', '600')
    .style('fill', '#e2e8f0')
    .each(function (d) {
      const text = d3.select(this)
      text.append('tspan').text(d.data.name)
      if (d.data.subtitle) {
        text.append('tspan')
          .attr('x', 0)
          .attr('dy', '1.3em')
          .style('font-size', '10px')
          .style('fill', '#93c5fd')
          .text(d.data.subtitle)
      }
      if (d.data.internalPort) {
        text.append('tspan')
          .attr('x', 0)
          .attr('dy', '1.2em')
          .style('font-size', '10px')
          .style('fill', '#cbd5f5')
          .text(`Port interne ${d.data.internalPort}`)
      }
    })

  portNodes
    .append('rect')
    .attr('rx', 8)
    .attr('ry', 8)
    .attr('x', -90)
    .attr('y', -16)
    .attr('width', 180)
    .attr('height', 32)
    .attr('fill', 'rgba(15, 23, 42, 0.82)')
    .attr('stroke', d => protocolColors[d.data.protocol] || protocolColors.other)
    .attr('stroke-width', 1.2)
    .attr('filter', 'url(#node-shadow)')
    .on('mouseover', (event, d) => {
      if (!tooltipRef.value) return
      const lines = [`Port ${d.data.name}`]
      if (d.data.containers?.length) lines.push(`Conteneurs: ${d.data.containers.length}`)
      tooltipRef.value.innerHTML = lines.map(line => `<div>${escapeHtml(line)}</div>`).join('')
      tooltipRef.value.style.display = 'block'
      tooltipRef.value.style.left = `${event.pageX + 12}px`
      tooltipRef.value.style.top = `${event.pageY + 12}px`
    })
    .on('mousemove', (event) => {
      if (!tooltipRef.value || tooltipRef.value.style.display !== 'block') return
      tooltipRef.value.style.left = `${event.pageX + 12}px`
      tooltipRef.value.style.top = `${event.pageY + 12}px`
    })
    .on('mouseout', () => {
      if (tooltipRef.value) tooltipRef.value.style.display = 'none'
    })

  portNodes
    .append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', '0.35em')
    .style('font-size', '11px')
    .style('fill', '#e2e8f0')
    .text(d => d.data.name)

  // Authelia node + arcs (purple dashed arcs to authelia-linked nodes)
  const autheliaTargets = treeData.descendants().filter(
    d => (d.data.type === 'service' || d.data.type === 'port') && d.data.isAutheliaLinked
  )
  const hasAuthelia = props.autheliaLabel && autheliaTargets.length > 0
  const rootNode = treeData.descendants().find(d => d.data.type === 'root')

  if (hasAuthelia && rootNode) {
    // Position Authelia node above the root node
    const rootSvgX = rootNode.y + 100
    const rootSvgY = rootNode.x + 40
    const authX = rootSvgX
    const authY = rootSvgY - 120

    // Draw Authelia node (purple rounded rect)
    const authNode = specialNodeGroup.append('g')
      .attr('transform', `translate(${authX},${authY})`)
    authNode.append('rect')
      .attr('rx', 10).attr('ry', 10)
      .attr('x', -100).attr('y', -20)
      .attr('width', 200).attr('height', 40)
      .attr('fill', 'rgba(139, 92, 246, 0.15)')
      .attr('stroke', '#8b5cf6')
      .attr('stroke-width', 1.6)
      .attr('filter', 'url(#node-shadow)')
    authNode.append('text')
      .attr('text-anchor', 'middle').attr('dy', '-0.1em')
      .style('font-size', '12px').style('font-weight', '700').style('fill', '#c4b5fd')
      .text(props.autheliaLabel || 'Authelia')
    if (props.autheliaIp) {
      authNode.append('text')
        .attr('text-anchor', 'middle').attr('dy', '1.2em')
        .style('font-size', '10px').style('fill', '#8b5cf6')
        .text(props.autheliaIp)
    }

    // Draw arcs: root → Authelia (dotted cyan) and Authelia → targets (dotted purple)
    autheliaLinkGroup.append('path')
      .attr('fill', 'none')
      .attr('stroke', 'rgba(139, 92, 246, 0.5)')
      .attr('stroke-width', 1.5)
      .attr('stroke-dasharray', '5 4')
      .attr('d', () => {
        const sx = rootSvgX + 120  // right edge of root
        const sy = rootSvgY
        const ex = authX - 100     // left edge of authelia
        const ey = authY
        const mx = (sx + ex) / 2
        return `M${sx},${sy} C${mx},${sy} ${mx},${ey} ${ex},${ey}`
      })

    autheliaTargets.forEach(d => {
      const targetLeftOffset = d.data.type === 'service' ? -130 : -90
      autheliaLinkGroup.append('path')
        .attr('fill', 'none')
        .attr('stroke', 'rgba(167, 139, 250, 0.8)')
        .attr('stroke-width', 1.8)
        .attr('stroke-dasharray', '6 4')
        .attr('d', () => {
          const sx = authX + 100  // right edge of authelia
          const sy = authY
          const ex = d.y + 100 + targetLeftOffset
          const ey = d.x + 40
          const mx = (sx + ex) / 2
          return `M${sx},${sy} C${mx},${sy} ${mx},${ey} ${ex},${ey}`
        })
    })
  }

  // Internet node + arcs (orange dashed arcs to internet-exposed nodes)
  const internetTargets = treeData.descendants().filter(
    d => (d.data.type === 'service' || d.data.type === 'port') && d.data.isInternetExposed
  )

  if (props.internetLabel && rootNode) {
    const rootSvgX = rootNode.y + centerX
    const rootSvgY = rootNode.x + 40
    const intX = rootSvgX - 320
    const intY = rootSvgY

    // Draw Internet node (orange rounded rect)
    const intNode = specialNodeGroup.append('g')
      .attr('transform', `translate(${intX},${intY})`)
    intNode.append('rect')
      .attr('rx', 10).attr('ry', 10)
      .attr('x', -100).attr('y', -20)
      .attr('width', 200).attr('height', 40)
      .attr('fill', 'rgba(251, 146, 60, 0.12)')
      .attr('stroke', '#fb923c')
      .attr('stroke-width', 1.6)
      .attr('filter', 'url(#node-shadow)')
    intNode.append('text')
      .attr('text-anchor', 'middle').attr('dy', '-0.1em')
      .style('font-size', '12px').style('font-weight', '700').style('fill', '#fed7aa')
      .text(props.internetLabel || 'Internet')
    if (props.internetIp) {
      intNode.append('text')
        .attr('text-anchor', 'middle').attr('dy', '1.2em')
        .style('font-size', '10px').style('fill', '#fb923c')
        .text(props.internetIp)
    }

    // Draw arc: Internet → root (dotted orange)
    internetLinkGroup.append('path')
      .attr('fill', 'none')
      .attr('stroke', 'rgba(251, 146, 60, 0.5)')
      .attr('stroke-width', 1.5)
      .attr('stroke-dasharray', '5 4')
      .attr('d', () => {
        const sx = intX + 100  // right edge of internet
        const sy = intY
        const ex = rootSvgX - 120  // left edge of root
        const ey = rootSvgY
        const mx = (sx + ex) / 2
        return `M${sx},${sy} C${mx},${sy} ${mx},${ey} ${ex},${ey}`
      })

    internetTargets.forEach(d => {
      const targetLeftOffset = d.data.type === 'service' ? -130 : -90
      const extPort = d.data.externalPort
      internetLinkGroup.append('path')
        .attr('fill', 'none')
        .attr('stroke', 'rgba(251, 146, 60, 0.85)')
        .attr('stroke-width', 1.8)
        .attr('stroke-dasharray', '6 4')
        .attr('d', () => {
          const sx = intX + 100
          const sy = intY
          const ex = d.y + 100 + targetLeftOffset
          const ey = d.x + 40
          const mx = (sx + ex) / 2
          return `M${sx},${sy} C${mx},${sy} ${mx},${ey} ${ex},${ey}`
        })
      // Label external port on the arc
      if (extPort) {
        const sx = intX + 100, sy = intY
        const ex = d.y + 100 + targetLeftOffset, ey = d.x + 40
        internetLinkGroup.append('text')
          .attr('x', (sx + ex) / 2)
          .attr('y', (sy + ey) / 2 - 6)
          .attr('text-anchor', 'middle')
          .style('font-size', '10px')
          .style('fill', '#fb923c')
          .style('pointer-events', 'none')
          .text(`:${extPort}`)
      }
    })
  }
}

let handleResize = null

onMounted(() => {
  render()
  handleResize = () => render()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  if (handleResize) {
    window.removeEventListener('resize', handleResize)
  }
})

// Watch for data changes — hierarchyRoot covers props.data/services/excludedPorts/hostPortOverrides/serviceMap/rootLabel
// rootIp, autheliaLabel/IP and internetLabel/IP are render-only (not part of the hierarchy)
watch(
  [hierarchyRoot, () => props.rootIp, () => props.autheliaLabel, () => props.autheliaIp, () => props.internetLabel, () => props.internetIp],
  () => { render() }
)
</script>

<style scoped>
.d3-graph-container {
  width: 100%;
  height: 100%;
  min-height: 520px;
  position: relative;
  background: radial-gradient(circle at 15% 20%, rgba(96, 165, 250, 0.18), transparent 42%),
    radial-gradient(circle at 70% 0%, rgba(16, 185, 129, 0.12), transparent 40%),
    radial-gradient(circle at 20% 80%, rgba(244, 63, 94, 0.1), transparent 45%),
    #0b0f1a;
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 16px;
  overflow: hidden;
}

.d3-graph-container::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(circle, rgba(148, 163, 184, 0.22) 1px, transparent 1px);
  background-size: 48px 48px;
  opacity: 0.35;
  pointer-events: none;
  z-index: 0;
}

.d3-graph {
  width: 100%;
  height: 100%;
  min-height: 520px;
  position: relative;
  z-index: 1;
}

.d3-tooltip {
  position: fixed;
  padding: 10px 12px;
  background-color: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 12px;
  pointer-events: none;
  display: none;
  z-index: 1000;
  white-space: nowrap;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.25);
}

.graph-legend {
  position: absolute;
  top: 16px;
  left: 16px;
  padding: 12px 14px;
  background: rgba(15, 23, 42, 0.75);
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 12px;
  backdrop-filter: blur(10px);
  z-index: 2;
  font-size: 12px;
  color: #e2e8f0;
  min-width: 160px;
}

.legend-title {
  font-weight: 700;
  margin-bottom: 6px;
  letter-spacing: 0.4px;
  text-transform: uppercase;
  font-size: 11px;
  color: #94a3b8;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}

.legend-dot.port-tcp     { background: #2563eb; }
.legend-dot.port-udp     { background: #f97316; }
.legend-dot.service-node { background: #38bdf8; }

.graph-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  z-index: 1;
  text-align: center;
  color: #94a3b8;
  padding: 24px;
}

.empty-title {
  font-size: 18px;
  font-weight: 600;
  color: #f8fafc;
}

.empty-subtitle {
  margin-top: 6px;
  font-size: 13px;
}

.link { fill: none; }

.proxy-link {
  fill: none;
  stroke: rgba(56, 189, 248, 0.65);
  stroke-width: 1.6;
  stroke-dasharray: 6 6;
}

.service-cluster rect {
  fill: rgba(15, 23, 42, 0.5);
  stroke: rgba(148, 163, 184, 0.3);
  stroke-width: 1.5;
}

.service-cluster:hover rect {
  stroke: rgba(148, 163, 184, 0.6);
  fill: rgba(15, 23, 42, 0.65);
}

.cluster-label {
  font-size: 12px;
  font-weight: 700;
  fill: #e2e8f0;
  pointer-events: none;
}

.cluster-sublabel {
  font-size: 10px;
  fill: #cbd5e1;
  pointer-events: none;
}

.node { cursor: pointer; }
.node text { pointer-events: none; }
</style>

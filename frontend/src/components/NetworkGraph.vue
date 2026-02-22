<template>
  <div class="d3-graph-container">
    <div class="graph-legend">
      <div class="legend-title">Topology</div>
      <div class="legend-item">
        <span class="legend-dot host-online"></span>
        Hote en ligne
      </div>
      <div class="legend-item">
        <span class="legend-dot host-offline"></span>
        Hote hors ligne
      </div>
      <div class="legend-item">
        <span class="legend-dot port-tcp"></span>
        Port TCP
      </div>
      <div class="legend-item">
        <span class="legend-dot port-udp"></span>
        Port UDP
      </div>
      <div class="legend-item">
        <span class="legend-dot service-node"></span>
        Service expose
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
  showProxyLinks: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['host-click'])

const svgRef = ref(null)
const tooltipRef = ref(null)
const hasData = computed(() => Array.isArray(props.data) && props.data.length > 0)

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

const buildHierarchy = () => {
  const globalExcluded = (props.excludedPorts || []).map(value => Number(value)).filter(Boolean)
  const servicesByHost = new Map()
  for (const service of props.services || []) {
    if (!service?.hostId) continue
    if (!servicesByHost.has(service.hostId)) {
      servicesByHost.set(service.hostId, [])
    }
    servicesByHost.get(service.hostId).push(service)
  }
  const root = {
    id: 'root',
    name: props.rootLabel || 'root',
    type: 'root',
    children: props.data.map((host) => {
      const hostOverride = props.hostPortOverrides?.[host.id]
      const hostExcluded = new Set([
        ...globalExcluded,
        ...(hostOverride?.excludedPorts || [])
      ].map(value => Number(value)).filter(Boolean))
      const hostPortMap = hostOverride?.portMap || {}
      const rawPorts = host.ports || []
      const filteredPorts = rawPorts.filter((port) => {
        const portNumber = Number(port.port || 0)
        return portNumber && !hostExcluded.has(portNumber)
      })
      const hostServices = servicesByHost.get(host.id) || []

      return {
        id: `host-${host.id}`,
        name: host.name || host.hostname || host.id,
        type: 'host',
        hostId: host.id,
        status: host.status || 'unknown',
        portCount: hostServices.length ? hostServices.length : filteredPorts.length,
        ipAddress: host.ip_address,
        children: hostServices.length
          ? hostServices.map((service) => {
            const internalPort = Number(service.internalPort || 0)
            const externalPort = Number(service.externalPort || 0)
            const path = service.path || '/'
            const domain = service.domain || ''
            const name = service.name || 'Service'
            const label = domain ? `${domain}${path}` : path
            return {
              id: `service-${host.id}-${service.id}`,
              name,
              subtitle: label,
              internalPort,
              externalPort,
              type: 'service',
              protocol: 'tcp',
              port: internalPort,
              hostId: host.id,
              tags: service.tags || '',
              isProxyLinked: true
            }
          })
          : filteredPorts.map((port) => {
            const portNumber = Number(port.port || 0)
            const protocol = (port.protocol || 'tcp').toLowerCase()
            const serviceName = hostPortMap?.[portNumber] || props.serviceMap?.[portNumber]
            const label = serviceName ? `${serviceName} (${portNumber}/${protocol.toUpperCase()})` : `${portNumber}/${protocol.toUpperCase()}`
            return {
              id: `port-${host.id}-${port.port}-${protocol}`,
              name: label,
              type: 'port',
              protocol,
              port: portNumber,
              hostId: host.id,
              containers: port.containers || [],
              isProxyLinked: !!(hostPortMap?.[portNumber])
            }
          })
      }
    })
  }

  return d3.hierarchy(root)
}

const render = () => {
  if (!svgRef.value) return

  const width = svgRef.value.clientWidth || 1000
  const height = svgRef.value.clientHeight || 600

  d3.select(svgRef.value).selectAll('*').remove()
  if (!hasData.value) return

  const root = buildHierarchy()
  if (!root.children || root.children.length === 0) return

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
  const proxyLinkGroup = g.append('g').attr('class', 'proxy-links')
  const linkGroup = g.append('g').attr('class', 'links')
  const nodeGroup = g.append('g').attr('class', 'nodes')

  const serviceNodesData = treeData.descendants().filter((d) => d.data.type === 'service')
  const hostNodesData = treeData.descendants().filter((d) => d.data.type === 'host')
  const rootNodeData = treeData.descendants().find((d) => d.data.type === 'root')
  const hostById = new Map(hostNodesData.map((node) => [node.data.hostId, node]))

  const clusterPadding = { x: 80, y: 36 }
  const clusters = d3.group(serviceNodesData, (d) => d.data.hostId)

  // Render service clusters
  for (const [hostId, nodes] of clusters.entries()) {
    if (nodes.length === 0) continue
    const hostNode = hostById.get(hostId)
    const positions = nodes.map((node) => ({
      x: node.y + 100,
      y: node.x + 40
    }))
    const minX = d3.min(positions, (pos) => pos.x) ?? 0
    const maxX = d3.max(positions, (pos) => pos.x) ?? 0
    const minY = d3.min(positions, (pos) => pos.y) ?? 0
    const maxY = d3.max(positions, (pos) => pos.y) ?? 0

    const rectX = minX - 130 - clusterPadding.x
    const rectY = minY - 22 - clusterPadding.y
    const rectW = (maxX - minX) + 260 + clusterPadding.x * 2
    const rectH = (maxY - minY) + 44 + clusterPadding.y * 2

    const label = hostNode?.data?.name || 'Host'
    const ip = hostNode?.data?.ipAddress ? ` • ${hostNode.data.ipAddress}` : ''

    const cluster = clusterGroup.append('g').attr('class', 'service-cluster')
    cluster.append('rect')
      .attr('x', rectX)
      .attr('y', rectY)
      .attr('width', rectW)
      .attr('height', rectH)
      .attr('rx', 16)
      .attr('ry', 16)

    cluster.append('text')
      .attr('x', rectX + 20)
      .attr('y', rectY + 26)
      .attr('class', 'cluster-label')
      .text(`${label}${ip}`)
  }

  linkGroup
    .selectAll('path')
    .data(treeData.links())
    .enter()
    .append('path')
    .attr('class', 'link')
    .attr('d', (d) => {
      const startX = d.source.y
      const startY = d.source.x
      const endX = d.target.y
      const endY = d.target.x
      const midX = (startX + endX) / 2
      return `M${startX},${startY} C${midX},${startY} ${midX},${endY} ${endX},${endY}`
    })
    .attr('stroke', d => {
      if (d.target.data.type === 'host') return 'rgba(148, 163, 184, 0.7)'
      const protocol = d.target.data.protocol
      return protocolColors[protocol] || protocolColors.other
    })
    .attr('stroke-width', d => (d.target.data.type === 'host' ? 2.4 : 1.4))
    .attr('opacity', 0.7)

  const nodes = nodeGroup
    .selectAll('g')
    .data(treeData.descendants())
    .enter()
    .append('g')
    .attr('class', d => `node ${d.data.type}`)
    .attr('transform', d => `translate(${d.y + 100},${d.x + 40})`)
    .style('cursor', d => d.data.type === 'host' ? 'pointer' : 'default')

  const rootNodes = nodes.filter(d => d.data.type === 'root')
  const hostNodes = nodes.filter(d => d.data.type === 'host')
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

  hostNodes
    .append('rect')
    .attr('rx', 10)
    .attr('ry', 10)
    .attr('x', -110)
    .attr('y', -21)
    .attr('width', 220)
    .attr('height', 42)
    .attr('fill', d => statusColors[d.data.status] ? 'rgba(30, 41, 59, 0.85)' : 'rgba(30, 41, 59, 0.85)')
    .attr('stroke', d => statusColors[d.data.status] || statusColors.unknown)
    .attr('stroke-width', 1.6)
    .attr('filter', 'url(#node-shadow)')
    .on('click', (event, d) => {
      emit('host-click', d.data.hostId)
    })
    .on('mouseover', (event, d) => {
      if (!tooltipRef.value) return
      const lines = [d.data.name, `Statut: ${d.data.status}`, `Ports: ${d.data.portCount}`]
      tooltipRef.value.innerHTML = lines.map(line => `<div>${line}</div>`).join('')
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

  hostNodes
    .append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', '-0.5em')
    .style('font-size', '12px')
    .style('font-weight', '600')
    .style('fill', '#f8fafc')
    .each(function (d) {
      const text = d3.select(this)
      text.append('tspan').text(d.data.name)
      if (d.data.ipAddress) {
        text.append('tspan')
          .attr('x', 0)
          .attr('dy', '1.3em')
          .style('font-size', '10px')
          .style('fill', '#cbd5f5')
          .text(d.data.ipAddress)
      }
    })

  hostNodes
    .append('circle')
    .attr('r', 5)
    .attr('cx', -118)
    .attr('cy', 0)
    .attr('fill', d => statusColors[d.data.status] || statusColors.unknown)

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
      tooltipRef.value.innerHTML = lines.map(line => `<div>${line}</div>`).join('')
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
      tooltipRef.value.innerHTML = lines.map(line => `<div>${line}</div>`).join('')
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

  // Draw proxy links if enabled
  if (props.showProxyLinks) {
    const rootNode = treeData.descendants().find(d => d.data.type === 'root')
    const proxyTargets = treeData.descendants().filter(
      d => (d.data.type === 'service' || d.data.type === 'port') && d.data.isProxyLinked
    )

    if (rootNode && proxyTargets.length > 0) {
      proxyLinkGroup
        .selectAll('.proxy-link')
        .data(proxyTargets)
        .enter()
        .append('path')
        .attr('class', 'proxy-link')
        .attr('d', (d) => {
          const sx = rootNode.y + 100
          const sy = rootNode.x + 40
          const ex = d.y + 100
          const ey = d.x + 40
          const mx = (sx + ex) / 2
          return `M${sx},${sy} C${mx},${sy} ${mx},${ey} ${ex},${ey}`
        })
    }
  }
}

onMounted(() => {
  render()

  const handleResize = () => {
    render()
  }

  window.addEventListener('resize', handleResize)

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
  })
})

// Watch for data changes
watch(
  () => [props.data, props.services, props.excludedPorts, props.hostPortOverrides, props.showProxyLinks, props.serviceMap, props.rootLabel, props.rootIp],
  () => { render() },
  { deep: true }
)
</script>

<style scoped>
</style>

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
  }
})

const emit = defineEmits(['host-click'])

const svgRef = ref(null)
const tooltipRef = ref(null)
const hasData = computed(() => Array.isArray(props.data) && props.data.length > 0)
let simulation = null

const statusColors = {
  online: '#10b981',
  warning: '#f59e0b',
  offline: '#ef4444',
  unknown: '#94a3b8'
}

const protocolColors = {
  tcp: '#2563eb',
  udp: '#f97316',
  other: '#14b8a6'
}

const buildGraphData = () => {
  const nodes = []
  const links = []

  for (const host of props.data) {
    const hostNode = {
      id: `host-${host.id}`,
      type: 'host',
      name: host.name || host.hostname || host.id,
      status: host.status || 'unknown',
      hostId: host.id,
      portCount: (host.ports || []).length,
      containerCount: (host.containers || []).length
    }
    nodes.push(hostNode)

    const ports = host.ports || []
    for (const port of ports) {
      const protocol = (port.protocol || 'tcp').toLowerCase()
      const portNode = {
        id: `port-${host.id}-${port.port}-${protocol}`,
        type: 'port',
        name: `${port.port}/${protocol.toUpperCase()}`,
        protocol,
        port: port.port,
        hostId: host.id,
        containers: port.containers || []
      }
      nodes.push(portNode)
      links.push({ source: hostNode.id, target: portNode.id, protocol })
    }
  }

  return { nodes, links }
}

const render = () => {
  if (!svgRef.value) return

  const width = svgRef.value.clientWidth || 1000
  const height = svgRef.value.clientHeight || 600

  d3.select(svgRef.value).selectAll('*').remove()
  if (!hasData.value) return

  if (simulation) {
    simulation.stop()
    simulation = null
  }

  const { nodes, links } = buildGraphData()
  if (!nodes.length) return

  const svg = d3.select(svgRef.value)
    .attr('width', width)
    .attr('height', height)

  const defs = svg.append('defs')
  defs.append('filter')
    .attr('id', 'node-shadow')
    .append('feDropShadow')
    .attr('dx', 0)
    .attr('dy', 6)
    .attr('stdDeviation', 6)
    .attr('flood-opacity', 0.25)

  const g = svg.append('g').attr('class', 'graph-root')

  const zoom = d3.zoom()
    .scaleExtent([0.4, 2.6])
    .on('zoom', (event) => {
      g.attr('transform', event.transform)
    })

  svg.call(zoom)

  const linkGroup = g.append('g').attr('class', 'links')
  const nodeGroup = g.append('g').attr('class', 'nodes')

  const link = linkGroup
    .selectAll('line')
    .data(links)
    .enter()
    .append('line')
    .attr('class', 'link')
    .attr('stroke', d => protocolColors[d.protocol] || protocolColors.other)
    .attr('stroke-width', 1.8)
    .attr('opacity', 0.55)

  const drag = d3.drag()
    .on('start', (event, d) => {
      if (!event.active) simulation.alphaTarget(0.3).restart()
      d.fx = d.x
      d.fy = d.y
    })
    .on('drag', (event, d) => {
      d.fx = event.x
      d.fy = event.y
    })
    .on('end', (event, d) => {
      if (!event.active) simulation.alphaTarget(0)
      d.fx = null
      d.fy = null
    })

  const node = nodeGroup
    .selectAll('g')
    .data(nodes)
    .enter()
    .append('g')
    .attr('class', d => `node ${d.type}`)
    .style('cursor', d => d.type === 'host' ? 'pointer' : 'default')
    .call(drag)

  node.append('circle')
    .attr('r', d => d.type === 'host' ? 18 : 8)
    .attr('fill', d => {
      if (d.type === 'host') return statusColors[d.status] || statusColors.unknown
      return protocolColors[d.protocol] || protocolColors.other
    })
    .attr('stroke', d => {
      if (d.type === 'host') return '#0f172a'
      return '#1f2937'
    })
    .attr('stroke-width', d => d.type === 'host' ? 2 : 1)
    .attr('filter', d => d.type === 'host' ? 'url(#node-shadow)' : null)
    .on('click', (event, d) => {
      if (d.type === 'host') {
        emit('host-click', d.hostId)
      }
    })
    .on('mouseover', (event, d) => {
      if (!tooltipRef.value) return
      const lines = []
      if (d.type === 'host') {
        lines.push(d.name)
        lines.push(`Statut: ${d.status}`)
        lines.push(`Ports: ${d.portCount}`)
        if (d.containerCount) lines.push(`Conteneurs: ${d.containerCount}`)
      } else {
        lines.push(`Port ${d.port}/${(d.protocol || 'tcp').toUpperCase()}`)
        if (d.containers?.length) lines.push(`Conteneurs: ${d.containers.length}`)
      }

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
      if (tooltipRef.value) {
        tooltipRef.value.style.display = 'none'
      }
    })

  node
    .filter(d => d.type === 'host')
    .append('text')
    .attr('x', 26)
    .attr('dy', '0.31em')
    .style('font-size', '12px')
    .style('font-weight', '600')
    .style('fill', '#0f172a')
    .text(d => d.name)

  const showPortLabels = nodes.filter(n => n.type === 'port').length <= 24
  if (showPortLabels) {
    node
      .filter(d => d.type === 'port')
      .append('text')
      .attr('x', 14)
      .attr('dy', '0.35em')
      .style('font-size', '10px')
      .style('fill', '#1f2937')
      .text(d => d.name)
  }

  simulation = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id(d => d.id).distance(d => d.protocol ? 70 : 120))
    .force('charge', d3.forceManyBody().strength(d => d.type === 'host' ? -420 : -90))
    .force('collision', d3.forceCollide().radius(d => d.type === 'host' ? 34 : 16))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('x', d3.forceX(width / 2).strength(0.05))
    .force('y', d3.forceY(height / 2).strength(0.05))
    .on('tick', () => {
      link
        .attr('x1', d => d.source.x)
        .attr('y1', d => d.source.y)
        .attr('x2', d => d.target.x)
        .attr('y2', d => d.target.y)

      node.attr('transform', d => `translate(${d.x},${d.y})`)
    })
}

onMounted(() => {
  render()

  const handleResize = () => {
    render()
  }

  window.addEventListener('resize', handleResize)

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
    if (simulation) simulation.stop()
  })
})

// Watch for data changes
watch(() => props.data, () => {
  render()
}, { deep: true })
</script>

<style scoped>

.d3-graph-container {
  width: 100%;
  height: 100%;
  min-height: 520px;
  position: relative;
  background: radial-gradient(circle at 20% 20%, rgba(59, 130, 246, 0.08), transparent 45%),
    radial-gradient(circle at 80% 0%, rgba(16, 185, 129, 0.08), transparent 40%),
    linear-gradient(180deg, #f8fafc 0%, #eef2f7 100%);
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  overflow: hidden;
}

.d3-graph-container::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image: linear-gradient(rgba(148, 163, 184, 0.2) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.2) 1px, transparent 1px);
  background-size: 40px 40px;
  opacity: 0.35;
  pointer-events: none;
}

.d3-graph {
  width: 100%;
  height: 100%;
  position: relative;
  z-index: 1;
}

.d3-tooltip {
  position: fixed;
  padding: 10px 12px;
  background-color: rgba(15, 23, 42, 0.92);
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
  background: rgba(255, 255, 255, 0.85);
  border: 1px solid rgba(148, 163, 184, 0.4);
  border-radius: 12px;
  backdrop-filter: blur(10px);
  z-index: 2;
  font-size: 12px;
  color: #1f2937;
  min-width: 160px;
}

.legend-title {
  font-weight: 700;
  margin-bottom: 6px;
  letter-spacing: 0.4px;
  text-transform: uppercase;
  font-size: 11px;
  color: #64748b;
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
}

.legend-dot.host-online {
  background: #10b981;
}

.legend-dot.host-offline {
  background: #ef4444;
}

.legend-dot.port-tcp {
  background: #2563eb;
}

.legend-dot.port-udp {
  background: #f97316;
}

.graph-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  z-index: 1;
  text-align: center;
  color: #64748b;
  padding: 24px;
}

.empty-title {
  font-size: 18px;
  font-weight: 600;
  color: #0f172a;
}

.empty-subtitle {
  margin-top: 6px;
  font-size: 13px;
}

.link {
  fill: none;
}

.node {
  cursor: pointer;
}

.node text {
  pointer-events: none;
}
</style>

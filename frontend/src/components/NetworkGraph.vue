<template>
  <div class="d3-graph-container">
    <div ref="tooltipRef" class="d3-tooltip"></div>
    <svg ref="svgRef" class="d3-graph"></svg>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
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
let simulation = null

// Build hierarchical data structure for D3 tree layout
const buildHierarchy = () => {
  const root = {
    id: 'root',
    name: 'Network',
    type: 'root',
    children: props.data.map(host => ({
      id: `host-${host.id}`,
      name: host.name || host.id,
      type: 'host',
      hostId: host.id,
      status: host.status,
      children: (host.ports || []).map(port => ({
        id: `${host.id}-${port.port}`,
        name: `Port ${port.port} (${port.protocol})`,
        type: 'port',
        protocol: port.protocol,
        port: port.port,
        containers: port.containers || []
      }))
    }))
  }
  return d3.hierarchy(root)
}

const render = () => {
  if (!svgRef.value || !props.data.length) return

  const width = svgRef.value.clientWidth || 1000
  const height = svgRef.value.clientHeight || 600

  d3.select(svgRef.value).selectAll('*').remove()

  const svg = d3.select(svgRef.value)
    .attr('width', width)
    .attr('height', height)

  const g = svg.append('g')
    .attr('transform', `translate(${width / 4},${height / 2})`) // Offset for tree layout

  // Create tree layout
  const treeLayout = d3.tree().size([height, width / 2])
  const root = buildHierarchy()
  const treeData = treeLayout(root)

  // Draw links (lines between nodes)
  g.selectAll('.link')
    .data(treeData.links())
    .enter()
    .append('line')
    .attr('class', 'link')
    .attr('x1', d => d.source.y)
    .attr('y1', d => d.source.x)
    .attr('x2', d => d.target.y)
    .attr('y2', d => d.target.x)
    .style('stroke', '#999')
    .style('stroke-width', '2')
    .style('opacity', 0.6)

  // Draw nodes
  const nodes = g.selectAll('.node')
    .data(treeData.descendants())
    .enter()
    .append('g')
    .attr('class', 'node')
    .attr('transform', d => `translate(${d.y},${d.x})`)

  // Node circles with color coding
  nodes.append('circle')
    .attr('r', d => {
      if (d.data.type === 'root') return 0
      if (d.data.type === 'host') return 12
      return 6 // ports
    })
    .style('fill', d => {
      if (d.data.type === 'root') return 'none'
      if (d.data.type === 'host') {
        return d.data.status === 'online' ? '#10b981' : '#ef4444' // green/red
      }
      return '#3b82f6' // blue for ports
    })
    .style('stroke', d => {
      if (d.data.type === 'host') return d.data.status === 'online' ? '#059669' : '#dc2626'
      return '#1e40af'
    })
    .style('stroke-width', '2')
    .style('cursor', d => d.data.type === 'host' ? 'pointer' : 'default')
    .on('click', (event, d) => {
      if (d.data.type === 'host') {
        emit('host-click', d.data.hostId)
      }
    })
    .on('mouseover', function(event, d) {
      d3.select(this)
        .transition()
        .duration(200)
        .attr('r', r => {
          if (d.data.type === 'host') return 16
          if (r == 0) return 0
          return 8
        })
      
      if (tooltipRef.value) {
        let tooltipText = d.data.name
        if (d.data.type === 'host') {
          tooltipText += ` [${d.data.status}]`
        }
        if (d.data.containers && d.data.containers.length > 0) {
          tooltipText += ` (${d.data.containers.length} containers)`
        }
        
        tooltipRef.value.textContent = tooltipText
        tooltipRef.value.style.display = 'block'
        tooltipRef.value.style.left = (event.pageX + 10) + 'px'
        tooltipRef.value.style.top = (event.pageY + 10) + 'px'
      }
    })
    .on('mousemove', (event) => {
      if (tooltipRef.value && tooltipRef.value.style.display === 'block') {
        tooltipRef.value.style.left = (event.pageX + 10) + 'px'
        tooltipRef.value.style.top = (event.pageY + 10) + 'px'
      }
    })
    .on('mouseout', function(event, d) {
      d3.select(this)
        .transition()
        .duration(200)
        .attr('r', r => {
          if (d.data.type === 'host') return 12
          if (r == 0) return 0
          return 6
        })
      
      if (tooltipRef.value) {
        tooltipRef.value.style.display = 'none'
      }
    })

  // Node labels
  nodes.append('text')
    .attr('dy', '0.31em')
    .attr('x', d => d.data.type === 'host' ? 20 : 12)
    .style('font-size', d => d.data.type === 'host' ? '12px' : '10px')
    .style('font-weight', d => d.data.type === 'host' ? '600' : '400')
    .style('fill', '#374151')
    .text(d => {
      if (d.data.type === 'root') return ''
      return d.data.name
    })
}

onMounted(() => {
  // Render initial graph
  render()

  // Handle window resize
  const handleResize = () => {
    render()
  }

  window.addEventListener('resize', handleResize)

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
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
  min-height: 500px;
  position: relative;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
}

.d3-graph {
  width: 100%;
  height: 100%;
}

.d3-tooltip {
  position: fixed;
  padding: 8px 12px;
  background-color: rgba(0, 0, 0, 0.8);
  color: white;
  border-radius: 4px;
  font-size: 12px;
  pointer-events: none;
  display: none;
  z-index: 1000;
  white-space: nowrap;
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

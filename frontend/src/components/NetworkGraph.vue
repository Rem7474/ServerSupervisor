<template>
  <div class="cy-graph-container">
    <!-- Legend -->
    <div class="graph-legend">
      <div class="legend-title">Légende</div>
      <div class="legend-item">
        <span style="display:inline-block;width:20px;height:12px;border:1.5px solid rgba(148,163,184,0.4);border-radius:3px;background:rgba(15,23,42,0.5);flex-shrink:0;"></span>
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
      <div v-if="hasAutheliaTargets" class="legend-item">
        <span style="display:inline-block;width:18px;height:3px;background:#8b5cf6;border-top:2px dashed #8b5cf6;border-radius:0;flex-shrink:0;"></span>
        {{ autheliaLabel || 'Authelia' }}
      </div>
      <div v-if="hasInternetTargets" class="legend-item">
        <span style="display:inline-block;width:18px;height:3px;background:#fb923c;border-top:2px dashed #fb923c;border-radius:0;flex-shrink:0;"></span>
        {{ internetLabel || 'Internet' }}
      </div>
    </div>

    <!-- Controls -->
    <div class="graph-controls">
      <button class="graph-btn" title="Reset layout" @click="resetLayout">
        <svg width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path d="M20 11a8.1 8.1 0 0 0-15.5-2m-.5-4v4h4"/>
          <path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4"/>
        </svg>
      </button>
      <button class="graph-btn" title="Fit to view" @click="fitView">
        <svg width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path d="M3 3h6M3 3v6M21 3h-6M21 3v6M3 21h6M3 21v-6M21 21h-6M21 21v-6"/>
        </svg>
      </button>
    </div>

    <div v-if="!hasData" class="graph-empty">
      <div class="empty-title">Aucune topologie disponible</div>
      <div class="empty-subtitle">Les hôtes actifs apparaîtront ici dès que les données remontent.</div>
    </div>

    <div ref="tooltipRef" class="cy-tooltip"></div>
    <div ref="cyContainer" class="cy-canvas"></div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import cytoscape from 'cytoscape'
import fcose from 'cytoscape-fcose'

cytoscape.use(fcose)

const props = defineProps({
  data: { type: Array, required: true },
  rootLabel: { type: String, default: 'root' },
  rootIp: { type: String, default: '' },
  services: { type: Array, default: () => [] },
  hostPortOverrides: { type: Object, default: () => ({}) },
  autheliaLabel: { type: String, default: 'Authelia' },
  autheliaIp: { type: String, default: '' },
  internetLabel: { type: String, default: 'Internet' },
  internetIp: { type: String, default: '' },
  nodePositions: { type: Object, default: () => ({}) },
})

const emit = defineEmits(['host-click', 'update:nodePositions'])

const cyContainer = ref(null)
const tooltipRef = ref(null)
let cy = null
let resizeObserver = null

const hasData = computed(() => Array.isArray(props.data) && props.data.length > 0)

const statusColors = { online: '#22c55e', warning: '#f59e0b', offline: '#ef4444', unknown: '#94a3b8' }

// Computed: does the data contain authelia/internet targets?
const hasAutheliaTargets = computed(() => props.services.some(s => s.linkToAuthelia))
const hasInternetTargets = computed(() => props.services.some(s => s.exposedToInternet))

function escapeHtml(str) {
  if (str == null) return ''
  return String(str)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

// Build Cytoscape elements from props
function buildElements() {
  const elements = []

  // === Root node ===
  const rootLabel = props.rootLabel || 'Infrastructure'
  elements.push({
    group: 'nodes',
    data: { id: 'root', label: rootLabel, sublabel: props.rootIp || '', type: 'root' }
  })

  // === Services by host ===
  const servicesByHost = new Map()
  for (const svc of props.services || []) {
    if (!svc?.hostId) continue
    if (!servicesByHost.has(svc.hostId)) servicesByHost.set(svc.hostId, [])
    servicesByHost.get(svc.hostId).push(svc)
  }

  // === Host nodes (compound parents) ===
  for (const host of props.data) {
    const hostStatus = host.status || 'unknown'
    const statusColor = statusColors[hostStatus] || statusColors.unknown
    elements.push({
      group: 'nodes',
      data: {
        id: `host-${host.id}`,
        label: host.name || host.hostname || host.id,
        sublabel: host.ip_address || '',
        type: 'host',
        status: hostStatus,
        statusColor,
        hostId: host.id
      }
    })

    const override = props.hostPortOverrides?.[host.id] || {}
    const hostExcluded = new Set((override.excludedPorts || []).map(Number).filter(Boolean))
    const portMap = override.portMap || {}
    const proxyPorts = override.proxyPorts || new Set()
    const autheliaPortNumbers = override.autheliaPortNumbers || new Set()
    const internetExposedPorts = override.internetExposedPorts || {}

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
          containers: port.containers || []
        }
      })

      // Proxy edge
      if (isProxyLinked) {
        elements.push({ group: 'edges', data: { id: `e-proxy-${nodeId}`, source: 'root', target: nodeId, edgeType: 'proxy' } })
      }
      // Authelia edge
      if (isAutheliaLinked) {
        elements.push({ group: 'edges', data: { id: `e-auth-${nodeId}`, source: 'authelia', target: nodeId, edgeType: 'authelia' } })
      }
      // Internet edge
      if (isInternetExposed) {
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
          tags: svc.tags || ''
        }
      })

      if (svc.linkToProxy) {
        elements.push({ group: 'edges', data: { id: `e-proxy-${nodeId}`, source: 'root', target: nodeId, edgeType: 'proxy' } })
      }
      if (svc.linkToAuthelia) {
        elements.push({ group: 'edges', data: { id: `e-auth-${nodeId}`, source: 'authelia', target: nodeId, edgeType: 'authelia' } })
      }
      if (svc.exposedToInternet) {
        elements.push({ group: 'edges', data: { id: `e-inet-${nodeId}`, source: 'internet', target: nodeId, edgeType: 'internet', externalPort: svc.externalPort } })
      }
    }
  }

  // === Authelia node (only if there are linked targets) ===
  const needsAuthelia = elements.some(e => e.group === 'edges' && e.data.edgeType === 'authelia')
  if (needsAuthelia && props.autheliaLabel) {
    elements.push({
      group: 'nodes',
      data: { id: 'authelia', label: props.autheliaLabel || 'Authelia', sublabel: props.autheliaIp || '', type: 'authelia' }
    })
  } else {
    // Remove any pending authelia edges
    const toRemove = elements.filter(e => e.group === 'edges' && e.data.edgeType === 'authelia')
    for (const e of toRemove) elements.splice(elements.indexOf(e), 1)
  }

  // === Internet node (only if there are linked targets) ===
  const needsInternet = elements.some(e => e.group === 'edges' && e.data.edgeType === 'internet')
  if (needsInternet) {
    elements.push({
      group: 'nodes',
      data: { id: 'internet', label: props.internetLabel || 'Internet', sublabel: props.internetIp || '', type: 'internet' }
    })
  } else {
    const toRemove = elements.filter(e => e.group === 'edges' && e.data.edgeType === 'internet')
    for (const e of toRemove) elements.splice(elements.indexOf(e), 1)
  }

  return elements
}

function getCyStyle() {
  return [
    {
      selector: 'node',
      style: {
        'font-family': 'system-ui, sans-serif',
        'color': '#e2e8f0',
        'text-wrap': 'wrap',
        'text-max-width': '180px',
        'text-valign': 'center',
        'text-halign': 'center',
        'overlay-padding': '4px'
      }
    },
    {
      selector: 'node[type="root"]',
      style: {
        'background-color': 'rgba(15,23,42,0.92)',
        'border-color': '#94a3b8',
        'border-width': 2,
        'label': 'data(label)',
        'font-size': '13px',
        'font-weight': 'bold',
        'width': 180,
        'height': 52,
        'shape': 'roundrectangle',
        'color': '#e2e8f0'
      }
    },
    {
      selector: 'node[type="host"]',
      style: {
        'background-color': 'rgba(15,23,42,0.42)',
        'border-color': 'rgba(148,163,184,0.35)',
        'border-width': 1.5,
        'label': 'data(label)',
        'font-size': '12px',
        'font-weight': 'bold',
        'text-valign': 'top',
        'text-halign': 'center',
        'text-margin-y': '6px',
        'padding': '22px',
        'shape': 'roundrectangle',
        'color': '#e2e8f0'
      }
    },
    {
      selector: 'node[type="service"]',
      style: {
        'background-color': 'rgba(15,23,42,0.88)',
        'border-color': '#38bdf8',
        'border-width': 1.4,
        'label': 'data(label)',
        'font-size': '11px',
        'font-weight': '600',
        'width': 200,
        'height': 52,
        'shape': 'roundrectangle',
        'color': '#e2e8f0'
      }
    },
    {
      selector: 'node[type="port"][protocol="tcp"]',
      style: {
        'background-color': 'rgba(15,23,42,0.82)',
        'border-color': '#60a5fa',
        'border-width': 1.3,
        'label': 'data(label)',
        'font-size': '11px',
        'width': 160,
        'height': 38,
        'shape': 'roundrectangle',
        'color': '#e2e8f0'
      }
    },
    {
      selector: 'node[type="port"][protocol="udp"]',
      style: {
        'background-color': 'rgba(15,23,42,0.82)',
        'border-color': '#fb923c',
        'border-width': 1.3,
        'label': 'data(label)',
        'font-size': '11px',
        'width': 160,
        'height': 38,
        'shape': 'roundrectangle',
        'color': '#e2e8f0'
      }
    },
    {
      selector: 'node[type="port"]',
      style: {
        'background-color': 'rgba(15,23,42,0.82)',
        'border-color': '#34d399',
        'border-width': 1.3,
        'label': 'data(label)',
        'font-size': '11px',
        'width': 160,
        'height': 38,
        'shape': 'roundrectangle',
        'color': '#e2e8f0'
      }
    },
    {
      selector: 'node[type="authelia"]',
      style: {
        'background-color': 'rgba(139,92,246,0.15)',
        'border-color': '#8b5cf6',
        'border-width': 1.8,
        'label': 'data(label)',
        'font-size': '12px',
        'font-weight': 'bold',
        'width': 160,
        'height': 44,
        'shape': 'roundrectangle',
        'color': '#c4b5fd'
      }
    },
    {
      selector: 'node[type="internet"]',
      style: {
        'background-color': 'rgba(251,146,60,0.12)',
        'border-color': '#fb923c',
        'border-width': 1.8,
        'label': 'data(label)',
        'font-size': '12px',
        'font-weight': 'bold',
        'width': 160,
        'height': 44,
        'shape': 'roundrectangle',
        'color': '#fed7aa'
      }
    },
    {
      selector: 'node:selected',
      style: {
        'border-width': 2.5,
        'border-color': '#f8fafc'
      }
    },
    // Edges
    {
      selector: 'edge[edgeType="proxy"]',
      style: {
        'line-color': '#60a5fa',
        'target-arrow-color': '#60a5fa',
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.8,
        'width': 1.5,
        'curve-style': 'bezier',
        'opacity': 0.7
      }
    },
    {
      selector: 'edge[edgeType="authelia"]',
      style: {
        'line-color': '#8b5cf6',
        'target-arrow-color': '#8b5cf6',
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.7,
        'width': 1.5,
        'line-style': 'dashed',
        'line-dash-pattern': [6, 4],
        'curve-style': 'bezier',
        'opacity': 0.75
      }
    },
    {
      selector: 'edge[edgeType="internet"]',
      style: {
        'line-color': '#fb923c',
        'target-arrow-color': '#fb923c',
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.7,
        'width': 1.5,
        'line-style': 'dashed',
        'line-dash-pattern': [6, 4],
        'curve-style': 'bezier',
        'opacity': 0.75
      }
    },
    {
      selector: 'edge:selected',
      style: { 'opacity': 1, 'width': 2.2 }
    }
  ]
}

function getLayoutOptions() {
  return {
    name: 'fcose',
    animate: true,
    animationDuration: 600,
    randomize: true,
    nodeRepulsion: 8000,
    idealEdgeLength: 160,
    edgeElasticity: 0.45,
    nestingFactor: 0.1,
    padding: 48,
    componentSpacing: 80,
    fit: true,
    numIter: 2500,
    tile: true,
    tilingPaddingVertical: 20,
    tilingPaddingHorizontal: 20,
  }
}

function showTooltip(event, lines) {
  if (!tooltipRef.value) return
  tooltipRef.value.innerHTML = lines.map(l => `<div>${escapeHtml(l)}</div>`).join('')
  tooltipRef.value.style.display = 'block'
  tooltipRef.value.style.left = `${event.originalEvent.pageX + 12}px`
  tooltipRef.value.style.top = `${event.originalEvent.pageY + 12}px`
}

function hideTooltip() {
  if (tooltipRef.value) tooltipRef.value.style.display = 'none'
}

function moveTooltip(event) {
  if (!tooltipRef.value || tooltipRef.value.style.display !== 'block') return
  tooltipRef.value.style.left = `${event.originalEvent.pageX + 12}px`
  tooltipRef.value.style.top = `${event.originalEvent.pageY + 12}px`
}

let positionSaveTimeout = null
function emitPositions() {
  if (!cy) return
  if (positionSaveTimeout) clearTimeout(positionSaveTimeout)
  positionSaveTimeout = setTimeout(() => {
    const positions = {}
    cy.nodes().forEach(n => {
      const pos = n.position()
      positions[n.id()] = { x: Math.round(pos.x), y: Math.round(pos.y) }
    })
    emit('update:nodePositions', positions)
  }, 600)
}

function initCytoscape() {
  if (!cyContainer.value) return
  if (cy) cy.destroy()

  const savedPositions = props.nodePositions || {}
  const hasPositions = Object.keys(savedPositions).length > 0

  cy = cytoscape({
    container: cyContainer.value,
    elements: buildElements(),
    style: getCyStyle(),
    layout: hasPositions ? { name: 'preset', fit: true, padding: 48 } : getLayoutOptions(),
    minZoom: 0.2,
    maxZoom: 3,
    wheelSensitivity: 0.3,
    boxSelectionEnabled: false,
  })

  if (hasPositions) {
    cy.nodes().forEach(n => {
      const pos = savedPositions[n.id()]
      if (pos) n.position(pos)
    })
    cy.fit(undefined, 48)
  }

  cy.on('dragfree', 'node', emitPositions)

  // Click on host → emit event
  cy.on('tap', 'node[type="host"]', (event) => {
    const hostId = event.target.data('hostId')
    if (hostId) emit('host-click', hostId)
  })

  // Hover tooltips
  cy.on('mouseover', 'node', (event) => {
    const d = event.target.data()
    const lines = []
    if (d.type === 'host') {
      lines.push(d.label)
      if (d.sublabel) lines.push(`IP : ${d.sublabel}`)
      lines.push(`Statut : ${d.status || 'unknown'}`)
    } else if (d.type === 'service') {
      lines.push(d.label)
      if (d.sublabel) lines.push(d.sublabel)
      lines.push(`Port interne : ${d.internalPort || '-'}`)
      if (d.externalPort) lines.push(`Port externe : ${d.externalPort}`)
      if (d.tags) lines.push(`Tags : ${d.tags}`)
    } else if (d.type === 'port') {
      lines.push(d.label)
      if (d.containers?.length) lines.push(`Conteneurs : ${d.containers.join(', ')}`)
    } else if (d.type === 'root') {
      lines.push(d.label)
      if (d.sublabel) lines.push(`IP : ${d.sublabel}`)
    } else if (d.type === 'authelia' || d.type === 'internet') {
      lines.push(d.label)
      if (d.sublabel) lines.push(d.sublabel)
    }
    if (lines.length) showTooltip(event, lines)
  })

  cy.on('mousemove', 'node', moveTooltip)
  cy.on('mouseout', 'node', hideTooltip)

  cy.on('mouseover', 'edge', (event) => {
    const d = event.target.data()
    const lines = []
    if (d.edgeType === 'inferred') {
      lines.push(`Lien inféré (${d.linkType || 'unknown'})`)
      if (d.envKey) lines.push(`Variable : ${d.envKey}`)
      if (d.confidence) lines.push(`Confiance : ${d.confidence}%`)
    } else if (d.edgeType === 'internet' && d.externalPort) {
      lines.push(`Exposé Internet : port ${d.externalPort}`)
    }
    if (lines.length) showTooltip(event, lines)
  })
  cy.on('mousemove', 'edge', moveTooltip)
  cy.on('mouseout', 'edge', hideTooltip)
}

function resetLayout() {
  if (!cy) return
  cy.layout(getLayoutOptions()).run()
}

function fitView() {
  if (!cy) return
  cy.fit(undefined, 40)
}

defineExpose({ resetLayout, fitView })

onMounted(() => {
  initCytoscape()

  resizeObserver = new ResizeObserver(() => {
    if (cy) {
      cy.resize()
      cy.fit(undefined, 40)
    }
  })
  if (cyContainer.value) resizeObserver.observe(cyContainer.value)
})

onUnmounted(() => {
  if (resizeObserver) resizeObserver.disconnect()
  if (cy) { cy.destroy(); cy = null }
})

// Rebuild graph when data changes — preserve positions of existing nodes
watch(
  [
    () => props.data,
    () => props.services,
    () => props.hostPortOverrides,
    () => props.rootLabel,
    () => props.rootIp,
    () => props.autheliaLabel,
    () => props.autheliaIp,
    () => props.internetLabel,
    () => props.internetIp,
  ],
  () => {
    if (!cy) return

    const hasSavedPositions = Object.keys(props.nodePositions || {}).length > 0

    if (!hasSavedPositions) {
      // No user-saved positions: always run full auto-layout
      const newElements = buildElements()
      cy.elements().remove()
      cy.add(newElements)
      cy.layout(getLayoutOptions()).run()
      return
    }

    // Snapshot current in-memory positions before rebuild
    const currentPositions = {}
    cy.nodes().forEach(n => {
      currentPositions[n.id()] = { x: n.position('x'), y: n.position('y') }
    })

    const newElements = buildElements()
    cy.elements().remove()
    cy.add(newElements)

    // Restore positions: use in-memory first, fallback to prop (DB-loaded)
    let hasNewNodes = false
    cy.nodes().forEach(n => {
      const pos = currentPositions[n.id()] || (props.nodePositions || {})[n.id()]
      if (pos) {
        n.position(pos)
      } else {
        hasNewNodes = true
      }
    })

    if (hasNewNodes) {
      cy.layout({ ...getLayoutOptions(), randomize: false, fit: false }).run()
    }
  },
  { deep: true }
)
</script>

<style scoped>
.cy-graph-container {
  width: 100%;
  height: 100%;
  min-height: 520px;
  position: relative;
  display: flex;
  flex: 1;
  background: radial-gradient(circle at 15% 20%, rgba(96,165,250,0.18), transparent 42%),
    radial-gradient(circle at 70% 0%, rgba(16,185,129,0.12), transparent 40%),
    radial-gradient(circle at 20% 80%, rgba(244,63,94,0.1), transparent 45%),
    #0b0f1a;
  border: 1px solid rgba(148,163,184,0.25);
  border-radius: 16px;
  overflow: hidden;
}

.cy-graph-container::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(circle, rgba(148,163,184,0.22) 1px, transparent 1px);
  background-size: 48px 48px;
  opacity: 0.35;
  pointer-events: none;
  z-index: 0;
}

.cy-canvas {
  width: 100%;
  height: 100%;
  min-height: 520px;
  position: relative;
  z-index: 1;
}

.cy-tooltip {
  position: fixed;
  padding: 10px 12px;
  background-color: rgba(15,23,42,0.94);
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 12px;
  pointer-events: none;
  display: none;
  z-index: 1000;
  white-space: nowrap;
  box-shadow: 0 12px 24px rgba(15,23,42,0.25);
}

.graph-legend {
  position: absolute;
  top: 16px;
  left: 16px;
  padding: 12px 14px;
  background: rgba(15,23,42,0.75);
  border: 1px solid rgba(148,163,184,0.3);
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

.legend-dot.port-tcp     { background: #60a5fa; }
.legend-dot.port-udp     { background: #fb923c; }
.legend-dot.service-node { background: #38bdf8; }

.graph-controls {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  z-index: 2;
}

.graph-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15,23,42,0.75);
  border: 1px solid rgba(148,163,184,0.3);
  border-radius: 8px;
  color: #94a3b8;
  cursor: pointer;
  backdrop-filter: blur(8px);
  transition: background 0.15s, color 0.15s;
}

.graph-btn:hover {
  background: rgba(15,23,42,0.9);
  color: #e2e8f0;
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
</style>

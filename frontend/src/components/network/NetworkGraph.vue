<template>
  <div class="cy-graph-container">
    <!-- Legend -->
    <div class="graph-legend card">
      <div class="legend-title">
        Légende
      </div>
      <div class="legend-item">
        <span class="legend-box root-box" />
        Reverse proxy
      </div>
      <div class="legend-item">
        <span class="legend-box host-box" />
        Hôte
      </div>
      <div class="legend-item">
        <span class="legend-dot online-dot" />
        En ligne
      </div>
      <div class="legend-item">
        <span class="legend-dot offline-dot" />
        Hors ligne
      </div>
      <div class="legend-item">
        <span class="legend-dot service-node" />
        Service proxy
      </div>
      <div class="legend-item">
        <span class="legend-dot port-tcp" />
        Port TCP
      </div>
      <div class="legend-item">
        <span class="legend-dot port-udp" />
        Port UDP
      </div>
      <div
        v-if="hasAutheliaTargets"
        class="legend-item"
      >
        <span class="legend-dash proxy-authelia-dash" />
        Proxy → {{ autheliaLabel || 'Authelia' }}
      </div>
      <div
        v-if="hasAutheliaTargets"
        class="legend-item"
      >
        <span class="legend-dash authelia-dash" />
        {{ autheliaLabel || 'Authelia' }} → service
      </div>
      <div
        v-if="hasInternetTargets"
        class="legend-item"
      >
        <span class="legend-dash internet-proxy-dash" />
        Internet → Proxy
      </div>
    </div>

    <!-- Controls -->
    <div class="graph-controls">
      <button
        class="btn btn-sm btn-outline-secondary graph-btn"
        title="Zoom +"
        @click="zoomIn"
      >
        <svg
          width="14"
          height="14"
          fill="currentColor"
          viewBox="0 0 16 16"
        >
          <path d="M6.5 1a5.5 5.5 0 1 0 0 11A5.5 5.5 0 0 0 6.5 1zm-4.5 5.5a4.5 4.5 0 1 1 9 0 4.5 4.5 0 0 1-9 0z" />
          <path d="M6.5 3.5a.5.5 0 0 1 .5.5V6h2a.5.5 0 0 1 0 1H7v2a.5.5 0 0 1-1 0V7H4a.5.5 0 0 1 0-1h2V4a.5.5 0 0 1 .5-.5zm5.35 4.85a.5.5 0 0 1 .707 0l3.5 3.5a.5.5 0 0 1-.707.707l-3.5-3.5a.5.5 0 0 1 0-.707z" />
        </svg>
      </button>
      <button
        class="btn btn-sm btn-outline-secondary graph-btn"
        title="Zoom −"
        @click="zoomOut"
      >
        <svg
          width="14"
          height="14"
          fill="currentColor"
          viewBox="0 0 16 16"
        >
          <path d="M6.5 1a5.5 5.5 0 1 0 0 11A5.5 5.5 0 0 0 6.5 1zm-4.5 5.5a4.5 4.5 0 1 1 9 0 4.5 4.5 0 0 1-9 0z" />
          <path d="M4 6.5a.5.5 0 0 1 .5-.5h4a.5.5 0 0 1 0 1h-4a.5.5 0 0 1-.5-.5zm5.35 1.85a.5.5 0 0 1 .707 0l3.5 3.5a.5.5 0 0 1-.707.707l-3.5-3.5a.5.5 0 0 1 0-.707z" />
        </svg>
      </button>
      <button
        class="btn btn-sm btn-outline-secondary graph-btn"
        title="Ajuster à l'écran"
        @click="fitView"
      >
        <svg
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          viewBox="0 0 24 24"
        >
          <path d="M3 3h6M3 3v6M21 3h-6M21 3v6M3 21h6M3 21v-6M21 21h-6M21 21v-6" />
        </svg>
      </button>
      <button
        class="btn btn-sm btn-outline-secondary graph-btn"
        title="Réinitialiser la disposition"
        @click="resetLayout"
      >
        <svg
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          viewBox="0 0 24 24"
        >
          <path d="M20 11a8.1 8.1 0 0 0-15.5-2m-.5-4v4h4" />
          <path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" />
        </svg>
      </button>
    </div>

    <div
      v-if="!hasData"
      class="graph-empty"
    >
      <div class="empty-title">
        Aucune topologie disponible
      </div>
      <div class="empty-subtitle">
        Les hôtes actifs apparaîtront ici dès que les données remontent.
      </div>
    </div>

    <div
      ref="tooltipRef"
      class="cy-tooltip"
    />
    <div
      ref="cyContainer"
      class="cy-canvas"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import cytoscape from 'cytoscape'
// @ts-expect-error cytoscape-fcose lacks types
import fcose from 'cytoscape-fcose'
import { createCytoscapeInstance, bindCytoscapeResize, destroyCytoscapeInstance } from '../../composables/useCytoscape'
import { buildNetworkElements, type NetworkHost, type NetworkService, type HostPortOverride } from './buildNetworkElements'
import { getNetworkGraphStyle } from './networkGraphStyle'

cytoscape.use(fcose)

const props = withDefaults(defineProps<{
  data: NetworkHost[]
  rootLabel?: string
  rootIp?: string
  services?: NetworkService[]
  hostPortOverrides?: Record<string, HostPortOverride>
  autheliaLabel?: string
  autheliaIp?: string
  internetLabel?: string
  internetIp?: string
  nodePositions?: Record<string, { x: number; y: number }>
  rootHostId?: string
  autheliaHostId?: string
  rootPortId?: string
  autheliaPortId?: string
}>(), {
  rootLabel: 'root',
  rootIp: '',
  services: () => [],
  hostPortOverrides: () => ({}),
  autheliaLabel: 'Authelia',
  autheliaIp: '',
  internetLabel: 'Internet',
  internetIp: '',
  nodePositions: () => ({}),
  rootHostId: '',
  autheliaHostId: '',
  rootPortId: '',
  autheliaPortId: '',
})

const emit = defineEmits<{
  (e: 'host-click', hostId: string): void
  (e: 'node-select', data: Record<string, unknown>): void
  (e: 'update:nodePositions', positions: Record<string, { x: number; y: number }>): void
}>()

const cyContainer = ref<HTMLElement | null>(null)
const tooltipRef = ref<HTMLElement | null>(null)
let cy: cytoscape.Core | null = null
let resizeBinding: { disconnect: () => void } | null = null

const hasData = computed(() => Array.isArray(props.data) && props.data.length > 0)

// Canvas (Cytoscape) status hues — kept in sync with the --ss-status-* CSS
// tokens in style.css. Canvas can't read CSS vars, so the hex is duplicated;
// update both when changing a status color.
const statusColors: Record<string, string> = { online: '#2fb344', warning: '#fb923c', offline: '#d63939', unknown: '#94a3b8' }

// Computed: does the data contain authelia/internet targets?
const hasAutheliaTargets = computed(() => props.services.some(s => s.linkToAuthelia))
const hasInternetTargets = computed(() => props.services.some(s => s.exposedToInternet))

function escapeHtml(str: unknown): string {
  if (str == null) return ''
  return String(str)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

// Build Cytoscape elements from props (delegates to the pure builder).
function buildElements() {
  return buildNetworkElements({
    data: props.data,
    services: props.services,
    hostPortOverrides: props.hostPortOverrides,
    rootLabel: props.rootLabel,
    rootIp: props.rootIp,
    autheliaLabel: props.autheliaLabel,
    autheliaIp: props.autheliaIp,
    internetLabel: props.internetLabel,
    internetIp: props.internetIp,
    rootHostId: props.rootHostId,
    autheliaHostId: props.autheliaHostId,
    rootPortId: props.rootPortId,
    autheliaPortId: props.autheliaPortId,
    statusColors,
  })
}

function getLayoutOptions(): Record<string, unknown> {
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

function showTooltip(event: any, lines: string[]): void {
  if (!tooltipRef.value) return
  tooltipRef.value.innerHTML = lines.map(l => `<div>${escapeHtml(l)}</div>`).join('')
  tooltipRef.value.style.display = 'block'
  tooltipRef.value.style.left = `${event.originalEvent.pageX + 12}px`
  tooltipRef.value.style.top = `${event.originalEvent.pageY + 12}px`
}

function hideTooltip(): void {
  if (tooltipRef.value) tooltipRef.value.style.display = 'none'
}

function moveTooltip(event: any): void {
  if (!tooltipRef.value || tooltipRef.value.style.display !== 'block') return
  tooltipRef.value.style.left = `${event.originalEvent.pageX + 12}px`
  tooltipRef.value.style.top = `${event.originalEvent.pageY + 12}px`
}

let positionSaveTimeout: ReturnType<typeof setTimeout> | null = null
function emitPositions(): void {
  if (!cy) return
  if (positionSaveTimeout) clearTimeout(positionSaveTimeout)
  positionSaveTimeout = setTimeout(() => {
    if (!cy) return
    const positions: Record<string, { x: number; y: number }> = {}
    cy.nodes().forEach((n: any) => {
      const pos = n.position()
      positions[n.id()] = { x: Math.round(pos.x), y: Math.round(pos.y) }
    })
    emit('update:nodePositions', positions)
  }, 600)
}

function initCytoscape(): void {
  if (!cyContainer.value) return
  if (cy) cy.destroy()

  const savedPositions = props.nodePositions || {}
  const hasPositions = Object.keys(savedPositions).length > 0

  cy = createCytoscapeInstance({
    container: cyContainer.value,
    elements: buildElements(),
    style: getNetworkGraphStyle(),
    layout: hasPositions ? { name: 'preset', fit: true, padding: 48 } : (getLayoutOptions() as any),
    minZoom: 0.2,
    maxZoom: 3,
    wheelSensitivity: 0.3,
    boxSelectionEnabled: false,
  })

  if (hasPositions) {
    cy.nodes().forEach((n: any) => {
      const pos = savedPositions[n.id()]
      if (pos) n.position(pos)
    })
    cy.fit(undefined, 48)
  }

  cy.on('dragfree', 'node', emitPositions)

  cy.on('tap', 'node', (event: any) => {
    emit('node-select', { ...event.target.data() })
  })

  // Click on host → emit event
  cy.on('tap', 'node[type="host"]', (event: any) => {
    const hostId = event.target.data('hostId')
    if (hostId) emit('host-click', hostId)
  })

  // Hover tooltips
  cy.on('mouseover', 'node', (event: any) => {
    const d = event.target.data()
    const lines: string[] = []
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

  cy.on('mouseover', 'edge', (event: any) => {
    const d = event.target.data()
    const lines: string[] = []
    if (d.edgeType === 'internet-proxy') {
      lines.push('Trafic Internet → Proxy')
      if (d.ports?.length) lines.push(`Ports exposés : ${d.ports.map((p: number) => `:${p}`).join(', ')}`)
    } else if (d.edgeType === 'proxy-authelia') {
      lines.push('Proxy → Authelia')
      lines.push('Vérification d\'authentification avant routage')
    } else if (d.edgeType === 'internet' && d.externalPort) {
      lines.push(`Exposé directement sur Internet : port ${d.externalPort}`)
    } else if (d.edgeType === 'proxy') {
      lines.push('Proxy → Service (route directe)')
    } else if (d.edgeType === 'authelia') {
      lines.push('Authelia → Service (accès autorisé)')
    }
    if (lines.length) showTooltip(event, lines)
  })
  cy.on('mousemove', 'edge', moveTooltip)
  cy.on('mouseout', 'edge', hideTooltip)
}

function resetLayout() {
  if (!cy) return
  // fcose options aren't part of cytoscape's typed LayoutOptions union.
  cy.layout(getLayoutOptions() as any).run()
}

function fitView() {
  if (!cy) return
  cy.fit(undefined, 40)
}

function zoomIn() {
  if (!cy) return
  cy.zoom({ level: cy.zoom() * 1.3, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } })
}

function zoomOut() {
  if (!cy) return
  cy.zoom({ level: cy.zoom() / 1.3, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } })
}

defineExpose({ resetLayout, fitView, zoomIn, zoomOut })

onMounted(() => {
  initCytoscape()

  if (cyContainer.value) {
    resizeBinding = bindCytoscapeResize(cyContainer.value, () => {
      if (!cy) return
      cy.resize()
      cy.fit(undefined, 40)
    })
  }
})

onUnmounted(() => {
  if (resizeBinding) resizeBinding.disconnect()
  destroyCytoscapeInstance(cy)
  cy = null
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
    () => props.rootHostId,
    () => props.autheliaHostId,
    () => props.rootPortId,
    () => props.autheliaPortId,
  ],
  () => {
    if (!cy) return

    const hasSavedPositions = Object.keys(props.nodePositions || {}).length > 0

    if (!hasSavedPositions) {
      // No user-saved positions: always run full auto-layout
      const newElements = buildElements()
      cy.elements().remove()
      cy.add(newElements)
      cy.layout(getLayoutOptions() as any).run()
      return
    }

    // Snapshot current in-memory positions before rebuild
    const currentPositions: Record<string, { x: number; y: number }> = {}
    cy.nodes().forEach((n: any) => {
      currentPositions[n.id()] = { x: n.position('x'), y: n.position('y') }
    })

    const newElements = buildElements()
    cy.elements().remove()
    cy.add(newElements)

    // Restore positions: use in-memory first, fallback to prop (DB-loaded)
    let hasNewNodes = false
    cy.nodes().forEach((n: any) => {
      const pos = currentPositions[n.id()] || (props.nodePositions || {})[n.id()]
      if (pos) {
        n.position(pos)
      } else {
        hasNewNodes = true
      }
    })

    if (hasNewNodes) {
      cy.layout({ ...getLayoutOptions(), randomize: false, fit: false } as any).run()
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
  border: 1px solid var(--ss-border-default);
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
  background-color: var(--ss-panel-strong);
  color: var(--ss-text-on-dark);
  border-radius: 8px;
  font-size: 12px;
  pointer-events: none;
  display: none;
  z-index: 1000;
  white-space: nowrap;
  box-shadow: var(--ss-shadow-card);
}

.graph-legend {
  position: absolute;
  top: 16px;
  left: 16px;
  padding: 12px 14px;
  background: var(--ss-panel-strong);
  border: 1px solid var(--ss-border-default);
  border-radius: 12px;
  backdrop-filter: blur(10px);
  z-index: 2;
  font-size: 12px;
  color: var(--ss-text-on-dark);
  min-width: 160px;
}

.legend-box {
  display: inline-block;
  width: 20px;
  height: 12px;
  border-radius: 3px;
  flex-shrink: 0;
}

.host-box {
  border: 1.5px solid var(--ss-border-strong);
  background: var(--ss-panel-medium);
}

.root-box {
  border: 1.5px solid #94a3b8;
  background: var(--ss-panel-strong);
}

.online-dot {
  background: var(--ss-status-online);
}

.offline-dot {
  background: var(--ss-status-offline);
}

.legend-dash {
  display: inline-block;
  width: 18px;
  height: 3px;
  border-radius: 0;
  flex-shrink: 0;
}

.authelia-dash {
  background: transparent;
  border-top: 2px dashed #8b5cf6;
}

.proxy-authelia-dash {
  background: transparent;
  border-top: 2px dashed #8b5cf6;
  opacity: 0.6;
}

.internet-proxy-dash {
  background: #fb923c;
  border-top: 2px solid #fb923c;
  height: 3px;
}

.legend-title {
  font-weight: 700;
  margin-bottom: 6px;
  letter-spacing: 0.4px;
  text-transform: uppercase;
  font-size: 11px;
  color: var(--ss-text-muted-on-dark);
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
  gap: 4px;
  z-index: 2;
}

.graph-btn {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ss-panel-strong);
  border: 1px solid var(--ss-border-default);
  border-radius: 8px;
  color: var(--ss-text-muted-on-dark);
  cursor: pointer;
  backdrop-filter: blur(8px);
  transition: background 0.15s, color 0.15s;
}

.graph-btn:hover {
  background: var(--ss-panel-strong);
  color: var(--ss-text-on-dark);
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
  color: var(--ss-text-muted-on-dark);
  padding: 24px;
}

.empty-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--ss-text-strong);
}

.empty-subtitle {
  margin-top: 6px;
  font-size: 13px;
}
</style>

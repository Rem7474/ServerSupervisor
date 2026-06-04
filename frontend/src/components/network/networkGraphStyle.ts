import type cytoscape from 'cytoscape'

/**
 * getNetworkGraphStyle returns the Cytoscape stylesheet for the topology graph
 * (node shapes per type, routing edge styles, proxy/Authelia role highlights).
 * Colours are duplicated from the --ss-* CSS tokens because the canvas renderer
 * cannot read CSS variables — keep them in sync with style.css.
 */
export function getNetworkGraphStyle(): cytoscape.CytoscapeOptions['style'] {
  const style = [
    {
      selector: 'node',
      style: {
        'font-family': 'system-ui, sans-serif',
        'color': '#e2e8f0',
        'text-wrap': 'wrap',
        'text-max-width': '180px',
        'text-valign': 'center',
        'text-halign': 'center',
        'overlay-padding': '4px',
      },
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
        'color': '#e2e8f0',
      },
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
        'color': '#e2e8f0',
      },
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
        'color': '#e2e8f0',
      },
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
        'color': '#e2e8f0',
      },
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
        'color': '#e2e8f0',
      },
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
        'color': '#e2e8f0',
      },
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
        'color': '#c4b5fd',
      },
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
        'color': '#fed7aa',
      },
    },
    {
      selector: 'node:selected',
      style: {
        'border-width': 2.5,
        'border-color': '#f8fafc',
      },
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
        'opacity': 0.7,
      },
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
        'opacity': 0.75,
      },
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
        'opacity': 0.75,
      },
    },
    // internet → proxy: arête agrégée, épaisse, avec label de ports
    {
      selector: 'edge[edgeType="internet-proxy"]',
      style: {
        'line-color': '#fb923c',
        'target-arrow-color': '#fb923c',
        'target-arrow-shape': 'triangle',
        'arrow-scale': 1,
        'width': 2.5,
        'curve-style': 'bezier',
        'opacity': 0.9,
        'label': 'data(label)',
        'font-size': '10px',
        'font-family': 'ui-monospace, monospace',
        'color': '#fb923c',
        'text-background-color': '#0b0f1a',
        'text-background-opacity': 0.85,
        'text-background-padding': '3px',
        'text-rotation': 'autorotate',
        'text-margin-y': -8,
      },
    },
    // proxy → authelia: arête partagée, tiretée violette
    {
      selector: 'edge[edgeType="proxy-authelia"]',
      style: {
        'line-color': '#8b5cf6',
        'target-arrow-color': '#8b5cf6',
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.85,
        'width': 1.8,
        'line-style': 'dashed',
        'line-dash-pattern': [6, 3],
        'curve-style': 'bezier',
        'opacity': 0.85,
      },
    },
    // Host nodes acting as proxy or authelia (linked via rootHostId / autheliaHostId)
    {
      selector: 'node[role="proxy"]',
      style: {
        'border-color': '#94a3b8',
        'border-width': 2.5,
        'border-style': 'solid',
        'background-color': 'rgba(15,23,42,0.65)',
      },
    },
    {
      selector: 'node[role="authelia"]',
      style: {
        'border-color': '#8b5cf6',
        'border-width': 2.5,
        'border-style': 'solid',
        'background-color': 'rgba(139,92,246,0.10)',
      },
    },
    {
      selector: 'edge:selected',
      style: { 'opacity': 1, 'width': 2.5 },
    },
  ]
  // Cytoscape's stylesheet value types are stricter than these literals (numbers,
  // dash arrays…); the runtime accepts them, so cast once here rather than fight
  // the per-property types.
  return style as unknown as cytoscape.CytoscapeOptions['style']
}

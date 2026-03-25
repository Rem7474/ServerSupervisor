import cytoscape from 'cytoscape'

type ResizeBinding = {
  observer: ResizeObserver
  disconnect: () => void
}

export function createCytoscapeInstance(options: cytoscape.CytoscapeOptions): cytoscape.Core {
  return cytoscape(options)
}

export function bindCytoscapeResize(
  container: HTMLElement,
  onResize: () => void
): ResizeBinding {
  const observer = new ResizeObserver(() => {
    onResize()
  })
  observer.observe(container)
  return {
    observer,
    disconnect: () => observer.disconnect(),
  }
}

export function destroyCytoscapeInstance(instance: cytoscape.Core | null): void {
  if (!instance) return
  instance.destroy()
}

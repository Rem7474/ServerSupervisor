// Theme-aware chart palette for Chart.js options.
//
// Reads the current Tabler theme via CSS variables on the document root so that
// tooltips, grid lines, and tick labels stay legible in both light and dark modes.
// Dark mode is the only active theme today, but resolving via vars means the
// charts follow Tabler automatically if a light theme is ever introduced.

export interface ChartThemePalette {
  legendText: string
  tickText: string
  grid: string
  tooltipBackground: string
  tooltipText: string
  tooltipBorder: string
}

const FALLBACK_DARK: ChartThemePalette = {
  legendText: '#c9d6ea',
  tickText: '#9aa4b8',
  grid: 'rgba(148,163,184,0.12)',
  tooltipBackground: 'rgba(15,23,42,0.95)',
  tooltipText: '#f1f5f9',
  tooltipBorder: 'rgba(148,163,184,0.35)',
}

const FALLBACK_LIGHT: ChartThemePalette = {
  legendText: '#1f2937',
  tickText: '#6b7280',
  grid: 'rgba(15,23,42,0.08)',
  tooltipBackground: 'rgba(255,255,255,0.96)',
  tooltipText: '#1f2937',
  tooltipBorder: 'rgba(15,23,42,0.12)',
}

// cssVar resolves a CSS custom property by checking <body> first, then
// <html>. Tabler attaches the [data-bs-theme="dark"] selector to <body>, so
// reading only from documentElement returns the light defaults inherited from
// :root which leads to white tooltips on the dark theme.
function cssVar(name: string, fallback: string): string {
  if (typeof window === 'undefined' || typeof document === 'undefined') return fallback
  const fromBody = document.body
    ? window.getComputedStyle(document.body).getPropertyValue(name).trim()
    : ''
  if (fromBody) return fromBody
  const fromRoot = window.getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return fromRoot || fallback
}

function isDarkTheme(): boolean {
  if (typeof document === 'undefined') return true
  const bodyTheme = document.body?.getAttribute('data-bs-theme')
  if (bodyTheme) return bodyTheme !== 'light'
  return document.documentElement.getAttribute('data-bs-theme') !== 'light'
}

export function getChartPalette(): ChartThemePalette {
  const dark = isDarkTheme()
  const fallback = dark ? FALLBACK_DARK : FALLBACK_LIGHT
  const bodyColor = cssVar('--tblr-body-color', fallback.legendText)
  const borderColor = cssVar('--tblr-border-color', fallback.tooltipBorder)

  // Force an opaque dark/light tooltip background regardless of what Tabler
  // exposes via --tblr-bg-surface, because a translucent or near-page-color
  // tooltip is illegible over a chart of the same hue.
  const tooltipBackground = dark
    ? FALLBACK_DARK.tooltipBackground
    : FALLBACK_LIGHT.tooltipBackground

  return {
    legendText: bodyColor,
    tickText: cssVar('--tblr-secondary', fallback.tickText) || bodyColor,
    grid: fallback.grid,
    tooltipBackground,
    tooltipText: dark ? FALLBACK_DARK.tooltipText : FALLBACK_LIGHT.tooltipText,
    tooltipBorder: borderColor,
  }
}

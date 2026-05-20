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

function cssVar(name: string, fallback: string): string {
  if (typeof window === 'undefined' || typeof document === 'undefined') return fallback
  const value = window.getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

function isDarkTheme(): boolean {
  if (typeof document === 'undefined') return true
  return document.documentElement.getAttribute('data-bs-theme') !== 'light'
}

export function getChartPalette(): ChartThemePalette {
  const fallback = isDarkTheme() ? FALLBACK_DARK : FALLBACK_LIGHT
  const bodyColor = cssVar('--tblr-body-color', fallback.legendText)
  const bgSurface = cssVar('--tblr-bg-surface', fallback.tooltipBackground)
  const borderColor = cssVar('--tblr-border-color', fallback.tooltipBorder)

  return {
    legendText: bodyColor,
    tickText: cssVar('--tblr-secondary', fallback.tickText) || bodyColor,
    grid: fallback.grid,
    tooltipBackground: bgSurface,
    tooltipText: bodyColor,
    tooltipBorder: borderColor,
  }
}

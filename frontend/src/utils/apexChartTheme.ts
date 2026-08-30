// Theme-aware chart palette for ApexCharts options.
//
// Consolidates what used to be two independent implementations: this file's
// predecessor (chartTheme.ts, used by 3/10 charts) and a second, more robust
// system that lived only inside composables/useDashboard.ts (used by the
// dashboard trend chart alone). Reads Tabler's CSS custom properties so
// charts follow the current theme automatically — dark-only today, but
// resolving via vars means a future light theme would be picked up without
// touching chart code.
//
// Two entry points:
// - getApexChartPalette(): resolves the palette once, for the (large)
//   majority of charts that build their options a single time per data
//   refresh — same usage shape as the old getChartPalette()/
//   getDashboardChartPalette().
// - useReactiveApexChartPalette(): a composable wrapping the same resolution
//   in a MutationObserver that re-resolves on a live [data-bs-theme]/class
//   flip. Only the dashboard needs this today, but it's exported as a normal
//   composable (not dashboard-specific) so another chart can opt in later
//   without re-inventing the observer wiring.

import { ref, onMounted, onBeforeUnmount, defineAsyncComponent, type Ref } from 'vue'
import type { ApexOptions } from 'apexcharts'

// vue3-apexcharts ships no usable TypeScript types for the component
// instance itself; this covers the one method every chart consumer that
// pushes a data update via the exposed instance (as opposed to remounting
// on new options) actually calls.
export interface ApexChartInstance {
  updateOptions(options: ApexOptions, redrawPaths?: boolean, animate?: boolean, updateSyncedCharts?: boolean): Promise<void>
}

// Every chart component lazy-loads vue3-apexcharts the same way, so the
// dependency stays out of the main bundle (see vite.config.js's
// vendor-apexcharts manual chunk) — one shared const instead of each file
// re-declaring an identical loader.
export const AsyncApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))
// Vue's <script setup> compiler infers a component's name from its local
// `const X = ...` declaration for devtools/Vue Test Utils stub-by-name
// matching — but only for components declared directly inside the SFC, not
// ones imported from elsewhere (this used to be such a local declaration in
// every consumer). Naming it explicitly keeps `global.stubs: { ApexChart }`
// working in every spec that stubs it, regardless of which file imports it.
;(AsyncApexChart as { name?: string }).name = 'ApexChart'
;(AsyncApexChart as { name?: string }).name = 'ApexChart'

export interface ApexChartPalette {
  legendText: string
  tickText: string
  grid: string
  // Semantic series colors shared across every chart that needs one, so
  // color choices stay token-driven instead of each chart file hardcoding
  // its own --tblr-* read (the pre-migration state for disk/network charts).
  cpu: string
  ram: string
  disk: string
  networkRx: string
  networkTx: string
}

// `border` isn't part of the exported palette — it only seeds the
// --tblr-border-color lookup used to compute `grid` below. Tooltip chrome
// (background/border/text) is no longer resolved here: it's ApexCharts' own
// theme-dark tooltip (see `theme: { mode: 'dark' }` in each chart's options),
// which ships hardcoded --apx-tt-* colors in apexcharts.css rather than
// reading Tabler tokens — there is no CSS-var passthrough for it to hook
// into, so duplicating a Tabler-derived color here would just drift from
// what's actually rendered.
const FALLBACK_DARK: ApexChartPalette & { border: string } = {
  legendText: '#c9d6ea',
  tickText: '#9aa4b8',
  grid: 'rgba(148,163,184,0.12)',
  border: 'rgba(148,163,184,0.35)',
  cpu: '#206bc4',
  ram: '#2fb344',
  disk: '#f59f00', // --tblr-yellow — matches DiskHistoryChart's pre-migration color
  networkRx: '#4299e1', // --tblr-azure — CVD-safe RX/TX pair, preserve literally (see NetworkFlowsHistoryChart)
  networkTx: '#0ca678', // --tblr-teal
}

const FALLBACK_LIGHT: ApexChartPalette & { border: string } = {
  legendText: '#1f2937',
  tickText: '#6b7280',
  grid: 'rgba(15,23,42,0.08)',
  border: 'rgba(15,23,42,0.12)',
  cpu: '#206bc4',
  ram: '#2fb344',
  disk: '#f59f00',
  networkRx: '#4299e1',
  networkTx: '#0ca678',
}

function getThemeStyles(): { body: CSSStyleDeclaration | null; root: CSSStyleDeclaration | null } | null {
  if (typeof window === 'undefined' || typeof document === 'undefined') return null
  return {
    body: document.body ? window.getComputedStyle(document.body) : null,
    root: window.getComputedStyle(document.documentElement),
  }
}

// Tabler attaches [data-bs-theme] to <body>, so a var lookup must check body
// first — reading only documentElement would silently fall back to the
// light-theme defaults inherited from :root.
function getCssVarValue(name: string, fallback: string): string {
  const styles = getThemeStyles()
  if (!styles) return fallback
  const value = styles.body?.getPropertyValue(name).trim() || styles.root?.getPropertyValue(name).trim() || ''
  return value || fallback
}

function isDarkRgbColor(color: string): boolean {
  const rgb = color.match(/^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/i)
  const rgba = color.match(/^rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9]*\.?[0-9]+)\s*\)$/i)
  const values = rgb || rgba
  if (!values) return false

  const r = Number(values[1])
  const g = Number(values[2])
  const b = Number(values[3])
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance < 0.5
}

// Resolves any CSS color value — including a var() chain — to a concrete
// rgb()/rgba() string, via an off-screen probe element. Needed because
// ApexCharts (like Chart.js before it) consumes plain color strings, not
// live CSS custom properties.
function resolveCssColorForCanvas(color: string, fallback: string): string {
  if (!color) return fallback
  if (typeof window === 'undefined' || typeof document === 'undefined') return fallback

  const probe = document.createElement('span')
  probe.style.color = color
  probe.style.position = 'fixed'
  probe.style.left = '-9999px'
  probe.style.top = '-9999px'
  probe.style.visibility = 'hidden'

  document.body.appendChild(probe)
  const resolved = window.getComputedStyle(probe).color.trim()
  document.body.removeChild(probe)

  if (!resolved || resolved === 'rgba(0, 0, 0, 0)' || resolved === 'transparent') {
    return fallback
  }
  return resolved
}

function toRgba(color: string, alpha: number, fallback: string): string {
  const clamped = Math.max(0, Math.min(1, alpha))
  const hex = color.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i)
  if (hex) {
    const raw = hex[1]
    const normalized = raw.length === 3 ? raw.split('').map((c) => c + c).join('') : raw
    const int = Number.parseInt(normalized, 16)
    const r = (int >> 16) & 255
    const g = (int >> 8) & 255
    const b = int & 255
    return `rgba(${r},${g},${b},${clamped})`
  }

  const rgb = color.match(/^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/i)
  if (rgb) {
    return `rgba(${rgb[1]},${rgb[2]},${rgb[3]},${clamped})`
  }

  const rgba = color.match(/^rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9]*\.?[0-9]+)\s*\)$/i)
  if (rgba) {
    return `rgba(${rgba[1]},${rgba[2]},${rgba[3]},${clamped})`
  }

  return fallback
}

export function getApexChartPalette(): ApexChartPalette {
  const pageBackground = resolveCssColorForCanvas(
    getCssVarValue('--tblr-bg-surface', getCssVarValue('--tblr-body-bg', '#111827')),
    '#111827',
  )
  const dark = isDarkRgbColor(pageBackground)
  const fallback = dark ? FALLBACK_DARK : FALLBACK_LIGHT

  const lightText = resolveCssColorForCanvas(getCssVarValue('--tblr-light', '#f8fafc'), '#f8fafc')
  const darkText = resolveCssColorForCanvas(getCssVarValue('--tblr-dark', '#111827'), '#111827')
  const textOnPage = dark ? lightText : darkText

  const gridBase = resolveCssColorForCanvas(
    getCssVarValue('--tblr-border-color', fallback.border),
    fallback.border,
  )

  const cpu = resolveCssColorForCanvas(getCssVarValue('--tblr-primary', fallback.cpu), fallback.cpu)
  const ram = resolveCssColorForCanvas(getCssVarValue('--tblr-success', fallback.ram), fallback.ram)
  const disk = resolveCssColorForCanvas(getCssVarValue('--tblr-yellow', fallback.disk), fallback.disk)
  const networkRx = resolveCssColorForCanvas(getCssVarValue('--tblr-azure', fallback.networkRx), fallback.networkRx)
  const networkTx = resolveCssColorForCanvas(getCssVarValue('--tblr-teal', fallback.networkTx), fallback.networkTx)

  return {
    legendText: textOnPage,
    tickText: toRgba(textOnPage, 0.82, textOnPage),
    grid: toRgba(gridBase, 0.35, fallback.grid),
    cpu,
    ram,
    disk,
    networkRx,
    networkTx,
  }
}

// Opt-in reactive layer: only a consumer that explicitly needs live
// theme-flip updates should call this (today: only the dashboard trend
// chart). Everything else calls getApexChartPalette() once per options
// build, exactly like before.
export function useReactiveApexChartPalette(): Ref<ApexChartPalette> {
  const palette = ref<ApexChartPalette>(getApexChartPalette()) as Ref<ApexChartPalette>
  let observer: MutationObserver | null = null

  onMounted(() => {
    palette.value = getApexChartPalette()
    if (typeof MutationObserver === 'undefined' || typeof document === 'undefined') return
    observer = new MutationObserver(() => {
      palette.value = getApexChartPalette()
    })
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-bs-theme', 'class'] })
    if (document.body) {
      observer.observe(document.body, { attributes: true, attributeFilter: ['data-bs-theme', 'class'] })
    }
  })

  onBeforeUnmount(() => {
    observer?.disconnect()
    observer = null
  })

  return palette
}

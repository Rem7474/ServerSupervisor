import { describe, it, expect, afterEach } from 'vitest'
import { getApexChartPalette } from './apexChartTheme'

// Real-browser test: Chromium's getComputedStyle() returns Tabler 1.6's
// oklch() colors verbatim, and ApexCharts only derives a translucent area fill
// from hex/rgb input. The palette must therefore hand out plain sRGB strings.
describe('getApexChartPalette (browser / oklch theme colors)', () => {
  afterEach(() => {
    document.documentElement.style.removeProperty('--tblr-primary')
    document.documentElement.style.removeProperty('--tblr-bg-surface')
    document.documentElement.style.removeProperty('--tblr-light')
    document.documentElement.style.removeProperty('--tblr-dark')
  })

  it('converts oklch series colors to rgb()', () => {
    document.documentElement.style.setProperty('--tblr-primary', 'oklch(0.546 0.1724 254.2)')
    const palette = getApexChartPalette()
    expect(palette.cpu).toMatch(/^rgba?\(/)
    expect(palette.cpu).not.toContain('oklch')
  })

  it('detects a dark oklch surface and picks the light legend text', () => {
    document.documentElement.style.setProperty('--tblr-bg-surface', 'oklch(0.2 0.02 260)')
    document.documentElement.style.setProperty('--tblr-light', '#f8fafc')
    document.documentElement.style.setProperty('--tblr-dark', '#111827')
    const palette = getApexChartPalette()
    expect(palette.legendText).toBe('rgb(248, 250, 252)')
  })
})

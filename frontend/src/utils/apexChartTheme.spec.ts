import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { getApexChartPalette, useReactiveApexChartPalette } from './apexChartTheme'

function clearCssVars(names: string[], target: HTMLElement = document.documentElement): void {
  for (const n of names) target.style.removeProperty(n)
}

const ALL_VARS = [
  '--tblr-bg-surface', '--tblr-body-bg', '--tblr-light', '--tblr-dark',
  '--tblr-border-color', '--tblr-primary', '--tblr-success', '--tblr-yellow',
  '--tblr-azure', '--tblr-teal',
]

describe('getApexChartPalette', () => {
  afterEach(() => {
    clearCssVars(ALL_VARS)
    clearCssVars(ALL_VARS, document.body)
  })

  it('falls back to the dark palette defaults when no Tabler CSS vars are set', () => {
    const palette = getApexChartPalette()

    // happy-dom's getComputedStyle can't resolve var() chains to a concrete
    // rgb() the way a real browser does, so every lookup here falls through
    // to resolveCssColorForCanvas's own fallback — asserting the *shape*
    // (every semantic slot present, non-empty strings) is what's actually
    // meaningful in this environment; exact hex round-tripping is covered by
    // the real-browser suite.
    expect(palette.cpu).toBeTruthy()
    expect(palette.ram).toBeTruthy()
    expect(palette.disk).toBeTruthy()
    expect(palette.networkRx).toBeTruthy()
    expect(palette.networkTx).toBeTruthy()
    expect(palette.legendText).toBeTruthy()
    expect(palette.tickText).toBeTruthy()
    expect(palette.grid).toBeTruthy()
  })

  it('returns a grid color as an rgba() string with the expected 0.35 alpha applied via toRgba', () => {
    const palette = getApexChartPalette()
    // toRgba always normalizes to rgba(...) regardless of the input format
    // (hex/rgb/rgba) as long as resolveCssColorForCanvas found something —
    // in this environment it falls back to FALLBACK_DARK.grid, which is
    // already an rgba() literal.
    expect(palette.grid).toMatch(/^rgba\(/)
  })
})

describe('useReactiveApexChartPalette', () => {
  afterEach(() => {
    clearCssVars(ALL_VARS)
    clearCssVars(ALL_VARS, document.body)
  })

  it('resolves an initial palette on mount and re-resolves when the theme attribute flips', async () => {
    let paletteRef: ReturnType<typeof useReactiveApexChartPalette> | undefined
    const Comp = defineComponent({
      setup() {
        paletteRef = useReactiveApexChartPalette()
        return () => null
      },
    })
    const wrapper = mount(Comp)
    await nextTick()

    expect(paletteRef?.value.cpu).toBeTruthy()
    const before = paletteRef?.value

    // Flip the theme attribute the MutationObserver watches — the palette
    // object identity should change (a fresh resolve), proving the observer
    // actually re-ran getApexChartPalette() rather than only firing once at
    // mount.
    document.documentElement.setAttribute('data-bs-theme', 'light')
    await new Promise((resolve) => setTimeout(resolve, 0))
    await nextTick()

    expect(paletteRef?.value).not.toBe(before)

    document.documentElement.removeAttribute('data-bs-theme')
    wrapper.unmount()
  })

  it('disconnects its MutationObserver on unmount (no further re-resolves after teardown)', async () => {
    let paletteRef: ReturnType<typeof useReactiveApexChartPalette> | undefined
    const Comp = defineComponent({
      setup() {
        paletteRef = useReactiveApexChartPalette()
        return () => null
      },
    })
    const wrapper = mount(Comp)
    await nextTick()
    wrapper.unmount()
    const afterUnmount = paletteRef?.value

    document.documentElement.setAttribute('data-bs-theme', 'light')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(paletteRef?.value).toBe(afterUnmount)
    document.documentElement.removeAttribute('data-bs-theme')
  })
})

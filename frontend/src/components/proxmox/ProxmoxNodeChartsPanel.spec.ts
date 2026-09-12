import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeChartsPanel from './ProxmoxNodeChartsPanel.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('ProxmoxNodeChartsPanel', () => {
  it('renders the translated section title and chart titles', () => {
    const wrapper = mount(ProxmoxNodeChartsPanel)
    expect(wrapper.text()).toContain('Historique RRD')
    expect(wrapper.text()).toContain('Réseau')
    expect(wrapper.text()).toContain('Température CPU')
    expect(wrapper.text()).toContain('RPM Ventilateurs')
  })

  it('renders translated timeframe buttons and emits the raw value on click', async () => {
    const wrapper = mount(ProxmoxNodeChartsPanel)
    const weekButton = wrapper.findAll('button').find((b) => b.text() === '7j')
    expect(weekButton).toBeTruthy()
    await weekButton!.trigger('click')
    expect(wrapper.emitted('timeframe-changed')?.[0]).toEqual(['week'])
  })

  it('falls back to the shared "no data" label for cpu/ram/iowait/network charts with no series', () => {
    const wrapper = mount(ProxmoxNodeChartsPanel)
    // 4 of the 6 charts (CPU/RAM/IO Wait/Network) share the generic fallback.
    const occurrences = wrapper.text().split('Aucune donnée').length - 1
    expect(occurrences).toBeGreaterThanOrEqual(4)
  })

  it('uses the dedicated translated empty-text for temperature and fan charts', () => {
    const wrapper = mount(ProxmoxNodeChartsPanel)
    expect(wrapper.text()).toContain('Aucune donnée température disponible')
    expect(wrapper.text()).toContain('Aucune donnée ventilateur disponible')
  })

  it('honors an explicit tempEmptyText/fanEmptyText override instead of the translated default', () => {
    const wrapper = mount(ProxmoxNodeChartsPanel, {
      props: { tempEmptyText: 'Capteur absent', fanEmptyText: 'Pas de ventilateur' },
    })
    expect(wrapper.text()).toContain('Capteur absent')
    expect(wrapper.text()).toContain('Pas de ventilateur')
  })
})

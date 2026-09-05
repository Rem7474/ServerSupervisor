import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import TrafficKpiCards from './TrafficKpiCards.vue'

function mountCards(compare: Record<string, unknown> = {}) {
  return mount(TrafficKpiCards, {
    props: {
      traffic: { total_requests: 12345, total_bytes: 2 * 1024 * 1024 },
      threats: { suspicious_ips: 7 },
      compare,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('TrafficKpiCards', () => {
  it('renders the translated KPI labels and formatted values', () => {
    const wrapper = mountCards()
    for (const label of ['Requêtes totales', 'Bande passante', 'Taux 5xx', 'IPs suspectes']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('2.0 MB')
  })

  it('shows the translated "N/A" delta when no comparison data is available', () => {
    const wrapper = mountCards()
    expect(wrapper.text()).toContain('N/A vs période précédente')
  })

  it('shows a signed percentage delta with the translated suffix when comparison data is available', () => {
    const wrapper = mountCards({ delta_percent: { total_requests: 12.345 } })
    expect(wrapper.text()).toContain('+12.3% vs période précédente')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mountCards({ delta_percent: { ratio_5xx: -3 } })
    expect(wrapper.text()).toContain('Total requests')
    expect(wrapper.text()).toContain('-3.0% vs previous period')
  })
})

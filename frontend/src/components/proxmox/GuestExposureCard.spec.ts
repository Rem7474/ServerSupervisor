import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getProxmoxGuestExposure } = vi.hoisted(() => ({
  getProxmoxGuestExposure: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { getProxmoxGuestExposure },
}))

import GuestExposureCard from './GuestExposureCard.vue'

const stubs = {
  ExposureDomainsPanel: { props: ['periodLabel', 'subjectLabel'], template: '<div class="exposure-panel">{{ periodLabel }} / {{ subjectLabel }}</div>' },
}

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

describe('GuestExposureCard', () => {
  it('shows the translated empty state when the guest has no known IP', async () => {
    getProxmoxGuestExposure.mockResolvedValue({ data: { ip_address: '' } })
    const wrapper = mount(GuestExposureCard, { props: { guestId: 'g1' }, global: { stubs } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucune adresse IP détectée pour ce guest')
  })

  it('renders the exposure panel with the translated period label and subject label once an IP is known', async () => {
    getProxmoxGuestExposure.mockResolvedValue({ data: { ip_address: '10.0.0.5' } })
    const wrapper = mount(GuestExposureCard, { props: { guestId: 'g1' }, global: { stubs } })
    await flushPromises()

    expect(wrapper.text()).toContain('24h / ce guest')
  })

  it('re-fetches with the translated period value when a period button is clicked', async () => {
    getProxmoxGuestExposure.mockResolvedValue({ data: { ip_address: '10.0.0.5' } })
    const wrapper = mount(GuestExposureCard, { props: { guestId: 'g1' }, global: { stubs } })
    await flushPromises()

    const sevenDayButton = wrapper.findAll('button').find((b) => b.text() === '7j')
    expect(sevenDayButton).toBeTruthy()
    await sevenDayButton!.trigger('click')
    await flushPromises()

    expect(getProxmoxGuestExposure).toHaveBeenLastCalledWith('g1', '168h')
    expect(wrapper.text()).toContain('7j / ce guest')
  })
})

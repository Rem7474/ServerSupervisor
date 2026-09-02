import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AlertReleaseSummary from './AlertReleaseSummary.vue'

const mountOpts = { global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } } }

beforeEach(() => {
  setLocale('fr')
})

describe('AlertReleaseSummary', () => {
  it('shows a loading skeleton while loading', () => {
    const wrapper = mount(AlertReleaseSummary, { props: { loading: true }, ...mountOpts })
    expect(wrapper.findComponent({ name: 'LoadingSkeleton' }).exists()).toBe(true)
  })

  it('shows the error state', () => {
    const wrapper = mount(AlertReleaseSummary, { props: { error: 'boom' }, ...mountOpts })
    expect(wrapper.text()).toContain('boom')
  })

  it('shows the empty state with a create-tracker CTA', () => {
    const wrapper = mount(AlertReleaseSummary, { props: { trackers: [] }, ...mountOpts })
    expect(wrapper.text()).toContain('Aucun tracker configuré')
    expect(wrapper.text()).toContain('Créer un tracker')
  })

  it('shows the disabled badge and the monitoring-only state', () => {
    const wrapper = mount(AlertReleaseSummary, {
      props: {
        trackers: [{ id: 1, name: 'vaultwarden', enabled: false, tracker_type: 'docker' }],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('Surveillance seule')
    expect(wrapper.text()).toContain('Jamais')
  })

  it('shows a check-error indicator when last_error is set', () => {
    const wrapper = mount(AlertReleaseSummary, {
      props: { trackers: [{ id: 1, name: 'vaultwarden', last_error: 'timeout' }] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Erreur lors de la vérification')
  })

  it('shows a translated execution status and target host, with a drift badge', () => {
    const wrapper = mount(AlertReleaseSummary, {
      props: {
        trackers: [{
          id: 1, name: 'vaultwarden', host_id: 'h1',
          last_execution: { status: 'succeeded', tag_name: 'v1.2.3' },
          drift_detected: true,
        }],
      },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Succès')
    expect(wrapper.text()).toContain('v1.2.3')
    expect(wrapper.text()).toContain('Dérive détectée')
  })

  it('falls back to the raw status for an unrecognized execution status', () => {
    const wrapper = mount(AlertReleaseSummary, {
      props: { trackers: [{ id: 1, name: 'vaultwarden', host_id: 'h1', last_execution: { status: 'weird' } }] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('weird')
  })
})

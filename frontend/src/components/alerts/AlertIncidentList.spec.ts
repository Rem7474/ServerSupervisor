import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AlertIncidentList from './AlertIncidentList.vue'
import type { NotificationItem } from '../../types/generated'

beforeEach(() => {
  setLocale('fr')
})

const mountOpts = { global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } } }

function incident(overrides: Partial<NotificationItem> = {}): NotificationItem {
  return {
    id: '1',
    type: 'alert',
    host_id: 'h1',
    host_name: 'web-01',
    rule_name: 'CPU high',
    metric: 'cpu',
    severity: 'crit',
    value: 95,
    triggered_at: '2026-01-01T00:00:00Z',
    browser_notify: false,
    ...overrides,
  }
}

describe('AlertIncidentList', () => {
  it('shows the empty state when there are no incidents at all', () => {
    const wrapper = mount(AlertIncidentList, { props: { incidents: [] }, ...mountOpts })
    expect(wrapper.text()).toContain('Aucune notification enregistrée')
  })

  it('shows the no-match empty state with a reset CTA when a filter excludes everything', async () => {
    const wrapper = mount(AlertIncidentList, { props: { incidents: [incident()] }, ...mountOpts })
    await wrapper.find('input[type="text"]').setValue('nonexistent-xyz')
    expect(wrapper.text()).toContain('Aucune notification ne correspond à cette recherche.')
    expect(wrapper.text()).toContain('Réinitialiser')
  })

  it('pluralizes the active-incidents header badge', () => {
    const wrapper = mount(AlertIncidentList, {
      props: { incidents: [incident()], activeIncidentCount: 3 },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('3 actifs')
  })

  it('renders translated type and status filter labels', () => {
    const wrapper = mount(AlertIncidentList, { props: { incidents: [incident()] }, ...mountOpts })
    expect(wrapper.text()).toContain('Critique')
    expect(wrapper.text()).toContain('Avertissement')
    expect(wrapper.text()).toContain('Tous états')
    expect(wrapper.text()).toContain('En cours')
  })

  it('groups by host and labels ungrouped incidents "Sans hôte"', async () => {
    const wrapper = mount(AlertIncidentList, {
      props: { incidents: [incident({ host_name: '' })] },
      ...mountOpts,
    })
    const groupButton = wrapper.findAll('button').find((b) => b.text().includes('Regrouper par hôte'))
    await groupButton!.trigger('click')
    expect(wrapper.text()).toContain('Sans hôte')
    expect(wrapper.text()).toContain('Vue chronologique')
  })

  it('shows the translated command-status remediation label', () => {
    const wrapper = mount(AlertIncidentList, {
      props: { incidents: [incident({ command_status: 'failed' })] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Remédiation :')
    expect(wrapper.text()).toContain('échouée')
  })

  it('shows the pagination label when there is more than one page', () => {
    const many = Array.from({ length: 55 }, (_, i) => incident({ id: String(i), rule_name: `rule-${i}` }))
    const wrapper = mount(AlertIncidentList, { props: { incidents: many }, ...mountOpts })
    expect(wrapper.text()).toContain('Page 1 / 2')
  })

  it('shows the correlated-incident icon title', () => {
    const wrapper = mount(AlertIncidentList, {
      props: { incidents: [incident({ correlated_with: 'other-id' })] },
      ...mountOpts,
    })
    expect(wrapper.find('[title="Corrélé avec l\'incident « hôte hors ligne » — pas de notification séparée"]').exists()).toBe(true)
  })

  it('falls back to the translated unknown-source label when host_name is empty', () => {
    const wrapper = mount(AlertIncidentList, {
      props: { incidents: [incident({ host_name: '' })] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Source inconnue')
  })
})

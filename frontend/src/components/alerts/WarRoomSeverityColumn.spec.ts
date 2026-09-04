import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import WarRoomSeverityColumn from './WarRoomSeverityColumn.vue'
import type { NotificationItem } from '../../types/generated'

const mountOpts = { global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } } }

function incident(overrides: Partial<NotificationItem>): NotificationItem {
  return {
    id: 'alert:1',
    type: 'alert_incident',
    host_id: 'h1',
    host_name: 'host-1',
    rule_name: 'CPU high',
    metric: 'cpu',
    severity: 'crit',
    value: 95,
    triggered_at: '2026-01-01T00:00:00Z',
    ...overrides,
  } as NotificationItem
}

beforeEach(() => {
  setLocale('fr')
})

describe('WarRoomSeverityColumn', () => {
  it('shows a lowercased empty-state message using the column title', () => {
    const wrapper = mount(WarRoomSeverityColumn, { props: { title: 'Critique', tone: 'danger', items: [] }, ...mountOpts })
    expect(wrapper.text()).toContain('Aucun incident critique actif.')
  })

  it('renders the item title, source and duration', () => {
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({})] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('CPU high')
    expect(wrapper.text()).toContain('host-1')
    expect(wrapper.text()).toContain('Depuis')
  })

  it('falls back to "Source inconnue" when host_name is missing', () => {
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({ host_name: undefined })] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('Source inconnue')
  })

  it('shows the "En cours" badge for an acknowledged incident', () => {
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({ acknowledged_at: '2026-01-01T00:30:00Z' })] },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('En cours')
  })

  it('pluralizes the correlated-count badge and shows its title', () => {
    const counts = new Map([[1, 3]])
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({ id: 'alert:1' })], correlatedCounts: counts },
      ...mountOpts,
    })
    expect(wrapper.text()).toContain('+3 corrélées')
  })

  it('hides admin action buttons for a non-admin', () => {
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({})], isAdmin: false },
      ...mountOpts,
    })
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('emits acknowledge and resolve for an admin', async () => {
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({})], isAdmin: true },
      ...mountOpts,
    })
    await wrapper.find('button[aria-label="Accuser réception de l\'incident"]').trigger('click')
    expect(wrapper.emitted('acknowledge')?.[0]?.[0]).toMatchObject({ id: 'alert:1' })

    await wrapper.find('button[aria-label="Clôturer l\'incident"]').trigger('click')
    expect(wrapper.emitted('resolve')?.[0]?.[0]).toMatchObject({ id: 'alert:1' })
  })

  it('hides the acknowledge button once already acknowledged', () => {
    const wrapper = mount(WarRoomSeverityColumn, {
      props: { title: 'Critique', tone: 'danger', items: [incident({ acknowledged_at: '2026-01-01T00:30:00Z' })], isAdmin: true },
      ...mountOpts,
    })
    expect(wrapper.find('button[aria-label="Accuser réception de l\'incident"]').exists()).toBe(false)
    expect(wrapper.find('button[aria-label="Clôturer l\'incident"]').exists()).toBe(true)
  })
})

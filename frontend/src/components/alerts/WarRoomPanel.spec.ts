import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import WarRoomPanel from './WarRoomPanel.vue'
import type { NotificationItem } from '../../types/generated'

beforeEach(() => {
  setLocale('fr')
})

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
    resolved_at: undefined,
    browser_notify: true,
    ...overrides,
  } as NotificationItem
}

function mountPanel(incidents: NotificationItem[]) {
  return mount(WarRoomPanel, {
    props: { incidents },
    // VTU's shorthand `true` stub drops slot content — router-link's text
    // (the incident title) is exactly what these tests assert on, so stub
    // with a slot-preserving template instead.
    global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } },
  })
}

describe('WarRoomPanel', () => {
  it('shows only active alert incidents, excluding resolved and tracker entries', () => {
    const wrapper = mountPanel([
      incident({ id: 'alert:1', rule_name: 'Active crit' }),
      incident({ id: 'alert:2', rule_name: 'Resolved crit', resolved_at: '2026-01-01T01:00:00Z' }),
      incident({ id: 'tracker:1', type: 'release_tracker_execution', rule_name: 'Tracker item' }),
    ])
    const text = wrapper.text()
    expect(text).toContain('Active crit')
    expect(text).not.toContain('Resolved crit')
    expect(text).not.toContain('Tracker item')
  })

  it('splits incidents into Critique / Avertissement columns by severity', () => {
    const wrapper = mountPanel([
      incident({ id: 'alert:1', rule_name: 'Crit one', severity: 'crit' }),
      incident({ id: 'alert:2', rule_name: 'Warn one', severity: 'warn' }),
    ])
    const text = wrapper.text()
    expect(text).toContain('Critique')
    expect(text).toContain('Avertissement')
    expect(text).toContain('Crit one')
    expect(text).toContain('Warn one')
  })

  it('excludes a correlated child from its own row and counts it on the parent', () => {
    const wrapper = mountPanel([
      incident({ id: 'alert:1', rule_name: 'Host down', severity: 'crit' }),
      incident({ id: 'alert:2', rule_name: 'Container A down', severity: 'crit', correlated_with: 1 }),
      incident({ id: 'alert:3', rule_name: 'Container B down', severity: 'crit', correlated_with: 1 }),
    ])
    const text = wrapper.text()
    expect(text).toContain('Host down')
    expect(text).not.toContain('Container A down')
    expect(text).not.toContain('Container B down')
    expect(text).toContain('+2 corrélées')
  })

  it('sorts each severity column oldest-first', () => {
    const wrapper = mountPanel([
      incident({ id: 'alert:1', rule_name: 'Newer', severity: 'crit', triggered_at: '2026-01-02T00:00:00Z' }),
      incident({ id: 'alert:2', rule_name: 'Older', severity: 'crit', triggered_at: '2026-01-01T00:00:00Z' }),
    ])
    const text = wrapper.text()
    expect(text.indexOf('Older')).toBeLessThan(text.indexOf('Newer'))
  })

  it('shows an empty state when there are no active incidents', () => {
    const wrapper = mountPanel([])
    expect(wrapper.text()).toContain('Aucun incident actif')
  })
})

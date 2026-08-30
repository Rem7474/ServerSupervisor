import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ConnectionsTable from './ConnectionsTable.vue'
import type { LoginEvent } from '../../types/generated'

beforeEach(() => {
  setLocale('fr')
})

const baseEvent: LoginEvent = {
  id: 1,
  username: 'admin',
  ip_address: '203.0.113.10',
  success: true,
  user_agent: 'Mozilla/5.0 (Windows NT 10.0) Chrome/120.0 Safari/537.36',
  created_at: '2026-01-15T10:30:00Z',
}

describe('ConnectionsTable', () => {
  it('shows a loading skeleton instead of rows while loading', () => {
    const wrapper = mount(ConnectionsTable, { props: { loading: true, events: [baseEvent] } })
    expect(wrapper.findComponent({ name: 'LoadingSkeleton' }).exists()).toBe(true)
    expect(wrapper.find('tr[key]').exists()).toBe(false)
  })

  it('shows the empty state when there are no events', () => {
    const wrapper = mount(ConnectionsTable, { props: { events: [] } })
    expect(wrapper.text()).toContain('Aucune connexion enregistrée')
  })

  it('hides the username column by default and shows it when showUsername is set', () => {
    const withoutUsername = mount(ConnectionsTable, { props: { events: [baseEvent] } })
    expect(withoutUsername.text()).not.toContain('Utilisateur')
    expect(withoutUsername.text()).not.toContain('admin')

    const withUsername = mount(ConnectionsTable, { props: { events: [baseEvent], showUsername: true } })
    expect(withUsername.text()).toContain('Utilisateur')
    expect(withUsername.text()).toContain('admin')
  })

  it('renders a successful login as a success badge and a failed one as a failure badge', () => {
    const success = mount(ConnectionsTable, { props: { events: [baseEvent] } })
    expect(success.text()).toContain('Succès')
    expect(success.find('.badge').classes()).toContain('bg-success-lt')

    const failed = mount(ConnectionsTable, { props: { events: [{ ...baseEvent, success: false }] } })
    expect(failed.text()).toContain('Échec')
    expect(failed.find('.badge').classes()).toContain('bg-danger-lt')
  })

  it('renders the parsed browser/OS and the event IP', () => {
    const wrapper = mount(ConnectionsTable, { props: { events: [baseEvent] } })
    expect(wrapper.text()).toContain('Chrome')
    expect(wrapper.text()).toContain('Windows')
    expect(wrapper.text()).toContain('203.0.113.10')
  })
})

import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SystemdTable from './SystemdTable.vue'
import type { SystemdService } from './SystemdTable.vue'

const services: SystemdService[] = [
  { name: 'nginx.service', active_state: 'active', sub_state: 'running', description: 'A high performance web server' },
  { name: 'cron.service', active_state: 'failed', sub_state: 'dead', description: 'Regular background program processing daemon' },
]

beforeEach(() => {
  setLocale('fr')
})

describe('SystemdTable', () => {
  it('renders a row per service with name/state/description', () => {
    const wrapper = mount(SystemdTable, { props: { services } })
    expect(wrapper.text()).toContain('nginx.service')
    expect(wrapper.text()).toContain('cron.service')
    expect(wrapper.text()).toContain('A high performance web server')
  })

  it('shows an empty state when there are no services', () => {
    const wrapper = mount(SystemdTable, { props: { services: [] } })
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.text()).toContain('Aucun service.')
  })

  it('renders the Actions column with start/stop/restart/status buttons by default', () => {
    const wrapper = mount(SystemdTable, { props: { services } })
    expect(wrapper.text()).toContain('Actions')
    // nginx is active -> Arrêter button, cron is not -> Démarrer button
    expect(wrapper.find('[aria-label="Arrêter le service"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="Démarrer le service"]').exists()).toBe(true)
  })

  it('hides the Actions column entirely in readonly mode', () => {
    const wrapper = mount(SystemdTable, { props: { services, readonly: true } })
    expect(wrapper.text()).not.toContain('Actions')
    expect(wrapper.find('[aria-label="Arrêter le service"]').exists()).toBe(false)
    expect(wrapper.find('[aria-label="Démarrer le service"]').exists()).toBe(false)
  })

  it('emits action with the service name and chosen action', async () => {
    const wrapper = mount(SystemdTable, { props: { services } })
    await wrapper.find('[aria-label="Redémarrer le service"]').trigger('click')
    expect(wrapper.emitted('action')?.[0]).toEqual(['nginx.service', 'restart'])
  })
})

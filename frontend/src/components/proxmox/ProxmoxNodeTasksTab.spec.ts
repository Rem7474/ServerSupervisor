import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeTasksTab from './ProxmoxNodeTasksTab.vue'
import type { ProxmoxTask } from '../../types/proxmox'

beforeEach(() => {
  setLocale('fr')
})

function task(overrides: Partial<ProxmoxTask> = {}): ProxmoxTask {
  return {
    id: 't1', connection_id: 'c1', node_name: 'pve1', upid: 'UPID:pve1:...',
    task_type: 'vzstart', status: 'stopped', user_name: 'root@pam',
    start_time: '2026-01-01T10:00:00Z', end_time: '2026-01-01T10:00:05Z',
    exit_status: 'OK', object_id: '100', last_seen_at: '2026-01-01T10:00:05Z',
    ...overrides,
  }
}

describe('ProxmoxNodeTasksTab', () => {
  it('shows the translated empty state when there are no tasks', () => {
    const wrapper = mount(ProxmoxNodeTasksTab, { props: { tasks: [] } })
    expect(wrapper.text()).toContain('Aucune tâche récente pour ce nœud.')
  })

  it('renders translated column headers', () => {
    const wrapper = mount(ProxmoxNodeTasksTab, { props: { tasks: [task()] } })
    for (const label of ['Type', 'Objet', 'Utilisateur', 'Début', 'Durée', 'Statut']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('shows the translated "En cours" status for a running task', () => {
    const wrapper = mount(ProxmoxNodeTasksTab, {
      props: { tasks: [task({ status: 'running', end_time: undefined, exit_status: '' })] },
    })
    expect(wrapper.text()).toContain('En cours')
  })

  it('shows the translated tooltip on the view-logs button and emits view-logs on click', async () => {
    const wrapper = mount(ProxmoxNodeTasksTab, { props: { tasks: [task()] } })
    const button = wrapper.find('button.btn-ghost-secondary')
    expect(button.attributes('title')).toBe('Voir les logs')
    await button.trigger('click')
    expect(wrapper.emitted('view-logs')?.[0][0]).toMatchObject({ upid: 'UPID:pve1:...' })
  })
})

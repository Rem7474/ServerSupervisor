import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CommandLogPanel from './CommandLogPanel.vue'

const baseCommand = {
  host_name: 'web-01',
  module: 'processes',
  action: 'list',
  status: 'completed',
  created_at: new Date().toISOString(),
}

describe('CommandLogPanel', () => {
  it('renders the processes table for a completed module=processes/action=list command', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: {
          ...baseCommand,
          output: JSON.stringify([
            { pid: 1, name: 'systemd', user: 'root', cpu_pct: 0.1, mem_pct: 0.2, mem_rss_kb: 1024, state: 'S' },
          ]),
        },
      },
    })
    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.text()).toContain('systemd')
    expect(wrapper.find('pre.console-output').exists()).toBe(false)
  })

  it('renders the systemd table (read-only, no action buttons) for a completed module=systemd/action=list command', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: {
          ...baseCommand,
          module: 'systemd',
          action: 'list',
          output: JSON.stringify([
            { name: 'nginx.service', active_state: 'active', sub_state: 'running', description: 'web server' },
          ]),
        },
      },
    })
    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.text()).toContain('nginx.service')
    expect(wrapper.text()).not.toContain('Actions')
    expect(wrapper.find('pre.console-output').exists()).toBe(false)
  })

  it('renders a summary card for a completed module=restic/action=run_backup command', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: {
          ...baseCommand,
          module: 'restic',
          action: 'run_backup',
          output: JSON.stringify({ status: 'ok', profile: 'files', duration_sec: 42, snapshot_id: 'abc123' }),
        },
      },
    })
    expect(wrapper.text()).toContain('abc123')
    const badges = wrapper.findAll('.badge')
    expect(badges[badges.length - 1].text()).toBe('ok')
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('pre.console-output').exists()).toBe(false)
  })

  it('falls back to raw output for a non-processes module', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: { ...baseCommand, module: 'apt', action: 'upgrade', output: 'Reading package lists...' },
      },
    })
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('pre.console-output').text()).toContain('Reading package lists')
  })

  it('falls back to raw output while a processes command is still streaming (not valid JSON yet)', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: { ...baseCommand, status: 'running', output: '[{"pid":1,"name":"sys' },
      },
    })
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('pre.console-output').exists()).toBe(true)
  })
})

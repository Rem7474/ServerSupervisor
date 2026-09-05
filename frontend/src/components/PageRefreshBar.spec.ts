import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import PageRefreshBar from './PageRefreshBar.vue'

describe('PageRefreshBar', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('shows the auto-refresh label and pause tooltip when active', () => {
    const wrapper = mount(PageRefreshBar, {
      props: { modelValue: true, intervalSec: 30, lastUpdatedAt: null },
    })
    expect(wrapper.text()).toContain('Auto (30s)')
    expect(wrapper.get('button').attributes('title')).toBe('Cliquer pour mettre en pause')
    expect(wrapper.text()).toContain('dernière MAJ Jamais')
  })

  it('shows the paused label and resume tooltip when inactive', () => {
    const wrapper = mount(PageRefreshBar, {
      props: { modelValue: false, intervalSec: 30, lastUpdatedAt: null },
    })
    expect(wrapper.text()).toContain('Pause')
    expect(wrapper.get('button').attributes('title')).toBe('Cliquer pour reprendre')
  })

  it('formats the last-updated time using the active locale', () => {
    const wrapper = mount(PageRefreshBar, {
      props: { modelValue: true, intervalSec: 30, lastUpdatedAt: new Date('2026-01-01T14:32:05Z') },
    })
    expect(wrapper.text()).toContain('dernière MAJ')
    expect(wrapper.text()).not.toContain('Jamais')
  })

  it('renders the optional label', () => {
    const wrapper = mount(PageRefreshBar, {
      props: { modelValue: true, intervalSec: 30, lastUpdatedAt: null, label: 'Dashboard' },
    })
    expect(wrapper.text()).toContain('Dashboard')
  })

  it('emits update:modelValue when the toggle is clicked', async () => {
    const wrapper = mount(PageRefreshBar, {
      props: { modelValue: true, intervalSec: 30, lastUpdatedAt: null },
    })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(PageRefreshBar, {
      props: { modelValue: true, intervalSec: 15, lastUpdatedAt: null },
    })
    expect(wrapper.text()).toContain('Auto (15s)')
    expect(wrapper.get('button').attributes('title')).toBe('Click to pause')
    expect(wrapper.text()).toContain('last updated Never')
  })
})

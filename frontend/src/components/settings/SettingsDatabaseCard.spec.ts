import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsDatabaseCard from './SettingsDatabaseCard.vue'

const dbStatus = {
  connected: true,
  auditLogCount: 1234,
  metricsCount: 5678,
  hostsCount: 3,
}

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsDatabaseCard', () => {
  it('shows a connected badge and the formatted counts', () => {
    const wrapper = mount(SettingsDatabaseCard, {
      props: { dbStatus, formatNumber: (n: number) => n.toLocaleString('fr-FR') },
    })
    expect(wrapper.text()).toContain('Connectée')
    expect(wrapper.find('.badge').classes()).toContain('bg-success-lt')
    expect(wrapper.text()).toContain('3')
  })

  it('shows a disconnected badge when the database is unreachable', () => {
    const wrapper = mount(SettingsDatabaseCard, {
      props: { dbStatus: { ...dbStatus, connected: false }, formatNumber: (n: number) => String(n) },
    })
    expect(wrapper.text()).toContain('Déconnectée')
    expect(wrapper.find('.badge').classes()).toContain('bg-danger-lt')
  })

  it('translates in English', () => {
    setLocale('en')
    const wrapper = mount(SettingsDatabaseCard, {
      props: { dbStatus, formatNumber: (n: number) => String(n) },
    })
    expect(wrapper.text()).toContain('Database')
    expect(wrapper.text()).toContain('Connected')
    expect(wrapper.text()).toContain('Registered hosts')
  })
})

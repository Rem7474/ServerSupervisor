import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsSystemInfoCard from './SettingsSystemInfoCard.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsSystemInfoCard', () => {
  it('shows the configured values and an "enabled" TLS badge', () => {
    const wrapper = mount(SettingsSystemInfoCard, {
      props: {
        settings: {
          baseUrl: 'https://ss.example.com', dbHost: 'db', dbPort: 5432,
          tlsEnabled: true, latestAgentVersion: '7.6.5',
        },
      },
    })
    expect(wrapper.text()).toContain('https://ss.example.com')
    expect(wrapper.text()).toContain('Activé')
    expect(wrapper.text()).toContain('7.6.5')
  })

  it('falls back to "not configured" for a missing base URL and shows a disabled TLS badge', () => {
    const wrapper = mount(SettingsSystemInfoCard, {
      props: { settings: { tlsEnabled: false } },
    })
    expect(wrapper.text()).toContain('Non configuré')
    expect(wrapper.text()).toContain('Désactivé')
    expect(wrapper.text()).toContain('-')
  })

  it('translates in English', () => {
    setLocale('en')
    const wrapper = mount(SettingsSystemInfoCard, {
      props: { settings: { tlsEnabled: true } },
    })
    expect(wrapper.text()).toContain('System')
    expect(wrapper.text()).toContain('Enabled')
    expect(wrapper.text()).toContain('Recommended agent version')
  })
})

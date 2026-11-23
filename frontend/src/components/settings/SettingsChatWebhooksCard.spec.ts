import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsChatWebhooksCard from './SettingsChatWebhooksCard.vue'

function baseProps(over: Record<string, unknown> = {}) {
  return {
    urls: { slack: '', teams: '', discord: '' },
    configured: { slack: false, teams: false, discord: false },
    testing: { slack: false, teams: false, discord: false },
    ...over,
  }
}

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsChatWebhooksCard', () => {
  it('renders one labelled secret input per provider', () => {
    const wrapper = mount(SettingsChatWebhooksCard, { props: baseProps() })
    const inputs = wrapper.findAll('input')
    expect(inputs).toHaveLength(3)
    for (const input of inputs) {
      expect(input.attributes('type')).toBe('password')
      const label = wrapper.find(`label[for="${input.attributes('id')}"]`)
      expect(label.exists()).toBe(true)
    }
    expect(wrapper.text()).toContain('Slack')
    expect(wrapper.text()).toContain('Microsoft Teams')
    expect(wrapper.text()).toContain('Discord')
  })

  it('only lets a provider be tested once its URL is saved, and emits the provider', async () => {
    const wrapper = mount(SettingsChatWebhooksCard, {
      props: baseProps({ configured: { slack: false, teams: true, discord: false } }),
    })
    const [slack, teams] = wrapper.findAll('button.btn-outline-secondary')
    expect(slack.attributes('disabled')).toBeDefined()
    expect(teams.attributes('disabled')).toBeUndefined()
    expect(teams.attributes('aria-label')).toContain('Microsoft Teams')

    await teams.trigger('click')
    expect(wrapper.emitted('test')?.[0]).toEqual(['teams'])
  })

  it('shows a masked placeholder and the "saved" hint for a configured URL', () => {
    const wrapper = mount(SettingsChatWebhooksCard, {
      props: baseProps({ configured: { slack: true, teams: false, discord: false } }),
    })
    expect(wrapper.findAll('input')[0].attributes('placeholder')).toBe('••••••••')
    expect(wrapper.findAll('input')[1].attributes('placeholder')).toContain('office.com')
    expect(wrapper.text()).toContain('Une URL est enregistrée')
  })

  it('shows the save button for admins only, and the save/test feedback', async () => {
    const viewer = mount(SettingsChatWebhooksCard, { props: baseProps() })
    expect(viewer.find('button.btn-primary').exists()).toBe(false)

    const admin = mount(SettingsChatWebhooksCard, {
      props: baseProps({ authIsAdmin: true, saveMsg: 'ok', saveOk: true, testMsg: 'raté', testOk: false }),
    })
    await admin.find('button.btn-primary').trigger('click')
    expect(admin.emitted('save')).toHaveLength(1)
    expect(admin.find('.text-success').text()).toBe('ok')
    expect(admin.find('.text-danger').text()).toBe('raté')
  })

  it('translates in English', () => {
    setLocale('en')
    const wrapper = mount(SettingsChatWebhooksCard, { props: baseProps() })
    expect(wrapper.text()).toContain('Chat webhooks')
  })
})

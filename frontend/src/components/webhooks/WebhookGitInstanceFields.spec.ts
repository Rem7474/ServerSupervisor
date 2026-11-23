import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import WebhookGitInstanceFields from './WebhookGitInstanceFields.vue'
import type { WebhookFormData } from '../../composables/useWebhookForm'

function form(overrides: Partial<WebhookFormData> = {}): WebhookFormData {
  return { name: '', notify_channels: [], enabled: true, tracker_type: 'git', provider: 'gitlab', ...overrides }
}

describe('WebhookGitInstanceFields', () => {
  beforeEach(() => setLocale('en'))

  it('renders nothing for GitHub', () => {
    const wrapper = mount(WebhookGitInstanceFields, { props: { form: form({ provider: 'github' }) } })
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it.each(['gitlab', 'gitea'])('shows the instance URL and token fields for %s', (provider) => {
    const wrapper = mount(WebhookGitInstanceFields, { props: { form: form({ provider }) } })
    expect(wrapper.find('input[type="url"]').exists()).toBe(true)
    expect(wrapper.find('input[type="password"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Instance URL')
  })

  it('links each label to its input and describes it with its hint', () => {
    const wrapper = mount(WebhookGitInstanceFields, { props: { form: form() } })
    for (const input of wrapper.findAll('input')) {
      const id = input.attributes('id')!
      expect(wrapper.find(`label[for="${id}"]`).exists()).toBe(true)
      expect(wrapper.find(`#${CSS.escape(input.attributes('aria-describedby')!)}`).exists()).toBe(true)
    }
  })

  it('emits a whole-form replacement when the URL and token are typed', async () => {
    const base = form()
    const wrapper = mount(WebhookGitInstanceFields, { props: { form: base } })
    await wrapper.find('input[type="url"]').setValue('https://git.lan')
    await wrapper.find('input[type="password"]').setValue('tok')
    const emitted = wrapper.emitted('update:form')!
    expect(emitted[0][0]).toMatchObject({ git_base_url: 'https://git.lan' })
    expect(emitted[1][0]).toMatchObject({ git_token: 'tok' })
    expect(base.git_base_url).toBeUndefined()
  })

  it('offers to remove a stored token and disables the token input once ticked', async () => {
    const wrapper = mount(WebhookGitInstanceFields, { props: { form: form({ has_git_token: true }) } })
    expect(wrapper.find('input[type="password"]').attributes('placeholder')).toContain('token stored')
    await wrapper.find('input[type="checkbox"]').setValue(true)
    expect(wrapper.emitted('update:form')![0][0]).toMatchObject({ clear_git_token: true })
    await wrapper.setProps({ form: form({ has_git_token: true, clear_git_token: true }) })
    expect(wrapper.find('input[type="password"]').attributes('disabled')).toBeDefined()
  })

  it('hides the removal checkbox when no token is stored', () => {
    const wrapper = mount(WebhookGitInstanceFields, { props: { form: form() } })
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(false)
  })
})

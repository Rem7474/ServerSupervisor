import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsNotificationsCard from './SettingsNotificationsCard.vue'

function baseForm() {
  return { ntfyUrl: '', githubToken: '' }
}

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsNotificationsCard', () => {
  it('toggles the GitHub token visibility label', async () => {
    const wrapper = mount(SettingsNotificationsCard, { props: { form: baseForm() } })
    const toggle = wrapper.find('button')
    expect(toggle.text()).toBe('Afficher')
    await toggle.trigger('click')
    expect(wrapper.emitted('update:show-github-token')?.[0]).toEqual([true])
  })

  it('only shows the save button for an admin', () => {
    const viewer = mount(SettingsNotificationsCard, { props: { form: baseForm(), authIsAdmin: false } })
    expect(viewer.text()).not.toContain('Enregistrer')

    const admin = mount(SettingsNotificationsCard, { props: { form: baseForm(), authIsAdmin: true } })
    expect(admin.text()).toContain('Enregistrer')
  })

  it('disables the ntfy test button until a URL is set, and shows the short in-progress label', async () => {
    const wrapper = mount(SettingsNotificationsCard, { props: { form: baseForm() } })
    const buttons = wrapper.findAll('button')
    const testButton = buttons[buttons.length - 1]
    expect(testButton.attributes('disabled')).toBeDefined()

    await wrapper.setProps({ form: { ...baseForm(), ntfyUrl: 'https://ntfy.sh/x' }, testingNtfy: true })
    expect(testButton.text()).toBe('Test…')
  })

  it('translates in English', () => {
    setLocale('en')
    const wrapper = mount(SettingsNotificationsCard, { props: { form: baseForm() } })
    expect(wrapper.text()).toContain('For tracking GitHub releases')
  })
})

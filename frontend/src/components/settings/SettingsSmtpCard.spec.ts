import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsSmtpCard from './SettingsSmtpCard.vue'

function baseForm() {
  return { smtpHost: '', smtpPort: 587, smtpUser: '', smtpPass: '', smtpFrom: '', smtpTo: '', smtpTls: true }
}

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsSmtpCard', () => {
  it('toggles the password field type and label when the show/hide button is clicked', async () => {
    const wrapper = mount(SettingsSmtpCard, { props: { form: baseForm() } })
    const toggle = wrapper.findAll('button').find((b) => b.text() === 'Afficher' || b.text() === 'Masquer')!
    expect(toggle.text()).toBe('Afficher')

    await toggle.trigger('click')
    expect(wrapper.emitted('update:show-smtp-pass')?.[0]).toEqual([true])

    await wrapper.setProps({ showSmtpPass: true })
    expect(toggle.text()).toBe('Masquer')
  })

  it('only shows the save button for an admin, disabled while saving', () => {
    const viewer = mount(SettingsSmtpCard, { props: { form: baseForm(), authIsAdmin: false } })
    expect(viewer.text()).not.toContain('Enregistrer SMTP')

    const admin = mount(SettingsSmtpCard, { props: { form: baseForm(), authIsAdmin: true, savingSmtp: true } })
    const saveButton = admin.findAll('button').find((b) => b.text().includes('Enregistr'))!
    expect(saveButton.text()).toBe('Enregistrement...')
  })

  it('disables the test button until a host is set', async () => {
    const wrapper = mount(SettingsSmtpCard, { props: { form: baseForm() } })
    const testButton = wrapper.findAll('button').find((b) => b.text().includes('Tester') || b.text().includes('Test'))!
    expect(testButton.attributes('disabled')).toBeDefined()

    await wrapper.setProps({ form: { ...baseForm(), smtpHost: 'mail.example.com' } })
    expect(testButton.attributes('disabled')).toBeUndefined()
  })

  it('disables the test button and shows the in-progress label while testing', async () => {
    const wrapper = mount(SettingsSmtpCard, {
      props: { form: { ...baseForm(), smtpHost: 'mail.example.com' }, testingSmtp: true },
    })
    const testButton = wrapper.findAll('button').find((b) => b.text().includes('Tester') || b.text().includes('Test'))!
    expect(testButton.attributes('disabled')).toBeDefined()
    expect(testButton.text()).toBe('Test en cours...')
  })

  it('shows the save and test result messages independently', () => {
    const wrapper = mount(SettingsSmtpCard, {
      props: {
        form: baseForm(),
        smtpSaveMsg: 'Paramètres enregistrés', smtpSaveOk: true,
        smtpTestMessage: 'Échec de connexion', smtpTestSuccess: false,
      },
    })
    expect(wrapper.text()).toContain('Paramètres enregistrés')
    expect(wrapper.text()).toContain('Échec de connexion')
  })

  it('translates in English', () => {
    setLocale('en')
    const wrapper = mount(SettingsSmtpCard, { props: { form: baseForm(), authIsAdmin: true } })
    expect(wrapper.text()).toContain('SMTP host')
    expect(wrapper.text()).toContain('Sender (From)')
    expect(wrapper.text()).toContain('Save SMTP')
  })
})

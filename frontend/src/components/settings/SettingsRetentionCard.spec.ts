import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import SettingsRetentionCard from './SettingsRetentionCard.vue'

function baseForm() {
  return { metricsRetentionDays: 30, auditRetentionDays: 90, auditRetentionDaysByCategory: {} as Record<string, number> }
}

beforeEach(() => {
  setLocale('fr')
})

describe('SettingsRetentionCard', () => {
  it('does not show the per-category section when there are no categories', () => {
    const wrapper = mount(SettingsRetentionCard, { props: { form: baseForm() } })
    expect(wrapper.text()).not.toContain('par catégorie')
  })

  it('shows one input per audit category, prefilled from the override map', () => {
    const wrapper = mount(SettingsRetentionCard, {
      props: {
        form: { ...baseForm(), auditRetentionDaysByCategory: { auth: 180 } },
        auditCategories: [{ key: 'auth', label: 'Authentification' }, { key: 'command', label: 'Commandes' }],
      },
    })
    const inputs = wrapper.findAll('.input-group input')
    expect((inputs[0].element as HTMLInputElement).value).toBe('180')
    expect((inputs[1].element as HTMLInputElement).value).toBe('')
  })

  it('sets a category override when a valid positive number is entered', async () => {
    const form = { ...baseForm(), auditRetentionDaysByCategory: {} as Record<string, number> }
    const wrapper = mount(SettingsRetentionCard, {
      props: { form, auditCategories: [{ key: 'auth', label: 'Authentification' }] },
    })
    await wrapper.find('.input-group input').setValue('365')
    expect(form.auditRetentionDaysByCategory.auth).toBe(365)
  })

  it('removes the override when the input is cleared', async () => {
    const form = { ...baseForm(), auditRetentionDaysByCategory: { auth: 180 } }
    const wrapper = mount(SettingsRetentionCard, {
      props: { form, auditCategories: [{ key: 'auth', label: 'Authentification' }] },
    })
    await wrapper.find('.input-group input').setValue('')
    expect(form.auditRetentionDaysByCategory.auth).toBeUndefined()
  })

  it('only shows the save button and emits save for an admin', async () => {
    const viewer = mount(SettingsRetentionCard, { props: { form: baseForm(), authIsAdmin: false } })
    expect(viewer.find('button').exists()).toBe(false)

    const admin = mount(SettingsRetentionCard, { props: { form: baseForm(), authIsAdmin: true } })
    expect(admin.find('button').text()).toBe('Enregistrer')
    await admin.find('button').trigger('click')
    expect(admin.emitted('save')).toHaveLength(1)
  })

  it('shows the saving label and the save-result message', () => {
    const wrapper = mount(SettingsRetentionCard, {
      props: {
        form: baseForm(), authIsAdmin: true, savingRetention: true,
        retentionSaveMsg: 'Erreur serveur', retentionSaveOk: false,
      },
    })
    expect(wrapper.find('button').text()).toBe('Enregistrement...')
    expect(wrapper.text()).toContain('Erreur serveur')
  })

  it('translates in English', () => {
    setLocale('en')
    const wrapper = mount(SettingsRetentionCard, {
      props: { form: baseForm(), authIsAdmin: true, auditCategories: [{ key: 'auth', label: 'Auth' }] },
    })
    expect(wrapper.text()).toContain('Data retention')
    expect(wrapper.text()).toContain('by category')
    expect(wrapper.find('button').text()).toBe('Save')
  })
})

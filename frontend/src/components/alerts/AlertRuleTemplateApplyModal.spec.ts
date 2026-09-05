import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AlertRuleTemplateApplyModal from './AlertRuleTemplateApplyModal.vue'
import type { AlertRuleTemplate, Host } from '../../types/generated'

const template: AlertRuleTemplate = { id: 1, name: 'High CPU' } as AlertRuleTemplate
const hosts: Host[] = [
  { id: 'h1', name: 'web-01', tags: ['prod'] } as Host,
  { id: 'h2', name: 'db-01', tags: ['prod', 'db'] } as Host,
]

beforeEach(() => {
  setLocale('fr')
})

describe('AlertRuleTemplateApplyModal', () => {
  it('shows nothing when not visible', () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: false } })
    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it('renders the template name in the title and lists hosts with tags', () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: true, template, hosts } })
    expect(wrapper.text()).toContain('Appliquer « High CPU »')
    expect(wrapper.text()).toContain('web-01')
    expect(wrapper.text()).toContain('prod, db')
  })

  it('filters hosts by name or tag', async () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: true, template, hosts } })
    await wrapper.find('input[type="text"]').setValue('db')
    const labels = wrapper.findAll('label.d-block')
    expect(labels).toHaveLength(1)
    expect(labels[0].text()).toContain('db-01')
  })

  it('shows the no-match message when the filter matches nothing', async () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: true, template, hosts } })
    await wrapper.find('input[type="text"]').setValue('nonexistent')
    expect(wrapper.text()).toContain('Aucun hôte ne correspond.')
  })

  it('pluralizes the selected-hosts count and disables Apply until at least one is checked', async () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: true, template, hosts } })
    expect(wrapper.text()).toContain('0 hôtes sélectionnés')
    const applyButton = wrapper.findAll('button').find((b) => b.text() === 'Appliquer')
    expect(applyButton?.attributes('disabled')).toBeDefined()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0].setValue(true)
    await checkboxes[1].setValue(true)
    expect(wrapper.text()).toContain('2 hôtes sélectionnés')
    expect(applyButton?.attributes('disabled')).toBeUndefined()
  })

  it('emits apply with the selected host ids and enabled flag', async () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: true, template, hosts } })
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(true)
    // The lone remaining checkbox after the host list is "enable immediately".
    await wrapper.find('label.form-check:not(.d-block) input[type="checkbox"]').setValue(true)

    const applyButton = wrapper.findAll('button').find((b) => b.text() === 'Appliquer')
    await applyButton?.trigger('click')

    expect(wrapper.emitted('apply')?.[0]).toEqual([['h1'], true])
  })

  it('shows the created-rules result and switches Cancel to Close', () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, {
      props: { visible: true, template, hosts, result: { created_rule_ids: [1, 2], errors: {} } },
    })
    expect(wrapper.text()).toContain('2 règle(s) créée(s).')
    expect(wrapper.text()).toContain('Fermer')
    expect(wrapper.text()).not.toContain('Annuler')
  })

  it('shows per-host failures with the resolved host name', () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, {
      props: { visible: true, template, hosts, result: { created_rule_ids: [], errors: { h1: 'metric already covered' } } },
    })
    expect(wrapper.text()).toContain('Échecs :')
    expect(wrapper.text()).toContain('web-01 : metric already covered')
  })

  it('emits close from the header and footer buttons', async () => {
    const wrapper = mount(AlertRuleTemplateApplyModal, { props: { visible: true, template, hosts } })
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)

    await wrapper.find('button.btn-outline-secondary').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(2)
  })
})

import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AlertRuleTemplateList from './AlertRuleTemplateList.vue'
import type { AlertRuleTemplate } from '../../types/generated'

function template(overrides: Partial<AlertRuleTemplate>): AlertRuleTemplate {
  return {
    id: 1,
    name: 'High CPU',
    metric: 'cpu',
    operator: '>',
    threshold_warn: 70,
    threshold_crit: 90,
    ...overrides,
  } as AlertRuleTemplate
}

beforeEach(() => {
  setLocale('fr')
})

describe('AlertRuleTemplateList', () => {
  it('shows the empty state when there are no templates', () => {
    const wrapper = mount(AlertRuleTemplateList, { props: { templates: [], fetched: true } })
    expect(wrapper.text()).toContain('Aucun modèle de règle')
  })

  it('shows a loading skeleton while loading before the first fetch', () => {
    const wrapper = mount(AlertRuleTemplateList, { props: { loading: true, fetched: false } })
    expect(wrapper.findComponent({ name: 'LoadingSkeleton' }).exists()).toBe(true)
  })

  it('renders a row per template with the threshold summary', () => {
    const wrapper = mount(AlertRuleTemplateList, {
      props: { templates: [template({})], fetched: true },
    })
    expect(wrapper.text()).toContain('High CPU')
    expect(wrapper.text()).toContain('avert. 70 · crit. 90')
  })

  it('hides admin actions for a non-admin', () => {
    const wrapper = mount(AlertRuleTemplateList, {
      props: { templates: [template({})], fetched: true, isAdmin: false },
    })
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('emits add/apply/edit/delete for an admin', async () => {
    const wrapper = mount(AlertRuleTemplateList, {
      props: { templates: [template({ id: 42 })], fetched: true, isAdmin: true },
    })

    await wrapper.find('button.btn-primary').trigger('click')
    expect(wrapper.emitted('add')).toBeTruthy()

    await wrapper.find('button[title="Appliquer à des hôtes"]').trigger('click')
    expect(wrapper.emitted('apply')?.[0]?.[0]).toMatchObject({ id: 42 })

    await wrapper.find('button[aria-label="Modifier le modèle"]').trigger('click')
    expect(wrapper.emitted('edit')?.[0]?.[0]).toMatchObject({ id: 42 })

    await wrapper.find('button[aria-label="Supprimer le modèle"]').trigger('click')
    expect(wrapper.emitted('delete')?.[0]?.[0]).toMatchObject({ id: 42 })
  })
})

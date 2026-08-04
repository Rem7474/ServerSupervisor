import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import RestrictedSelect from './RestrictedSelect.vue'

describe('RestrictedSelect', () => {
  it('falls back to a free-text input when options is empty', () => {
    const wrapper = mount(RestrictedSelect, {
      props: { modelValue: '', options: [], emptyLabel: 'Choisir...', placeholder: 'ex: files' },
    })
    expect(wrapper.find('select').exists()).toBe(false)
    const input = wrapper.find('input')
    expect(input.exists()).toBe(true)
    expect(input.attributes('placeholder')).toBe('ex: files')
  })

  it('renders a flat <select> for a plain string[] option list', () => {
    const wrapper = mount(RestrictedSelect, {
      props: { modelValue: '', options: ['files', 'db'], emptyLabel: 'Profil (défaut)' },
    })
    const select = wrapper.find('select')
    expect(select.exists()).toBe(true)
    expect(select.findAll('optgroup').length).toBe(0)
    const options = select.findAll('option')
    expect(options.map((o) => o.text())).toEqual(['Profil (défaut)', 'files', 'db'])
  })

  it('renders a flat <select> for {value,label} option objects', () => {
    const wrapper = mount(RestrictedSelect, {
      props: {
        modelValue: '',
        options: [{ value: 'task-1', label: 'Deploy (task-1)' }],
        emptyLabel: 'Sélectionner une tâche...',
      },
    })
    const option = wrapper.find('select').findAll('option')[1]
    expect(option.text()).toBe('Deploy (task-1)')
    expect(option.attributes('value')).toBe('task-1')
  })

  it('renders grouped <optgroup>s for OptionGroup[] input', () => {
    const wrapper = mount(RestrictedSelect, {
      props: {
        modelValue: '',
        options: [
          { label: 'Profils', options: ['files', 'db'] },
          { label: 'Groupes', options: ['full-backup'] },
        ],
        emptyLabel: 'Profil (défaut)',
      },
    })
    const groups = wrapper.find('select').findAll('optgroup')
    expect(groups.map((g) => g.attributes('label'))).toEqual(['Profils', 'Groupes'])
    expect(groups[1].findAll('option').map((o) => o.text())).toEqual(['full-backup'])
  })

  it('falls back to the input when every group is empty', () => {
    const wrapper = mount(RestrictedSelect, {
      props: {
        modelValue: '',
        options: [{ label: 'Profils', options: [] }],
        emptyLabel: 'Profil (défaut)',
        placeholder: 'ex: files',
      },
    })
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(true)
  })

  it('passes through extra attributes (disabled, title, style) to the rendered element', () => {
    const wrapper = mount(RestrictedSelect, {
      props: { modelValue: '', options: ['files'], emptyLabel: 'Profil (défaut)' },
      attrs: { disabled: true, title: 'a hint' },
    })
    const select = wrapper.find('select')
    expect(select.attributes('disabled')).toBeDefined()
    expect(select.attributes('title')).toBe('a hint')
  })

  it('updates the v-model on select', async () => {
    const wrapper = mount(RestrictedSelect, {
      props: { modelValue: '', options: ['files', 'db'], emptyLabel: 'Profil (défaut)' },
    })
    await wrapper.find('select').setValue('db')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['db'])
  })
})

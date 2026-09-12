import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import TimeRangePicker from './TimeRangePicker.vue'
import type { TimeRangeModel, TimeRangePreset } from '../../types/timeRange'

beforeEach(() => {
  setLocale('fr')
})

const presets: TimeRangePreset[] = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7 jours' },
]

describe('TimeRangePicker', () => {
  it('selecting a preset emits the preset model and clears any validation error', async () => {
    const wrapper = mount(TimeRangePicker, {
      props: { presets, modelValue: { mode: 'preset', period: '24h', from: null, to: null } },
    })
    const buttons = wrapper.findAll('.btn-group button')
    await buttons[1].trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({
      mode: 'preset', period: '7d', from: null, to: null,
    })
    expect(wrapper.emitted('change')).toHaveLength(1)
  })

  it('shows the custom fields when the model is already in custom mode, prefilled from the model', () => {
    const wrapper = mount(TimeRangePicker, {
      props: {
        presets,
        modelValue: { mode: 'custom', period: '24h', from: '2026-01-15T10:00:00.000Z', to: '2026-01-15T12:00:00.000Z' },
      },
    })
    expect(wrapper.find('input[type="datetime-local"]').exists()).toBe(true)
    const inputs = wrapper.findAll('input[type="datetime-local"]')
    expect((inputs[0].element as HTMLInputElement).value).not.toBe('')
    expect((inputs[1].element as HTMLInputElement).value).not.toBe('')
  })

  it('rejects applying with an empty date', async () => {
    const wrapper = mount(TimeRangePicker, {
      props: { presets, modelValue: { mode: 'preset', period: '24h', from: null, to: null } },
    })
    await wrapper.findAll('.btn-group button')[2].trigger('click') // "Personnalisé…" toggle
    await wrapper.find('.btn-primary.btn-sm').trigger('click') // Apply with both fields empty

    expect(wrapper.text()).toContain('Renseignez les deux dates.')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('rejects an end date before the start date', async () => {
    const wrapper = mount(TimeRangePicker, {
      props: { presets, modelValue: { mode: 'preset', period: '24h', from: null, to: null } },
    })
    await wrapper.findAll('.btn-group button')[2].trigger('click')
    const inputs = wrapper.findAll('input[type="datetime-local"]')
    await inputs[0].setValue('2026-01-15T12:00')
    await inputs[1].setValue('2026-01-15T10:00')
    await wrapper.find('.btn-primary.btn-sm').trigger('click')

    expect(wrapper.text()).toContain('La date de fin doit être après le début.')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('applies a valid custom range', async () => {
    const wrapper = mount(TimeRangePicker, {
      props: { presets, modelValue: { mode: 'preset', period: '24h', from: null, to: null } },
    })
    await wrapper.findAll('.btn-group button')[2].trigger('click')
    const inputs = wrapper.findAll('input[type="datetime-local"]')
    await inputs[0].setValue('2026-01-15T10:00')
    await inputs[1].setValue('2026-01-15T12:00')
    await wrapper.find('.btn-primary.btn-sm').trigger('click')

    const emitted = wrapper.emitted('update:modelValue')?.[0]?.[0] as TimeRangeModel
    expect(emitted.mode).toBe('custom')
    expect(emitted.from).toBeTruthy()
    expect(emitted.to).toBeTruthy()
    expect(wrapper.emitted('change')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('La date de fin doit être après le début.')
  })
})

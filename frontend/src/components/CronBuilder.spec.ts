import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setLocale } from '../i18n'
import CronBuilder from './CronBuilder.vue'

describe('CronBuilder', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders the visual builder by default with translated labels', () => {
    const wrapper = mount(CronBuilder, { props: { modelValue: '0 3 * * *' } })
    expect(wrapper.text()).toContain('Expression cron')
    expect(wrapper.text()).toContain('Fréquence')
    expect(wrapper.text()).toContain('Heure')
    expect(wrapper.text()).toContain('Minute')
    expect(wrapper.text()).toContain('Quotidien')
    expect(wrapper.text()).not.toContain('Jours')
  })

  it('shows the days-of-week toggles with translated abbreviations for a weekly frequency', async () => {
    const wrapper = mount(CronBuilder, { props: { modelValue: '0 3 * * 1' } })
    await nextTick()
    expect(wrapper.text()).toContain('Jours')
    expect(wrapper.text()).toContain('Lun')
    expect(wrapper.text()).toContain('Dim')
  })

  it('shows the day-of-month selector with translated label for a monthly frequency', async () => {
    const wrapper = mount(CronBuilder, { props: { modelValue: '0 3 15 * *' } })
    await nextTick()
    expect(wrapper.text()).toContain('Jour du mois')
  })

  it('shows the format hint with the TZ variable in expert mode', async () => {
    const wrapper = mount(CronBuilder, { props: { modelValue: '@daily' } })
    await nextTick()
    expect(wrapper.text()).toContain('Format : minute heure jour-du-mois mois jour-de-la-semaine')
    expect(wrapper.text()).toContain('TZ')
    expect(wrapper.text()).toContain('UTC par défaut')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(CronBuilder, { props: { modelValue: '0 3 * * *' } })
    expect(wrapper.text()).toContain('Cron expression')
    expect(wrapper.text()).toContain('Frequency')
    expect(wrapper.text()).toContain('Daily')
  })
})

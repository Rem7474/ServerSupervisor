import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setLocale } from '../i18n'
import RelativeTime from './RelativeTime.vue'

describe('RelativeTime', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the translated "never" fallback for an empty date', async () => {
    const wrapper = mount(RelativeTime, { props: { date: '' } })
    await nextTick()
    expect(wrapper.text()).toBe('Jamais')
  })

  it('shows the translated "never" fallback for the Go zero-value date', async () => {
    const wrapper = mount(RelativeTime, { props: { date: '0001-01-01T00:00:00Z' } })
    await nextTick()
    expect(wrapper.text()).toBe('Jamais')
  })

  it('shows a relative label for a real date', async () => {
    vi.useFakeTimers().setSystemTime(new Date('2026-01-01T00:05:00Z'))
    const wrapper = mount(RelativeTime, { props: { date: '2026-01-01T00:00:00Z' } })
    await nextTick()
    expect(wrapper.text()).not.toBe('')
    expect(wrapper.text()).not.toBe('Jamais')
  })

  it('translates the "never" fallback to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(RelativeTime, { props: { date: '' } })
    await nextTick()
    expect(wrapper.text()).toBe('Never')
  })
})

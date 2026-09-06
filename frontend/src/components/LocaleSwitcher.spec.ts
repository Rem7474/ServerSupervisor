import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale, SUPPORTED_LOCALES } from '../i18n'
import LocaleSwitcher from './LocaleSwitcher.vue'

describe('LocaleSwitcher', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  afterEach(() => {
    setLocale('fr')
  })

  it('renders one button per supported locale', () => {
    const w = mount(LocaleSwitcher)
    expect(w.findAll('button')).toHaveLength(SUPPORTED_LOCALES.length)
  })

  it('marks only the active locale as pressed', async () => {
    const w = mount(LocaleSwitcher)
    const [fr, en] = w.findAll('button')

    expect(fr.attributes('aria-pressed')).toBe('true')
    expect(en.attributes('aria-pressed')).toBe('false')
    expect(fr.classes()).toContain('btn-primary')

    await en.trigger('click')

    expect(en.attributes('aria-pressed')).toBe('true')
    expect(fr.attributes('aria-pressed')).toBe('false')
    expect(en.classes()).toContain('btn-primary')
  })

  it('switches the app locale and persists the choice', async () => {
    const w = mount(LocaleSwitcher)

    await w.findAll('button')[1].trigger('click')

    expect(localStorage.getItem('locale')).toBe('en')
    expect(document.documentElement.lang).toBe('en')
  })

  it('labels each button with its own endonym, whatever the active locale', async () => {
    // A picker names each language in *that* language, so someone stranded on
    // a UI they can't read still recognises the one they want.
    const w = mount(LocaleSwitcher)
    const labels = () => w.findAll('button').map((b) => b.attributes('aria-label'))

    expect(labels()).toEqual(['Français', 'English'])

    setLocale('en')
    await w.vm.$nextTick()

    expect(labels()).toEqual(['Français', 'English'])
  })

  it('gives every button an accessible name and a title', () => {
    const w = mount(LocaleSwitcher)
    for (const button of w.findAll('button')) {
      expect(button.attributes('aria-label')).toBeTruthy()
      expect(button.attributes('title')).toBe(button.attributes('aria-label'))
    }
  })
})

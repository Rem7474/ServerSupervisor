import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useCommandPalette } from '../composables/useCommandPalette'
import CommandPalette from './CommandPalette.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

function mountPalette() {
  const auth = useAuthStore()
  auth.setAuth({ role: 'admin', username: 'u' } as never, 'admin')
  return mount(CommandPalette)
}

describe('CommandPalette', () => {
  beforeEach(() => {
    setLocale('fr')
    setActivePinia(createPinia())
    // `query` is a module-level singleton shared with useCommandPalette.spec.ts
    // and across mounts in this file — reset it so one test's typed value
    // can't leak into the next mount's initial render.
    let api!: ReturnType<typeof useCommandPalette>
    mount({ setup: () => { api = useCommandPalette(); return () => null } })
    api.query.value = ''
  })

  it('renders the translated search input and shortcut chrome', () => {
    const wrapper = mountPalette()
    expect(wrapper.get('[aria-label]').attributes('aria-label')).toBe('Palette de commande')
    expect(wrapper.get('.command-palette-input').attributes('placeholder')).toBe(
      'Rechercher une page, un hôte, un conteneur, un domaine, une IP…'
    )
    expect(wrapper.get('.command-palette-esc').text()).toBe('Échap')
  })

  it('groups nav results under a translated group label for an empty query', () => {
    const wrapper = mountPalette()
    expect(wrapper.text()).toContain('Navigation')
  })

  it('shows the no-results hint for a query matching nothing', async () => {
    const wrapper = mountPalette()
    await wrapper.get('.command-palette-input').setValue('zzz-no-match-zzz')
    expect(wrapper.text()).toContain('Aucun résultat.')
  })

  it('translates chrome to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mountPalette()
    expect(wrapper.get('.command-palette-input').attributes('placeholder')).toBe(
      'Search a page, a host, a container, a domain, an IP…'
    )
    expect(wrapper.get('.command-palette-esc').text()).toBe('Esc')
    expect(wrapper.text()).toContain('Navigation')
  })
})

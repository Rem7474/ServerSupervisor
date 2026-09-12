import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import TrackerVersionHistoryCard from './TrackerVersionHistoryCard.vue'
import type { ReleaseVersionHistoryItem } from '../../types/tracker'

function entry(version: string, overrides: Partial<ReleaseVersionHistoryItem> = {}): ReleaseVersionHistoryItem {
  return { version, published_at: '2026-01-01T00:00:00Z', name: '', release_url: '', ...overrides }
}

beforeEach(() => {
  setLocale('fr')
})

describe('TrackerVersionHistoryCard', () => {
  it('renders the translated title, subtitle and column headers', () => {
    const wrapper = mount(TrackerVersionHistoryCard, { props: { history: [entry('v1.0')], loading: false } })
    expect(wrapper.text()).toContain('Historique des versions')
    expect(wrapper.text()).toContain('Publication / détection')
    for (const label of ['Version', 'Détails', 'Date de publication']) {
      expect(wrapper.text()).toContain(label)
    }
  })

  it('shows the translated empty state when there is no version', () => {
    const wrapper = mount(TrackerVersionHistoryCard, { props: { history: [], loading: false } })
    expect(wrapper.text()).toContain('Aucune version disponible.')
  })

  it('shows a translated "show more" button with the hidden count, then toggles to "show less"', async () => {
    const history = Array.from({ length: 8 }, (_, i) => entry(`v${i}`))
    const wrapper = mount(TrackerVersionHistoryCard, { props: { history, loading: false } })

    const button = wrapper.find('button')
    expect(button.text()).toBe('Afficher plus (3)')

    await button.trigger('click')
    expect(button.text()).toBe('Afficher moins')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(TrackerVersionHistoryCard, { props: { history: [], loading: false } })
    expect(wrapper.text()).toContain('Version history')
    expect(wrapper.text()).toContain('No version available.')
  })
})

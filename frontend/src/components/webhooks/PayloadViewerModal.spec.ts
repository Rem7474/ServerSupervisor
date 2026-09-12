import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import PayloadViewerModal from './PayloadViewerModal.vue'

beforeEach(() => {
  setLocale('fr')
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
})

describe('PayloadViewerModal', () => {
  it('renders the translated title, formats valid JSON, and shows a translated copy tooltip', () => {
    const wrapper = mount(PayloadViewerModal, { props: { payload: '{"a":1}' } })
    expect(wrapper.text()).toContain('Payload reçu')
    expect(wrapper.find('pre').text()).toContain('"a": 1')
    expect(wrapper.find('button[title="Copier"]').exists()).toBe(true)
  })

  it('shows the translated "truncated/non-JSON" warning and raw payload for invalid JSON', () => {
    const wrapper = mount(PayloadViewerModal, { props: { payload: 'not-json' } })
    expect(wrapper.text()).toContain('Payload tronqué ou non-JSON — affichage brut.')
    expect(wrapper.find('pre').text()).toBe('not-json')
  })

  it('shows the translated "copied" tooltip after clicking copy, and the close button', async () => {
    const wrapper = mount(PayloadViewerModal, { props: { payload: '{}' } })
    await wrapper.find('button.btn-ghost-secondary').trigger('click')
    expect(wrapper.find('button[title="Copié !"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Fermer')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(PayloadViewerModal, { props: { payload: '{}' } })
    expect(wrapper.text()).toContain('Payload received')
    expect(wrapper.text()).toContain('Close')
  })
})

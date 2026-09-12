import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const { regenerateWebhookSecret } = vi.hoisted(() => ({
  regenerateWebhookSecret: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { regenerateWebhookSecret },
}))

import WebhookUrlCard from './WebhookUrlCard.vue'

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
})

describe('WebhookUrlCard', () => {
  it('renders the translated field labels and copy tooltips', () => {
    const wrapper = mount(WebhookUrlCard, { props: { webhookId: 'wh-1', secret: 'abc123' } })
    expect(wrapper.text()).toContain('URL du Webhook')
    expect(wrapper.text()).toContain('Secret HMAC')
    expect(wrapper.findAll('button[title="Copier"]').length).toBeGreaterThan(0)
  })

  it('shows the translated provider instructions for GitHub (generic branch)', () => {
    const wrapper = mount(WebhookUrlCard, { props: { webhookId: 'wh-1', provider: 'github' } })
    expect(wrapper.text()).toContain('Configuration GitHub:')
    expect(wrapper.text()).toContain('Content type:')
    expect(wrapper.text()).toContain('coller le secret ci-dessus')
    expect(wrapper.text()).toContain('SSL: enabled (recommandé)')
  })

  it('shows the translated GitLab-specific header instructions', () => {
    const wrapper = mount(WebhookUrlCard, { props: { webhookId: 'wh-1', provider: 'gitlab' } })
    expect(wrapper.text()).toContain('Configuration GitLab:')
    expect(wrapper.text()).toContain('Header:')
  })

  it('shows the translated regenerate confirm dialog and success message', async () => {
    regenerateWebhookSecret.mockResolvedValue({ data: { secret: 'new-secret' } })
    const wrapper = mount(WebhookUrlCard, { props: { webhookId: 'wh-1', initialSecret: false } })
    const dialog = useConfirmDialog()

    const regenPromise = wrapper.find('button.btn-outline-warning').trigger('click')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.title.value).toBe('Régénérer le secret ?')
    dialog.onConfirm()
    await regenPromise
    await flushPromises()

    expect(wrapper.text()).toContain('Secret régénéré — copiez-le maintenant.')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(WebhookUrlCard, { props: { webhookId: 'wh-1', provider: 'github' } })
    expect(wrapper.text()).toContain('Webhook URL')
    expect(wrapper.text()).toContain('HMAC Secret')
    expect(wrapper.text()).toContain('SSL: enabled (recommended)')
  })
})

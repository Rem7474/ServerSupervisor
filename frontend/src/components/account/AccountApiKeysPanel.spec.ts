import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import type { APIKey } from '../../types/generated'

const { listMyApiKeys, createApiKey, revokeApiKey } = vi.hoisted(() => ({
  listMyApiKeys: vi.fn(),
  createApiKey: vi.fn(),
  revokeApiKey: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { listMyApiKeys, createApiKey, revokeApiKey },
}))

const writeText = vi.fn(async () => {})
Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

import AccountApiKeysPanel from './AccountApiKeysPanel.vue'

const future = new Date(Date.now() + 30 * 86400_000).toISOString()
const past = new Date(Date.now() - 86400_000).toISOString()

const keys: APIKey[] = [
  { id: 1, name: 'grafana', prefix: 'ssk_aaaaaaaa', scope: 'read', username: 'alice', created_at: past, expires_at: future, last_used_at: past },
  { id: 2, name: 'ci', prefix: 'ssk_bbbbbbbb', scope: 'write', username: 'alice', created_at: past },
  { id: 3, name: 'old', prefix: 'ssk_cccccccc', scope: 'read', username: 'alice', created_at: past, expires_at: past },
  { id: 4, name: 'gone', prefix: 'ssk_dddddddd', scope: 'read', username: 'alice', created_at: past, revoked_at: past },
]

function mountPanel() {
  return mount(AccountApiKeysPanel, { attachTo: document.body })
}

describe('AccountApiKeysPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    listMyApiKeys.mockResolvedValue({ data: keys })
  })

  it('lists the keys with their scope, expiry and status', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(4)
    expect(rows[0].text()).toContain('grafana')
    expect(rows[0].text()).toContain('Lecture seule')
    expect(rows[0].text()).toContain('Active')
    expect(rows[1].text()).toContain('Lecture et écriture')
    expect(rows[1].text()).toContain('Jamais')
    expect(rows[1].text()).toContain('Sans expiration')
    expect(rows[2].text()).toContain('Expirée')
    expect(rows[3].text()).toContain('Révoquée')
    // Revoked keys offer no revoke button; the others name the key in their label.
    expect(rows[3].find('button').exists()).toBe(false)
    expect(rows[0].find('button').attributes('aria-label')).toContain('grafana')
    wrapper.unmount()
  })

  it('shows the empty state when there is no key', async () => {
    listMyApiKeys.mockResolvedValue({ data: [] })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('Aucune clé API')
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })

  it('creates a key and shows its secret once, behind a persistent dialog', async () => {
    createApiKey.mockResolvedValue({
      data: { key: { id: 5, name: 'backup', prefix: 'ssk_eeeeeeee', scope: 'write', username: 'alice', created_at: past }, secret: 'ssk_eeeeeeee_secret' },
    })
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.find('.card-header button').trigger('click')
    const dialog = wrapper.find('[role="dialog"]')
    expect(dialog.attributes('aria-modal')).toBe('true')
    const title = dialog.find('.modal-title')
    expect(dialog.attributes('aria-labelledby')).toBe(title.attributes('id'))

    const submit = dialog.find('button[type="submit"]')
    expect(submit.attributes('disabled')).toBeDefined()
    await dialog.find('input[type="text"]').setValue('backup')
    const selects = dialog.findAll('select')
    await selects[0].setValue('write')
    expect(dialog.text()).toContain('acquitter une alerte')
    await selects[1].setValue('0')
    await dialog.find('form').trigger('submit')
    await flushPromises()

    expect(createApiKey).toHaveBeenCalledWith({ name: 'backup', scope: 'write', expires_in_days: 0 })
    const secretInput = wrapper.find<HTMLInputElement>('[role="dialog"] input[readonly]')
    expect(secretInput.element.value).toBe('ssk_eeeeeeee_secret')
    // No close cross and no backdrop dismissal on the secret step.
    expect(wrapper.find('[role="dialog"] .btn-close').exists()).toBe(false)
    await wrapper.find('[role="dialog"]').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)

    const copy = wrapper.findAll('[role="dialog"] .input-group button')[0]
    await copy.trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith('ssk_eeeeeeee_secret')
    expect(copy.text()).toContain('Copiée')

    await wrapper.find('[role="dialog"] .modal-footer button').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('ssk_eeeeeeee_secret')
    expect(wrapper.findAll('tbody tr')).toHaveLength(5)
    wrapper.unmount()
  })

  it('closes the form step on cancel and on a backdrop click', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.find('.card-header button').trigger('click')
    await wrapper.find('[role="dialog"] .modal-footer .btn-outline-secondary').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)

    await wrapper.find('.card-header button').trigger('click')
    await wrapper.find('[role="dialog"]').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows a creation error inside the dialog', async () => {
    createApiKey.mockRejectedValue(Object.assign(new Error('x'), { response: { data: { error: 'nom invalide' } } }))
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.find('.card-header button').trigger('click')
    await wrapper.find('[role="dialog"] input[type="text"]').setValue('x')
    await wrapper.find('[role="dialog"] form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[role="dialog"] .alert-danger').text()).toBe('nom invalide')
    wrapper.unmount()
  })
})

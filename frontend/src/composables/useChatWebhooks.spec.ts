import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setLocale } from '../i18n'

const { updateSettings, testWebhook } = vi.hoisted(() => ({
  updateSettings: vi.fn(),
  testWebhook: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { updateSettings, testWebhook },
  getApiErrorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

import { useChatWebhooks } from './useChatWebhooks'

beforeEach(() => {
  setLocale('en')
  updateSettings.mockReset().mockResolvedValue({})
  testWebhook.mockReset().mockResolvedValue({})
})

describe('useChatWebhooks', () => {
  it('sends only the non-empty, non-masked URLs and refreshes afterwards', async () => {
    const onSaved = vi.fn().mockResolvedValue(undefined)
    const w = useChatWebhooks(onSaved)
    w.urls.slack = '  https://hooks.slack.com/services/a  '
    w.urls.teams = '••••••••'

    await w.save()

    expect(updateSettings).toHaveBeenCalledWith({ slack_webhook_url: 'https://hooks.slack.com/services/a' })
    expect(onSaved).toHaveBeenCalledTimes(1)
    expect(w.saveOk.value).toBe(true)
    expect(w.saveMsg.value).toBe('Chat webhooks saved')
    expect(w.urls.slack).toBe('')
  })

  it('keeps the typed URL and reports the server error when saving fails', async () => {
    updateSettings.mockRejectedValue(new Error('invalid slack webhook URL'))
    const onSaved = vi.fn()
    const w = useChatWebhooks(onSaved)
    w.urls.discord = 'http://discord.com/x'

    await w.save()

    expect(w.saveOk.value).toBe(false)
    expect(w.saveMsg.value).toContain('invalid slack webhook URL')
    expect(w.urls.discord).toBe('http://discord.com/x')
    expect(onSaved).not.toHaveBeenCalled()
    expect(w.saving.value).toBe(false)
  })

  it('tests one provider and reports success or failure', async () => {
    const w = useChatWebhooks(vi.fn())

    await w.test('teams')
    expect(testWebhook).toHaveBeenCalledWith('teams')
    expect(w.testOk.value).toBe(true)
    expect(w.testMsg.value).toBe('Test message sent to Microsoft Teams')

    testWebhook.mockRejectedValue(new Error('status 404'))
    await w.test('discord')
    expect(w.testOk.value).toBe(false)
    expect(w.testMsg.value).toContain('status 404')
    expect(w.testing.discord).toBe(false)
  })
})

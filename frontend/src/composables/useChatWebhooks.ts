import { reactive, ref } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { i18n } from '../i18n'
import type { SettingsUpdateRequest } from '../types/settings'

export type ChatProvider = 'slack' | 'teams' | 'discord'

export const CHAT_PROVIDERS: ChatProvider[] = ['slack', 'teams', 'discord']

const MASKED_SECRET = '••••••••'

const FIELD: Record<ChatProvider, keyof SettingsUpdateRequest> = {
  slack: 'slack_webhook_url',
  teams: 'teams_webhook_url',
  discord: 'discord_webhook_url',
}

/**
 * State of the Slack / Teams / Discord incoming-webhook card. The URLs are
 * secrets: the server never returns them, so the inputs start empty and only
 * a non-empty value is sent on save (an empty one keeps the stored URL).
 * `onSaved` refreshes the "configured" flags afterwards.
 */
export function useChatWebhooks(onSaved: () => Promise<void>) {
  const urls = reactive<Record<ChatProvider, string>>({ slack: '', teams: '', discord: '' })
  const saving = ref(false)
  const saveMsg = ref('')
  const saveOk = ref(false)
  const testing = reactive<Record<ChatProvider, boolean>>({ slack: false, teams: false, discord: false })
  const testMsg = ref('')
  const testOk = ref(false)

  async function save(): Promise<void> {
    saving.value = true
    saveMsg.value = ''
    try {
      const payload: Partial<SettingsUpdateRequest> = {}
      for (const p of CHAT_PROVIDERS) {
        const v = urls[p].trim()
        if (v && v !== MASKED_SECRET) Object.assign(payload, { [FIELD[p]]: v })
      }
      await apiClient.updateSettings(payload)
      saveOk.value = true
      saveMsg.value = i18n.global.t('settings.chatWebhooks.savedMsg')
      for (const p of CHAT_PROVIDERS) urls[p] = ''
      await onSaved()
    } catch (e) {
      saveOk.value = false
      saveMsg.value = i18n.global.t('settings.saveErrorPrefix', { message: getApiErrorMessage(e) })
    } finally {
      saving.value = false
      setTimeout(() => { saveMsg.value = '' }, 5000)
    }
  }

  async function test(provider: ChatProvider): Promise<void> {
    testing[provider] = true
    testMsg.value = ''
    try {
      await apiClient.testWebhook(provider)
      testOk.value = true
      testMsg.value = i18n.global.t('settings.chatWebhooks.testSentMsg', { provider: i18n.global.t(`settings.chatWebhooks.${provider}`) })
    } catch (e) {
      testOk.value = false
      testMsg.value = i18n.global.t('settings.saveErrorPrefix', { message: getApiErrorMessage(e) })
    } finally {
      testing[provider] = false
      setTimeout(() => { testMsg.value = '' }, 5000)
    }
  }

  return { urls, saving, saveMsg, saveOk, testing, testMsg, testOk, save, test }
}

<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('settings.chatWebhooks.title') }}
      </h3>
    </div>
    <div class="card-body">
      <p class="text-secondary small">
        {{ t('settings.chatWebhooks.intro') }}
      </p>
      <div
        v-for="(provider, i) in providers"
        :key="provider"
        class="mb-3"
      >
        <label
          :for="`${uid}-${i}`"
          class="form-label"
        >{{ t(`settings.chatWebhooks.${provider}`) }}</label>
        <div class="input-group">
          <input
            :id="`${uid}-${i}`"
            v-model="urls[provider]"
            type="password"
            class="form-control"
            :placeholder="configured[provider] ? '••••••••' : placeholders[provider]"
            autocomplete="new-password"
            :aria-describedby="`${uid}-hint-${i}`"
          >
          <button
            type="button"
            class="btn btn-outline-secondary"
            :disabled="!configured[provider] || testing[provider]"
            :aria-label="t('settings.chatWebhooks.testAria', { provider: t(`settings.chatWebhooks.${provider}`) })"
            @click="$emit('test', provider)"
          >
            {{ testing[provider] ? t('settings.testingShort') : t('settings.chatWebhooks.test') }}
          </button>
        </div>
        <div
          :id="`${uid}-hint-${i}`"
          class="form-hint"
        >
          {{ configured[provider] ? t('settings.chatWebhooks.configuredHint') : t('settings.chatWebhooks.emptyHint') }}
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        type="button"
        class="btn btn-primary"
        :disabled="saving"
        @click="$emit('save')"
      >
        {{ saving ? t('common.saving') : t('common.save') }}
      </button>
      <span
        v-if="saveMsg"
        :class="['ms-auto small', saveOk ? 'text-success' : 'text-danger']"
        role="status"
      >
        {{ saveMsg }}
      </span>
      <span
        v-if="testMsg"
        :class="['ms-auto small', testOk ? 'text-success' : 'text-danger']"
        role="status"
      >
        {{ testMsg }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { CHAT_PROVIDERS, type ChatProvider } from '../../composables/useChatWebhooks'

const uid = useId()
const { t } = useI18n()
const providers = CHAT_PROVIDERS

const placeholders: Record<ChatProvider, string> = {
  slack: 'https://hooks.slack.com/services/…',
  teams: 'https://…webhook.office.com/…',
  discord: 'https://discord.com/api/webhooks/…',
}

withDefaults(defineProps<{
  urls: Record<ChatProvider, string>
  configured: Record<ChatProvider, boolean>
  testing: Record<ChatProvider, boolean>
  authIsAdmin?: boolean
  saving?: boolean
  saveMsg?: string
  saveOk?: boolean
  testMsg?: string
  testOk?: boolean
}>(), {
  authIsAdmin: false,
  saving: false,
  saveMsg: '',
  saveOk: false,
  testMsg: '',
  testOk: false,
})

defineEmits<{
  (e: 'save'): void
  (e: 'test', provider: ChatProvider): void
}>()
</script>

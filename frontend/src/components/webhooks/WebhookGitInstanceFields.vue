<template>
  <template v-if="selfHostable">
    <div :class="colClass">
      <label
        :for="`${uid}-base-url`"
        class="form-label"
      >{{ t('webhooks.gitBaseUrlLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
      <input
        :id="`${uid}-base-url`"
        v-model="baseUrlModel"
        type="url"
        class="form-control"
        :placeholder="t('webhooks.gitBaseUrlPlaceholder')"
        autocomplete="off"
        :aria-describedby="`${uid}-base-url-hint`"
      >
      <div
        :id="`${uid}-base-url-hint`"
        class="form-hint"
      >
        {{ t('webhooks.gitBaseUrlHint') }}
      </div>
    </div>
    <div :class="colClass">
      <label
        :for="`${uid}-token`"
        class="form-label"
      >{{ t('webhooks.gitTokenLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
      <input
        :id="`${uid}-token`"
        v-model="tokenModel"
        type="password"
        class="form-control"
        :placeholder="hasStoredToken ? t('webhooks.gitTokenStoredPlaceholder') : ''"
        autocomplete="new-password"
        :disabled="form.clear_git_token"
        :aria-describedby="`${uid}-token-hint`"
      >
      <div
        :id="`${uid}-token-hint`"
        class="form-hint"
      >
        {{ t('webhooks.gitTokenHint') }}
      </div>
      <label
        v-if="hasStoredToken"
        class="form-check mt-1"
      >
        <input
          v-model="clearTokenModel"
          type="checkbox"
          class="form-check-input"
        >
        <span class="form-check-label">{{ t('webhooks.gitTokenClearLabel') }}</span>
      </label>
    </div>
  </template>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WebhookFormData } from '../../composables/useWebhookForm'
import { isSelfHostable } from '../../utils/gitRepoLinks'

const { t } = useI18n()
const uid = useId()

const props = withDefaults(defineProps<{
  form: WebhookFormData
  colClass?: string
}>(), { colClass: 'col-md-6' })

const emit = defineEmits<{
  (e: 'update:form', value: WebhookFormData): void
}>()

const selfHostable = computed(() => isSelfHostable(props.form.provider))
const hasStoredToken = computed(() => !!props.form.has_git_token)

function field<K extends keyof WebhookFormData>(key: K) {
  return computed<WebhookFormData[K]>({
    get: () => props.form[key],
    set: (value) => emit('update:form', { ...props.form, [key]: value }),
  })
}

const baseUrlModel = field('git_base_url')
const tokenModel = field('git_token')
const clearTokenModel = field('clear_git_token')
</script>

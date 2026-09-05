<template>
  <div class="card">
    <div class="card-body">
      <!-- Webhook URL -->
      <div class="mb-3">
        <label class="form-label fw-medium">{{ t('webhooks.webhookUrlLabel') }}</label>
        <div class="input-group">
          <input
            type="text"
            class="form-control font-monospace small"
            readonly
            :value="webhookUrl"
          >
          <button
            class="btn btn-outline-secondary"
            type="button"
            :title="urlCopied ? t('webhooks.copiedTooltip') : t('webhooks.copyTooltip')"
            @click="copyUrl"
          >
            <IconCopy
              v-if="!urlCopied"
              :size="16"
            />
            <IconCheck
              v-else
              :size="16"
              class="text-success"
            />
          </button>
        </div>
      </div>

      <!-- Secret -->
      <div class="mb-3">
        <label class="form-label fw-medium">{{ t('webhooks.hmacSecretLabel') }}</label>
        <div class="input-group">
          <input
            :type="showSecret ? 'text' : 'password'"
            class="form-control font-monospace small"
            readonly
            :value="currentSecret || '••••••••••••••••'"
          >
          <button
            v-if="currentSecret"
            class="btn btn-outline-secondary"
            type="button"
            @click="showSecret = !showSecret"
          >
            <IconEye
              v-if="!showSecret"
              :size="16"
            />
            <IconEyeOff
              v-else
              :size="16"
            />
          </button>
          <button
            v-if="currentSecret"
            class="btn btn-outline-secondary"
            type="button"
            :title="secretCopied ? t('webhooks.copiedTooltip') : t('webhooks.copyTooltip')"
            @click="copySecret"
          >
            <IconCopy
              v-if="!secretCopied"
              :size="16"
            />
            <IconCheck
              v-else
              :size="16"
              class="text-success"
            />
          </button>
          <button
            v-if="!initialSecret && webhookId"
            class="btn btn-outline-warning"
            type="button"
            :disabled="regenerating"
            @click="doRegenerate"
          >
            <IconRefresh :size="16" />
            {{ regenerating ? '...' : t('webhooks.regenerateButton') }}
          </button>
        </div>
        <div
          v-if="regenMsg"
          class="form-hint"
          :class="regenOk ? 'text-success' : 'text-danger'"
        >
          {{ regenMsg }}
        </div>
      </div>

      <!-- Provider instructions -->
      <div
        v-if="provider"
        class="small text-muted border-top pt-3"
      >
        <strong>{{ t('webhooks.providerConfigPrefix', { provider: providerLabel }) }}</strong>
        <div
          class="mt-1 font-monospace provider-setup-box"
        >
          <template v-if="provider === 'gitlab'">
            {{ t('webhooks.gitlabHeaderLabel') }} <strong>X-Gitlab-Token</strong> = &lt;secret&gt;
          </template>
          <template v-else>
            {{ t('webhooks.contentTypeLabel') }} <strong>application/json</strong><br>
            Secret: {{ t('webhooks.secretPasteInstructions') }}<br>
            {{ t('webhooks.sslEnabledRecommended') }}
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconCheck, IconCopy, IconEye, IconEyeOff, IconRefresh } from '@tabler/icons-vue'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import api from '../../api'
import { getApiErrorMessage } from '../../api/client'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  webhookId?: string
  secret?: string
  provider?: string
  initialSecret?: boolean
}>(), {
  webhookId: '',
  secret: '',
  provider: '',
  initialSecret: false,
})

const emit = defineEmits<{
  (e: 'secret-regenerated', secret: string): void
}>()
const dialog = useConfirmDialog()

const showSecret = ref<boolean>(!!props.secret)
const currentSecret = ref<string>(props.secret)
const urlCopied = ref(false)
const secretCopied = ref(false)
const regenerating = ref(false)
const regenMsg = ref('')
const regenOk = ref(false)

watch(() => props.secret, (v) => {
  currentSecret.value = v
  if (v) showSecret.value = true
})

const webhookUrl = computed(() => {
  const base = window.location.origin
  return `${base}/api/v1/webhooks/git/${props.webhookId}/receive`
})

const providerLabel = computed(() => {
  const map: Record<string, string> = { github: 'GitHub', gitlab: 'GitLab', gitea: 'Gitea', forgejo: 'Forgejo', custom: 'Custom' }
  return map[props.provider] || props.provider
})

function copyUrl(): void {
  navigator.clipboard.writeText(webhookUrl.value)
  urlCopied.value = true
  setTimeout(() => urlCopied.value = false, 2000)
}

function copySecret(): void {
  if (!currentSecret.value) return
  navigator.clipboard.writeText(currentSecret.value)
  secretCopied.value = true
  setTimeout(() => secretCopied.value = false, 2000)
}

async function doRegenerate(): Promise<void> {
  const ok = await dialog.confirm({
    title: t('webhooks.regenerateSecretConfirmTitle'),
    message: t('webhooks.regenerateSecretConfirmMessage'),
    variant: 'warning',
  })
  if (!ok) return
  regenerating.value = true
  regenMsg.value = ''
  try {
    const res = await api.regenerateWebhookSecret(props.webhookId)
    currentSecret.value = res.data.secret
    showSecret.value = true
    regenMsg.value = t('webhooks.secretRegeneratedMessage')
    regenOk.value = true
    emit('secret-regenerated', res.data.secret)
  } catch (e: unknown) {
    regenMsg.value = getApiErrorMessage(e, t('common.error'))
    regenOk.value = false
  } finally {
    regenerating.value = false
  }
}
</script>

<style scoped>
.provider-setup-box {
  background: var(--tblr-bg-surface-secondary);
  border-radius: 4px;
  padding: 8px;
  font-size: 0.8rem;
  border: 1px solid var(--tblr-border-color);
}
</style>

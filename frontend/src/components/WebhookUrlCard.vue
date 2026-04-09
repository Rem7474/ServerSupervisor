<template>
  <div class="card">
    <div class="card-body">
      <!-- Webhook URL -->
      <div class="mb-3">
        <label class="form-label fw-medium">URL du Webhook</label>
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
            :title="urlCopied ? 'Copié !' : 'Copier'"
            @click="copyUrl"
          >
            <svg
              v-if="!urlCopied"
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
            >
              <rect
                x="9"
                y="9"
                width="13"
                height="13"
                rx="2"
              /><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
              class="text-success"
            >
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Secret -->
      <div class="mb-3">
        <label class="form-label fw-medium">Secret HMAC</label>
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
            <svg
              v-if="!showSecret"
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
            >
              <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle
                cx="12"
                cy="12"
                r="3"
              />
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
            >
              <path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24" />
              <line
                x1="1"
                y1="1"
                x2="23"
                y2="23"
              />
            </svg>
          </button>
          <button
            v-if="currentSecret"
            class="btn btn-outline-secondary"
            type="button"
            :title="secretCopied ? 'Copié !' : 'Copier'"
            @click="copySecret"
          >
            <svg
              v-if="!secretCopied"
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
            >
              <rect
                x="9"
                y="9"
                width="13"
                height="13"
                rx="2"
              /><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
              class="text-success"
            >
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </button>
          <button
            v-if="!initialSecret && webhookId"
            class="btn btn-outline-warning"
            type="button"
            :disabled="regenerating"
            @click="doRegenerate"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              viewBox="0 0 24 24"
            >
              <polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10" />
            </svg>
            {{ regenerating ? '...' : 'Régénérer' }}
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
        <strong>Configuration {{ providerLabel }}:</strong>
        <div
          class="mt-1 font-monospace"
          style="background:#f1f3f4;border-radius:4px;padding:8px;font-size:0.8rem;"
        >
          <template v-if="provider === 'gitlab'">
            Header: <strong>X-Gitlab-Token</strong> = &lt;secret&gt;
          </template>
          <template v-else>
            Content type: <strong>application/json</strong><br>
            Secret: coller le secret ci-dessus<br>
            SSL: enabled (recommandé)
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import api from '../api'

const props = defineProps({
  webhookId: { type: String, default: '' },
  secret: { type: String, default: '' },    // Initial secret (shown after creation)
  provider: { type: String, default: '' },
  initialSecret: { type: Boolean, default: false }, // true = first-time display, no regenerate needed
})

const emit = defineEmits(['secret-regenerated'])
const dialog = useConfirmDialog()

const showSecret = ref(!!props.secret)
const currentSecret = ref(props.secret)
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
  const map = { github: 'GitHub', gitlab: 'GitLab', gitea: 'Gitea', forgejo: 'Forgejo', custom: 'Custom' }
  return map[props.provider] || props.provider
})

function copyUrl() {
  navigator.clipboard.writeText(webhookUrl.value)
  urlCopied.value = true
  setTimeout(() => urlCopied.value = false, 2000)
}

function copySecret() {
  if (!currentSecret.value) return
  navigator.clipboard.writeText(currentSecret.value)
  secretCopied.value = true
  setTimeout(() => secretCopied.value = false, 2000)
}

async function doRegenerate() {
  const ok = await dialog.confirm({
    title: 'Régénérer le secret ?',
    message: 'L\'ancien secret sera invalidé immédiatement. Vous devrez mettre à jour la configuration dans votre provider Git.',
    variant: 'warning',
  })
  if (!ok) return
  regenerating.value = true
  regenMsg.value = ''
  try {
    const res = await api.regenerateWebhookSecret(props.webhookId)
    currentSecret.value = res.data.secret
    showSecret.value = true
    regenMsg.value = 'Secret régénéré — copiez-le maintenant.'
    regenOk.value = true
    emit('secret-regenerated', res.data.secret)
  } catch (e) {
    regenMsg.value = e.response?.data?.error || 'Erreur'
    regenOk.value = false
  } finally {
    regenerating.value = false
  }
}
</script>

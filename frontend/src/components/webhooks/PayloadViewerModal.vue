<template>
  <div
    v-if="payload"
    ref="modalRef"
    class="modal modal-blur fade show d-block"
    tabindex="-1"
    @click.self="$emit('close')"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            {{ t('webhooks.payloadReceivedTitle') }}
          </h5>
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary me-1"
            :title="copied ? t('webhooks.copiedTooltip') : t('webhooks.copyTooltip')"
            @click="copy"
          >
            <IconCheck
              v-if="copied"
              :size="16"
              class="icon text-success"
            />
            <IconCopy
              v-else
              :size="16"
              class="icon"
            />
          </button>
          <button
            type="button"
            class="btn-close"
            @click="$emit('close')"
          />
        </div>
        <div class="modal-body">
          <div
            v-if="!isValidJson"
            class="alert alert-warning small mb-2"
          >
            {{ t('webhooks.payloadTruncatedWarning') }}
          </div>
          <pre
            class="bg-dark text-light rounded p-3 small mb-0"
            style="max-height: 60vh; overflow: auto;"
          >{{ formattedPayload }}</pre>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-outline-secondary"
            @click="$emit('close')"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
  <div
    v-if="payload"
    class="modal-backdrop fade show"
  />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconCheck, IconCopy } from '@tabler/icons-vue'
import { useModalChrome } from '../../composables/useModalChrome'

const { t } = useI18n()

const props = defineProps<{
  payload: string | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => !!props.payload, { onClose: () => emit('close') })

const copied = ref(false)

const isValidJson = computed(() => {
  if (!props.payload) return false
  try {
    JSON.parse(props.payload)
    return true
  } catch {
    return false
  }
})

const formattedPayload = computed(() => {
  if (!props.payload) return ''
  if (!isValidJson.value) return props.payload
  try {
    return JSON.stringify(JSON.parse(props.payload), null, 2)
  } catch {
    return props.payload
  }
})

async function copy(): Promise<void> {
  if (!props.payload) return
  await navigator.clipboard.writeText(formattedPayload.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 1500)
}
</script>

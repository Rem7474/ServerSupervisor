<template>
  <slot
    v-if="!caught"
    name="default"
  />
  <div
    v-else
    class="alert alert-danger d-flex align-items-start gap-3 my-3"
    role="alert"
  >
    <IconAlertTriangle
      :size="24"
      class="icon icon-md flex-shrink-0 mt-1"
    />
    <div class="flex-fill">
      <div class="fw-semibold mb-1">
        {{ title }}
      </div>
      <div
        v-if="message"
        class="small text-muted mb-2"
      >
        {{ message }}
      </div>
      <button
        type="button"
        class="btn btn-sm btn-outline-danger"
        @click="reset"
      >
        {{ t('common.retry') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onErrorCaptured } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconAlertTriangle } from '@tabler/icons-vue'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  title?: string
}>(), {
  title: '',
})
const title = computed(() => props.title || t('common.unexpectedError'))

const caught = ref(false)
const message = ref('')

onErrorCaptured((err: unknown) => {
  caught.value = true
  message.value = err instanceof Error ? err.message : String(err)
  return false
})

function reset(): void {
  caught.value = false
  message.value = ''
}
</script>

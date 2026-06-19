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
        Réessayer
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'
import { IconAlertTriangle } from '@tabler/icons-vue'

withDefaults(defineProps<{
  title?: string
}>(), {
  title: 'Une erreur inattendue s\'est produite',
})

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

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
    <svg
      xmlns="http://www.w3.org/2000/svg"
      class="icon icon-md flex-shrink-0 mt-1"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      stroke-width="2"
      stroke="currentColor"
      fill="none"
    >
      <path
        stroke="none"
        d="M0 0h24v24H0z"
        fill="none"
      />
      <path d="M12 9v4" />
      <path d="M10.363 3.591l-8.106 13.534a1.914 1.914 0 0 0 1.636 2.871h16.214a1.914 1.914 0 0 0 1.636 -2.87l-8.106 -13.536a1.914 1.914 0 0 0 -3.274 0z" />
      <path d="M12 16h.01" />
    </svg>
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

const props = withDefaults(defineProps<{
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

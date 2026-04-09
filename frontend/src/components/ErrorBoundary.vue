<template>
  <div>
    <div
      v-if="hasError"
      class="alert alert-danger d-flex justify-content-between align-items-start gap-3"
      role="alert"
    >
      <div>
        <div class="fw-semibold">
          Une erreur inattendue est survenue dans l'interface.
        </div>
        <div class="small opacity-75">
          {{ errorMessage }}
        </div>
      </div>
      <button
        type="button"
        class="btn btn-sm btn-danger"
        @click="resetBoundary"
      >
        Réessayer
      </button>
    </div>
    <slot v-else />
  </div>
</template>

<script setup>
import { onErrorCaptured, ref } from 'vue'

const hasError = ref(false)
const errorMessage = ref('')

function resetBoundary() {
  hasError.value = false
  errorMessage.value = ''
}

onErrorCaptured((error) => {
  hasError.value = true
  errorMessage.value =
    error?.message ||
    "Une erreur s'est produite pendant le rendu de cette page"
  return false
})
</script>
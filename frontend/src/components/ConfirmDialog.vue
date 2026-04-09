<template>
  <div
    v-if="dialog.isOpen.value"
    class="modal modal-blur fade show"
    style="display: block;"
    tabindex="-1"
  >
    <div class="modal-dialog modal-sm modal-dialog-centered">
      <div class="modal-content">
        <div :class="['modal-status', dialog.variant.value === 'danger' ? 'bg-danger' : 'bg-warning']" />
        <div class="modal-body text-center py-4">
          <svg
            v-if="dialog.variant.value === 'danger'"
            class="icon mb-2 text-danger icon-lg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            stroke-width="2"
            stroke="currentColor"
            fill="none"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              stroke="none"
              d="M0 0h24v24H0z"
              fill="none"
            />
            <path d="M10.24 3.957l-8.422 14.06a1.989 1.989 0 0 0 1.7 2.983h16.845a1.989 1.989 0 0 0 1.7 -2.983l-8.423 -14.06a1.989 1.989 0 0 0 -3.4 0z" />
            <path d="M12 9v4" />
            <path d="M12 17h.01" />
          </svg>
          <svg
            v-else
            class="icon mb-2 text-warning icon-lg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            stroke-width="2"
            stroke="currentColor"
            fill="none"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              stroke="none"
              d="M0 0h24v24H0z"
              fill="none"
            />
            <path d="M10.24 3.957l-8.422 14.06a1.989 1.989 0 0 0 1.7 2.983h16.845a1.989 1.989 0 0 0 1.7 -2.983l-8.423 -14.06a1.989 1.989 0 0 0 -3.4 0z" />
            <path d="M12 9v4" />
            <path d="M12 17h.01" />
          </svg>
          <h3>{{ dialog.title.value }}</h3>
          <div
            class="text-secondary"
            style="white-space: pre-line;"
          >
            {{ dialog.message.value }}
          </div>

          <div
            v-if="dialog.requiredText.value"
            class="mt-3 text-start"
          >
            <div class="text-secondary small mb-1">
              Tapez <strong class="text-body">{{ dialog.requiredText.value }}</strong> pour confirmer :
            </div>
            <input
              ref="inputRef"
              v-model="typedText"
              type="text"
              class="form-control"
              :placeholder="dialog.requiredText.value"
              autocomplete="off"
              @keyup.enter="tryConfirm"
            >
          </div>
        </div>
        <div class="modal-footer">
          <div class="w-100 d-flex gap-2">
            <button
              class="btn link-secondary w-100"
              @click="handleCancel"
            >
              Annuler
            </button>
            <button
              :disabled="!!dialog.requiredText.value && typedText !== dialog.requiredText.value"
              :class="['btn', 'w-100', dialog.variant.value === 'danger' ? 'btn-danger' : 'btn-warning']"
              @click="tryConfirm"
            >
              Confirmer
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div
    v-if="dialog.isOpen.value"
    class="modal-backdrop fade show"
  />
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const dialog = useConfirmDialog()
const typedText = ref('')
const inputRef = ref(null)

watch(dialog.isOpen, (val) => {
  if (val) {
    typedText.value = ''
    if (dialog.requiredText.value) {
      nextTick(() => inputRef.value?.focus())
    }
  }
})

function tryConfirm() {
  if (dialog.requiredText.value && typedText.value !== dialog.requiredText.value) return
  typedText.value = ''
  dialog.onConfirm()
}

function handleCancel() {
  typedText.value = ''
  dialog.onCancel()
}
</script>

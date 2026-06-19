<template>
  <div
    v-if="dialog.isOpen.value"
    ref="modalRef"
    class="modal modal-blur fade show"
    style="display: block;"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
  >
    <div class="modal-dialog modal-sm modal-dialog-centered">
      <div class="modal-content">
        <div :class="['modal-status', dialog.destructive.value || dialog.variant.value === 'danger' ? 'bg-danger' : 'bg-warning']" />
        <div class="modal-body text-center py-4">
          <IconAlertTriangle
            :size="24"
            class="icon mb-2 text-danger icon-lg"
          />
          <IconAlertTriangle
            :size="24"
            class="icon mb-2 text-warning icon-lg"
          />
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
              type="button"
              class="btn link-secondary w-100"
              @click="handleCancel"
            >
              {{ dialog.cancelLabel.value }}
            </button>
            <button
              type="button"
              :disabled="!!dialog.requiredText.value && typedText !== dialog.requiredText.value"
              :class="['btn', 'w-100', dialog.destructive.value || dialog.variant.value === 'danger' ? 'btn-danger' : 'btn-warning']"
              @click="tryConfirm"
            >
              {{ dialog.okLabel.value }}
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

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { IconAlertTriangle } from '@tabler/icons-vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useModalFocusTrap } from '../composables/useModalFocusTrap'

const dialog = useConfirmDialog()
const typedText = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const modalRef = ref<HTMLElement | null>(null)

useModalFocusTrap(modalRef)

watch(dialog.isOpen, (val) => {
  if (val) {
    typedText.value = ''
    if (dialog.requiredText.value) {
      nextTick(() => inputRef.value?.focus())
    }
  }
})

function tryConfirm(): void {
  if (dialog.requiredText.value && typedText.value !== dialog.requiredText.value) return
  typedText.value = ''
  dialog.onConfirm()
}

function handleCancel(): void {
  typedText.value = ''
  dialog.onCancel()
}
</script>

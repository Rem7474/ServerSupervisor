<template>
  <div v-if="dialog.isOpen.value" class="modal modal-blur fade show" style="display: block;" tabindex="-1">
    <div class="modal-dialog modal-sm modal-dialog-centered">
      <div class="modal-content">
        <div :class="['modal-status', dialog.variant.value === 'danger' ? 'bg-danger' : 'bg-warning']"></div>
        <div class="modal-body text-center py-4">
          <svg v-if="dialog.variant.value === 'danger'" 
               class="icon mb-2 text-danger icon-lg" width="24" height="24" viewBox="0 0 24 24" 
               stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
            <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
            <path d="M10.24 3.957l-8.422 14.06a1.989 1.989 0 0 0 1.7 2.983h16.845a1.989 1.989 0 0 0 1.7 -2.983l-8.423 -14.06a1.989 1.989 0 0 0 -3.4 0z" />
            <path d="M12 9v4" />
            <path d="M12 17h.01" />
          </svg>
          <svg v-else 
               class="icon mb-2 text-warning icon-lg" width="24" height="24" viewBox="0 0 24 24" 
               stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
            <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
            <path d="M10.24 3.957l-8.422 14.06a1.989 1.989 0 0 0 1.7 2.983h16.845a1.989 1.989 0 0 0 1.7 -2.983l-8.423 -14.06a1.989 1.989 0 0 0 -3.4 0z" />
            <path d="M12 9v4" />
            <path d="M12 17h.01" />
          </svg>
          <h3>{{ dialog.title.value }}</h3>
          <div class="text-secondary" style="white-space: pre-line;">{{ dialog.message.value }}</div>
        </div>
        <div class="modal-footer">
          <div class="w-100 d-flex gap-2">
            <button @click="dialog.onCancel()" class="btn link-secondary w-100">
              Annuler
            </button>
            <button @click="dialog.onConfirm()" 
                    :class="['btn', 'w-100', dialog.variant.value === 'danger' ? 'btn-danger' : 'btn-warning']">
              Confirmer
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
  <div v-if="dialog.isOpen.value" class="modal-backdrop fade show"></div>
</template>

<script setup>
import { useConfirmDialog } from '../composables/useConfirmDialog'

const dialog = useConfirmDialog()
</script>

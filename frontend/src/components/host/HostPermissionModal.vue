<template>
  <div
    v-if="open"
    ref="permModalRef"
    class="modal modal-blur fade show d-block modal-permissions-overlay"
    tabindex="-1"
    role="dialog"
    aria-modal="true"
    @click.self="open = false"
  >
    <div
      class="modal-dialog modal-dialog-centered modal-permissions-dialog"
    >
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            {{ t('host.addPermissionTitle') }}
          </h5>
          <button
            type="button"
            :aria-label="t('common.close')"
            class="btn-close"
            @click="open = false"
          />
        </div>
        <div class="modal-body">
          <div class="mb-3">
            <label
              :for="`${uid}-2`"
              class="form-label"
            >{{ t('host.userColumn') }}</label>
            <select
              :id="`${uid}-2`"
              v-model="username"
              class="form-select"
            >
              <option value="">
                {{ t('host.chooseEllipsis') }}
              </option>
              <option
                v-for="u in availableUsers"
                :key="u.username"
                :value="u.username"
              >
                {{ u.username }}
              </option>
            </select>
          </div>
          <div class="mb-3">
            <label
              :for="`${uid}-3`"
              class="form-label"
            >{{ t('host.levelColumn') }}</label>
            <select
              :id="`${uid}-3`"
              v-model="level"
              class="form-select"
            >
              <option value="viewer">
                {{ t('host.viewerReadOnlyOption') }}
              </option>
              <option value="operator">
                {{ t('host.operatorOption') }}
              </option>
            </select>
          </div>
          <div
            v-if="permError"
            class="alert alert-danger py-2"
          >
            {{ permError }}
          </div>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary"
            @click="open = false"
          >
            {{ t('host.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!username || permSaving"
            @click="$emit('save')"
          >
            <span
              v-if="permSaving"
              class="spinner-border spinner-border-sm me-1"
            />
            {{ t('host.saveLabel') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalChrome } from '../../composables/useModalChrome'
import type { HostRecord } from '../../types/hostDetail'

defineProps<{
  availableUsers: HostRecord[]
  permSaving: boolean
  permError: string
}>()

const open = defineModel<boolean>('open', { required: true })
const username = defineModel<string>('username', { required: true })
const level = defineModel<string>('level', { required: true })

defineEmits<{ save: [] }>()

const { t } = useI18n()
const uid = useId()

const permModalRef = ref<HTMLElement | null>(null)
useModalChrome(permModalRef, () => open.value, { onClose: () => { open.value = false } })
</script>

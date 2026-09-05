<template>
  <div v-if="visible">
    <div
      ref="modalRef"
      class="modal modal-blur fade show"
      style="display: block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ t('alerts.applyTemplateTitle', { name: template?.name }) }}
            </h5>
            <button
              type="button"
              class="btn-close"
              :aria-label="t('common.close')"
              @click="$emit('close')"
            />
          </div>
          <div class="modal-body">
            <div
              v-if="!result"
            >
              <div class="mb-3">
                <label class="form-label">{{ t('alerts.hostsLabel') }}</label>
                <div class="input-icon mb-2">
                  <span class="input-icon-addon">
                    <IconSearch :size="16" />
                  </span>
                  <input
                    v-model="search"
                    type="text"
                    class="form-control"
                    :placeholder="t('alerts.filterHostsPlaceholder')"
                  >
                </div>
                <div
                  class="border rounded p-2"
                  style="max-height: 260px; overflow-y: auto;"
                >
                  <label
                    v-for="h in filteredHosts"
                    :key="h.id"
                    class="form-check d-block"
                  >
                    <input
                      v-model="selectedHostIds"
                      class="form-check-input"
                      type="checkbox"
                      :value="h.id"
                    >
                    <span class="form-check-label">
                      {{ h.name }}
                      <span
                        v-if="h.tags && h.tags.length"
                        class="text-muted small"
                      >— {{ h.tags.join(', ') }}</span>
                    </span>
                  </label>
                  <p
                    v-if="filteredHosts.length === 0"
                    class="text-muted small mb-0"
                  >
                    {{ t('alerts.noHostsMatch') }}
                  </p>
                </div>
                <div class="form-hint">
                  {{ t('alerts.hostsSelectedCount', { count: selectedHostIds.length }, selectedHostIds.length) }}
                </div>
              </div>
              <label class="form-check">
                <input
                  v-model="enabled"
                  class="form-check-input"
                  type="checkbox"
                >
                <span class="form-check-label">{{ t('alerts.enableCreatedRulesLabel') }}</span>
              </label>
              <div
                v-if="error"
                class="alert alert-danger mt-3 mb-0"
              >
                {{ error }}
              </div>
            </div>
            <div v-else>
              <div class="alert alert-success">
                {{ t('alerts.rulesCreatedMsg', { count: result.created_rule_ids?.length || 0 }) }}
              </div>
              <div
                v-if="result.errors && Object.keys(result.errors).length > 0"
                class="alert alert-danger"
              >
                <div class="fw-semibold mb-1">
                  {{ t('alerts.failuresLabel') }}
                </div>
                <ul class="mb-0 ps-3">
                  <li
                    v-for="(msg, hostId) in result.errors"
                    :key="hostId"
                  >
                    {{ hostName(hostId) }} : {{ msg }}
                  </li>
                </ul>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-outline-secondary"
              @click="$emit('close')"
            >
              {{ result ? t('alerts.closeButton') : t('alerts.cancelButton') }}
            </button>
            <button
              v-if="!result"
              type="button"
              class="btn btn-primary"
              :disabled="applying || selectedHostIds.length === 0"
              @click="$emit('apply', selectedHostIds, enabled)"
            >
              <span
                v-if="applying"
                class="spinner-border spinner-border-sm me-2"
              />
              {{ t('alerts.applyButton') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div
      v-if="visible"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconSearch } from '@tabler/icons-vue'
import { useModalChrome } from '../../composables/useModalChrome'
import type { AlertRuleTemplate, ApplyAlertRuleTemplateResult, Host } from '../../types/generated'

const props = withDefaults(defineProps<{
  visible?: boolean
  template?: AlertRuleTemplate | null
  hosts?: Host[]
  applying?: boolean
  error?: string
  result?: ApplyAlertRuleTemplateResult | null
}>(), {
  visible: false,
  template: null,
  hosts: () => [],
  applying: false,
  error: '',
  result: null,
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'apply', hostIds: string[], enabled: boolean): void
}>()

const { t } = useI18n()

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => props.visible, { onClose: () => emit('close') })

const search = ref('')
const selectedHostIds = ref<string[]>([])
const enabled = ref(false)

watch(() => props.visible, (v) => {
  if (v) {
    search.value = ''
    selectedHostIds.value = []
    enabled.value = false
  }
})

const filteredHosts = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return props.hosts
  return props.hosts.filter((h) =>
    (h.name || '').toLowerCase().includes(q) || (h.tags || []).some((tag) => tag.toLowerCase().includes(q))
  )
})

function hostName(hostId: string): string {
  return props.hosts.find((h) => h.id === hostId)?.name || hostId
}
</script>

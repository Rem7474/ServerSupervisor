<template>
  <div
    ref="modalRef"
    class="modal modal-blur fade show d-block"
    @click.self="emit('close')"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            {{ t('docker.composeDetailsTitle') }}
          </h5>
          <button
            type="button"
            class="btn-close"
            :aria-label="t('common.close')"
            @click="emit('close')"
          />
        </div>
        <div class="modal-body">
          <div class="mb-3">
            <label class="form-label fw-semibold">{{ t('docker.containerLabel') }}</label>
            <div>{{ container.name }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">{{ t('docker.composeProjectLabel') }}</label>
            <div>{{ composeInfo.project || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">{{ t('docker.serviceLabel') }}</label>
            <div>{{ composeInfo.service || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">{{ t('docker.workingDirLabel') }}</label>
            <div class="font-monospace small">
              {{ composeInfo.workingDir || '-' }}
            </div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">{{ t('docker.configFilesLabel') }}</label>
            <div class="font-monospace small">
              {{ composeInfo.configFiles || '-' }}
            </div>
          </div>
          <div
            v-if="Object.keys(container.labels || {}).length > 0"
            class="mb-3"
          >
            <label class="form-label fw-semibold">{{ t('docker.labelsLabel') }}</label>
            <div
              class="border rounded p-2 bg-dark small font-monospace"
              style="max-height: 200px; overflow-y: auto;"
            >
              <div
                v-for="(value, key) in container.labels"
                :key="key"
                class="mb-1"
              >
                <span class="text-muted">{{ key }}:</span> <span class="text-light">{{ value }}</span>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn"
            @click="emit('close')"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
  <div
    class="modal-backdrop fade show"
  />
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalChrome } from '../../composables/useModalChrome'
import { getComposeInfo } from '../../utils/dockerCompose'
import type { Container } from '../../types/dockerContainersTab'

const props = defineProps<{
  container: Container
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const modalRef = ref<HTMLElement | null>(null)
// The modal is created already open, so the chrome (focus trap, autofocus)
// must be armed once the element exists: with an always-true getter its
// immediate watcher would run in setup(), while modalRef is still null.
const mounted = ref(false)
onMounted(() => { mounted.value = true })
useModalChrome(modalRef, () => mounted.value, { onClose: () => emit('close') })

const composeInfo = computed(() => getComposeInfo(props.container.labels))
</script>

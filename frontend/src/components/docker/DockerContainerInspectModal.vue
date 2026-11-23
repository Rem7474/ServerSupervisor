<template>
  <div
    ref="modalRef"
    class="modal modal-blur fade show d-block"
    role="dialog"
    aria-modal="true"
    @click.self="emit('close')"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <div>
            <h5 class="modal-title">
              {{ container.name }}
            </h5>
            <div class="text-secondary small">
              {{ container.image }}:<code>{{ version?.running_version || container.image_tag }}</code>
              <span
                class="ms-2"
                :class="getEntityStateClass(container.state)"
              >{{ getEntityStateLabel(container.state) }}</span>
            </div>
          </div>
          <button
            type="button"
            class="btn-close"
            :aria-label="t('common.close')"
            @click="emit('close')"
          />
        </div>
        <div class="modal-body p-0">
          <div class="border-bottom px-3">
            <ul class="nav nav-tabs nav-tabs-alt">
              <li class="nav-item">
                <a
                  class="nav-link"
                  :class="{ active: tab === 'env' }"
                  href="#"
                  @click.prevent="tab = 'env'"
                >
                  Env Vars
                  <span class="badge bg-azure-lt text-azure ms-1">{{ Object.keys(container.env_vars || {}).length }}</span>
                </a>
              </li>
              <li class="nav-item">
                <a
                  class="nav-link"
                  :class="{ active: tab === 'volumes' }"
                  href="#"
                  @click.prevent="tab = 'volumes'"
                >
                  Volumes
                  <span class="badge bg-azure-lt text-azure ms-1">{{ (container.volumes || []).length }}</span>
                </a>
              </li>
              <li class="nav-item">
                <a
                  class="nav-link"
                  :class="{ active: tab === 'networks' }"
                  href="#"
                  @click.prevent="tab = 'networks'"
                >
                  {{ t('docker.networksTab') }}
                  <span class="badge bg-azure-lt text-azure ms-1">{{ (container.networks || []).length }}</span>
                </a>
              </li>
            </ul>
          </div>
          <div
            class="p-3"
            style="min-height: 200px; max-height: 400px; overflow-y: auto;"
          >
            <div v-if="tab === 'env'">
              <EmptyState
                v-if="Object.keys(container.env_vars || {}).length === 0"
                :title="t('docker.noEnvVars')"
              />
              <div
                v-else
                class="table-responsive"
              >
                <table class="table table-sm table-vcenter">
                  <thead><tr><th>{{ t('docker.variableColumn') }}</th><th>{{ t('docker.valueColumn') }}</th></tr></thead>
                  <tbody>
                    <tr
                      v-for="(val, key) in container.env_vars"
                      :key="key"
                    >
                      <td class="font-monospace small fw-semibold">
                        {{ key }}
                      </td>
                      <td
                        class="font-monospace small text-secondary"
                        style="word-break: break-all;"
                      >
                        {{ val }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div v-if="tab === 'volumes'">
              <EmptyState
                v-if="!(container.volumes || []).length"
                :title="t('docker.noVolumes')"
              />
              <ul
                v-else
                class="list-unstyled mb-0"
              >
                <li
                  v-for="vol in container.volumes"
                  :key="vol"
                  class="py-1 border-bottom font-monospace small"
                >
                  {{ vol }}
                </li>
              </ul>
            </div>
            <div v-if="tab === 'networks'">
              <EmptyState
                v-if="!(container.networks || []).length"
                :title="t('docker.noNetworks')"
              />
              <div
                v-else
                class="d-flex flex-wrap gap-2 pt-1"
              >
                <span
                  v-for="net in container.networks"
                  :key="net"
                  class="badge bg-blue-lt text-blue fs-6"
                >{{ net }}</span>
              </div>
              <div
                v-if="(container?.net_rx_bytes ?? 0) > 0 || (container?.net_tx_bytes ?? 0) > 0"
                class="mt-3 border-top pt-3"
              >
                <div class="text-secondary small fw-semibold mb-1">
                  {{ t('docker.cumulativeNetworkIO') }}
                </div>
                <div class="row row-sm">
                  <div class="col-6">
                    <div class="text-muted small">
                      ↓ {{ t('docker.receivedLabel') }}
                    </div>
                    <div class="fw-semibold text-info">
                      {{ formatBytes(container.net_rx_bytes) }}
                    </div>
                  </div>
                  <div class="col-6">
                    <div class="text-muted small">
                      ↑ {{ t('docker.sentLabel') }}
                    </div>
                    <div class="fw-semibold text-warning">
                      {{ formatBytes(container.net_tx_bytes) }}
                    </div>
                  </div>
                </div>
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
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'
import { useModalChrome } from '../../composables/useModalChrome'
import { getEntityStateClass, getEntityStateLabel } from '../../utils/statusClasses'
import { formatBytes as formatBytesShared } from '../../utils/formatters'
import type { Container, VersionComparison } from '../../types/dockerContainersTab'

defineProps<{
  container: Container
  version: VersionComparison | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

// Reopened on the Env Vars tab each time: the parent unmounts this modal's
// content (container = null) between inspections.
const tab = ref('env')

const modalRef = ref<HTMLElement | null>(null)
// The modal is created already open, so the chrome (focus trap, autofocus)
// must be armed once the element exists: with an always-true getter its
// immediate watcher would run in setup(), while modalRef is still null.
const mounted = ref(false)
onMounted(() => { mounted.value = true })
useModalChrome(modalRef, () => mounted.value, { onClose: () => emit('close') })

function formatBytes(bytes: number | undefined): string {
  return formatBytesShared(bytes || 0)
}
</script>

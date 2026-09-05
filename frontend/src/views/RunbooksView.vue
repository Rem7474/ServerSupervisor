<template>
  <div>
    <div class="page-header d-print-none mb-4">
      <div class="row g-2 align-items-center">
        <div class="col">
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              {{ t('runbooks.dashboardBreadcrumb') }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>Runbooks</span>
          </div>
          <h2 class="page-title">
            Runbooks
          </h2>
          <div class="text-muted small mt-1">
            {{ t('runbooks.pageSubtitle') }}
          </div>
        </div>
        <div class="col-auto ms-auto">
          <button
            type="button"
            class="btn btn-primary btn-sm"
            @click="startAdd"
          >
            <IconPlus
              :size="14"
              class="icon me-1"
            />
            {{ t('runbooks.newRunbookButton') }}
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div class="card">
      <div
        v-if="loading && !fetched"
        class="card-body text-center py-5"
      >
        <LoadingSkeleton
          variant="table"
          :lines="4"
        />
      </div>
      <div
        v-else-if="runbooks.length === 0"
        class="card-body"
      >
        <EmptyState
          :title="t('runbooks.noRunbookConfiguredTitle')"
          :subtitle="t('runbooks.createSequenceHint')"
          :cta-label="t('runbooks.createFirstRunbookButton')"
          @cta="startAdd"
        />
      </div>
      <div
        v-else
        class="table-responsive scroll-table"
      >
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('runbooks.nameColumn') }}</th>
              <th>{{ t('runbooks.stepsColumn') }}</th>
              <th class="w-1">
                {{ t('runbooks.actionsColumn') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="rb in runbooks"
              :key="rb.id"
            >
              <td>
                <div class="fw-bold">
                  {{ rb.name }}
                </div>
                <div
                  v-if="rb.description"
                  class="text-muted small"
                >
                  {{ rb.description }}
                </div>
              </td>
              <td>
                <span class="badge bg-azure-lt text-azure">{{ t('runbooks.stepsCountBadge', { n: rb.steps.length }, rb.steps.length) }}</span>
                <div class="text-muted small mt-1">
                  {{ hostNamesSummary(rb) }}
                </div>
              </td>
              <td>
                <div class="btn-group">
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    :title="t('runbooks.runButton')"
                    :disabled="runningIds.has(rb.id)"
                    @click="handleRun(rb)"
                  >
                    <span
                      v-if="runningIds.has(rb.id)"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerPlay
                      v-else
                      :size="14"
                      class="icon"
                    />
                  </button>
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    :title="t('runbooks.historyButton')"
                    @click="openHistory(rb)"
                  >
                    <IconHistory
                      :size="14"
                      class="icon"
                    />
                  </button>
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    :title="t('runbooks.editButton')"
                    @click="startEdit(rb)"
                  >
                    <IconPencil
                      :size="14"
                      class="icon"
                    />
                  </button>
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    :title="t('runbooks.deleteButton')"
                    @click="handleDelete(rb)"
                  >
                    <IconTrash
                      :size="14"
                      class="icon"
                    />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create/edit modal -->
    <div
      v-if="showModal"
      ref="editModalRef"
      class="modal modal-blur fade show d-block"
      tabindex="-1"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ editingRunbook ? t('runbooks.editRunbookTitle') : t('runbooks.newRunbookTitle') }}
            </h5>
            <button
              type="button"
              class="btn-close"
              @click="closeModal"
            />
          </div>
          <div class="modal-body">
            <div
              v-if="saveError"
              class="alert alert-danger py-2"
            >
              {{ saveError }}
            </div>
            <div class="mb-3">
              <label class="form-label">{{ t('runbooks.nameColumn') }}</label>
              <input
                v-model="form.name"
                type="text"
                class="form-control"
                :placeholder="t('runbooks.namePlaceholder')"
              >
            </div>
            <div class="mb-3">
              <label class="form-label">Description</label>
              <textarea
                v-model="form.description"
                class="form-control"
                rows="2"
                :placeholder="t('runbooks.optionalPlaceholder')"
              />
            </div>

            <label class="form-label">{{ t('runbooks.stepsOrderedLabel') }}</label>
            <div
              v-for="(step, index) in form.steps"
              :key="index"
              class="border rounded p-2 mb-2"
            >
              <div class="row g-2 align-items-end">
                <div class="col-auto pt-2 fw-bold text-muted">
                  {{ index + 1 }}
                </div>
                <div class="col">
                  <DispatchStepEditor
                    v-model:host-id="step.host_id"
                    v-model:module="step.module"
                    v-model:action="step.action"
                    v-model:target="step.target"
                    :actions-for-module="actionsForModule"
                    :target-config="runbookTargetConfig"
                  />
                </div>
                <div class="col-auto">
                  <button
                    type="button"
                    class="btn btn-sm btn-ghost-danger"
                    :title="t('runbooks.removeStepTooltip')"
                    @click="form.steps.splice(index, 1)"
                  >
                    <IconTrash
                      :size="16"
                      class="icon"
                    />
                  </button>
                </div>
              </div>
              <label class="form-check mt-2 mb-0">
                <input
                  v-model="step.continue_on_failure"
                  class="form-check-input"
                  type="checkbox"
                >
                <span class="form-check-label small">{{ t('runbooks.continueOnFailureLabel') }}</span>
              </label>
            </div>
            <button
              type="button"
              class="btn btn-outline-secondary btn-sm"
              @click="form.steps.push(emptyStep())"
            >
              <IconPlus
                :size="16"
                class="icon me-1"
              />
              {{ t('runbooks.addStepButton') }}
            </button>
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-outline-secondary"
              :disabled="saving"
              @click="closeModal"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="saving || !canSave"
              @click="handleSave"
            >
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div
      v-if="showModal"
      class="modal-backdrop fade show"
    />

    <!-- History modal -->
    <div
      v-if="historyRunbook"
      ref="historyModalRef"
      class="modal modal-blur fade show d-block"
      tabindex="-1"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ t('runbooks.historyTitle', { name: historyRunbook.name }) }}
            </h5>
            <button
              type="button"
              class="btn-close"
              @click="closeHistory"
            />
          </div>
          <div class="modal-body">
            <div v-if="executionsLoading">
              <LoadingSkeleton variant="table" />
            </div>
            <EmptyState
              v-else-if="executions.length === 0"
              :title="t('runbooks.noExecutionForRunbookTitle')"
            />
            <div
              v-else
              class="table-responsive scroll-table"
            >
              <table class="table table-sm table-vcenter">
                <thead>
                  <tr>
                    <th>{{ t('runbooks.stateColumn') }}</th>
                    <th>{{ t('runbooks.triggeredByColumn') }}</th>
                    <th>{{ t('runbooks.startedColumn') }}</th>
                    <th>{{ t('runbooks.completedColumn') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <template
                    v-for="exec in executions"
                    :key="exec.id"
                  >
                    <tr
                      class="clickable-row"
                      role="button"
                      tabindex="0"
                      @click="selectExecution(exec)"
                      @keydown.enter="selectExecution(exec)"
                      @keydown.space.prevent="selectExecution(exec)"
                    >
                      <td>
                        <span
                          class="badge"
                          :class="executionBadgeClass(exec.status)"
                        >{{ executionStatusLabel(exec.status) }}</span>
                      </td>
                      <td>{{ exec.triggered_by }}</td>
                      <td class="text-muted small">
                        {{ formatDate(exec.started_at) }}
                      </td>
                      <td class="text-muted small">
                        {{ exec.completed_at ? formatDate(exec.completed_at) : '-' }}
                      </td>
                    </tr>
                    <tr v-if="selectedExecution?.id === exec.id">
                      <td
                        colspan="4"
                        class="bg-dark-subtle"
                      >
                        <table class="table table-sm mb-0">
                          <thead>
                            <tr>
                              <th>#</th>
                              <th>{{ t('runbooks.hostColumn') }}</th>
                              <th>Action</th>
                              <th>{{ t('runbooks.stateColumn') }}</th>
                              <th />
                            </tr>
                          </thead>
                          <tbody>
                            <tr
                              v-for="s in selectedExecution.steps"
                              :key="s.position"
                            >
                              <td>{{ s.position + 1 }}</td>
                              <td>{{ hostName(s.host_id) }}</td>
                              <td><code>{{ s.module }}/{{ s.action }}{{ s.target ? ' → ' + s.target : '' }}</code></td>
                              <td>
                                <span
                                  v-if="s.status"
                                  class="badge"
                                  :class="executionBadgeClass(s.status)"
                                >{{ executionStatusLabel(s.status) }}
                                  <span
                                    v-if="s.status === 'running' || s.status === 'pending'"
                                    class="spinner-border spinner-border-sm ms-1"
                                  /></span>
                                <span
                                  v-else
                                  class="text-muted small"
                                >{{ t('runbooks.pendingLowerLabel') }}</span>
                              </td>
                              <td>
                                <button
                                  v-if="s.command_id"
                                  type="button"
                                  class="btn btn-sm btn-ghost-secondary"
                                  :title="t('runbooks.viewStepLogsTooltip')"
                                  @click="openStepLogs(s)"
                                >
                                  <IconFileText :size="16" />
                                </button>
                              </td>
                            </tr>
                          </tbody>
                        </table>
                      </td>
                    </tr>
                  </template>
                </tbody>
              </table>
            </div>

            <div
              v-if="showStepLogPanel"
              class="mt-3"
              style="height: 320px;"
            >
              <CommandLogPanel
                :command="selectedStepCommand"
                :show="showStepLogPanel"
                :title="t('runbooks.stepLogsTitle')"
                :empty-text="t('runbooks.noStepSelectedTitle')"
                @close="closeStepLogs"
                @open="showStepLogPanel = true"
              />
            </div>
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-outline-secondary"
              @click="closeHistory"
            >
              {{ t('common.close') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div
      v-if="historyRunbook"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconFileText, IconHistory, IconPencil, IconPlayerPlay, IconPlus, IconTrash } from '@tabler/icons-vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import DispatchStepEditor from '../components/DispatchStepEditor.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import {
  useRunbooks, actionsForModule, moduleRequiresTarget, emptyStep,
} from '../composables/useRunbooks'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useModalChrome } from '../composables/useModalChrome'
import type { Runbook, RunbookStepCreate } from '../types/generated'

const {
  hostsStore, runbooks, loading, fetched, error,
  showModal, editingRunbook, saving, saveError,
  historyRunbook, executions, executionsLoading, selectedExecution, runningIds,
  loadRunbooks, startAdd, startEdit, closeModal, saveRunbook, deleteRunbook, runRunbook,
  openHistory, closeHistory, selectExecution,
  selectedStepCommand, showStepLogPanel, openStepLogs, closeStepLogs,
} = useRunbooks()

const { t, locale } = useI18n()

const editModalRef = ref<HTMLElement | null>(null)
const historyModalRef = ref<HTMLElement | null>(null)
useModalChrome(editModalRef, () => showModal.value, { onClose: closeModal })
useModalChrome(historyModalRef, () => !!historyRunbook.value, { onClose: closeHistory })

const dialog = useConfirmDialog()

const form = reactive<{ name: string; description: string; steps: RunbookStepCreate[] }>({
  name: '', description: '', steps: [emptyStep()],
})

watch(showModal, (open) => {
  if (!open) return
  if (editingRunbook.value) {
    form.name = editingRunbook.value.name
    form.description = editingRunbook.value.description
    form.steps = editingRunbook.value.steps.map((s) => ({
      host_id: s.host_id, module: s.module, action: s.action, target: s.target,
      payload: s.payload, continue_on_failure: s.continue_on_failure,
    }))
  } else {
    form.name = ''
    form.description = ''
    form.steps = [emptyStep()]
  }
})

const canSave = computed(() =>
  form.name.trim().length > 0 &&
  form.steps.length > 0 &&
  form.steps.every((s) => s.host_id && s.module && s.action)
)

function runbookTargetConfig(module: string): { label: string; placeholder?: string } | null {
  if (!moduleRequiresTarget(module)) return null
  return { label: t('runbooks.targetLabel'), placeholder: module === 'custom' ? t('runbooks.taskIdPlaceholder') : t('runbooks.serviceNamePlaceholder') }
}

async function handleSave(): Promise<void> {
  await saveRunbook(form.name.trim(), form.description.trim(), form.steps)
}

async function handleDelete(rb: Runbook): Promise<void> {
  const confirmed = await dialog.confirm({
    title: t('runbooks.deleteRunbookConfirmTitle'),
    message: t('runbooks.deleteRunbookConfirmMessage', { name: rb.name }),
    variant: 'danger',
  })
  if (!confirmed) return
  await deleteRunbook(rb)
}

async function handleRun(rb: Runbook): Promise<void> {
  const confirmed = await dialog.confirm({
    title: t('runbooks.runRunbookConfirmTitle'),
    message: t('runbooks.runRunbookConfirmMessage', { name: rb.name, count: rb.steps.length }, rb.steps.length),
    variant: 'warning',
  })
  if (!confirmed) return
  await runRunbook(rb)
}

function hostName(hostId: string): string {
  const host = hostsStore.hosts.find((h) => h.id === hostId)
  return host?.name || host?.hostname || hostId
}

function hostNamesSummary(rb: Runbook): string {
  const names = Array.from(new Set(rb.steps.map((s) => hostName(s.host_id))))
  return names.join(', ')
}

function executionStatusLabel(status: string): string {
  if (status === 'running') return t('common.stateRunning')
  if (status === 'completed') return t('common.stateCompleted')
  if (status === 'failed') return t('common.stateFailed')
  if (status === 'pending') return t('common.statePending')
  return status || t('common.statusUnknown')
}

function executionBadgeClass(status: string): string {
  if (status === 'completed') return 'bg-success-lt text-success'
  if (status === 'failed') return 'bg-danger-lt text-danger'
  if (status === 'running' || status === 'pending') return 'bg-primary-lt text-primary'
  return 'bg-secondary-lt text-secondary'
}

function formatDate(dateStr: string | undefined | null): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString(locale.value)
}

onMounted(async () => {
  await Promise.all([loadRunbooks(), hostsStore.fetchHosts()])
})
</script>

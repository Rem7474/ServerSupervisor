<template>
  <div class="mb-3">
    <label class="form-check mb-2">
      <input
        :checked="enabled"
        class="form-check-input"
        type="checkbox"
        @change="emit('update:enabled', ($event.target as HTMLInputElement).checked)"
      >
      <span class="form-check-label fw-medium">{{ t('alerts.triggerCommandLabel') }}</span>
    </label>

    <div
      v-if="enabled"
      class="border rounded p-3 bg-dark-subtle"
    >
      <!-- Docker-scoped rules resolve module + target automatically from the
           rule's own scope (a specific container, a compose project, or the
           incident's container) — there is nothing to pick, only the action. -->
      <div
        v-if="isDockerRule"
        class="row g-2 align-items-end"
      >
        <div class="col-md-6">
          <label class="form-label form-label-sm">{{ t('alerts.actionLabel') }}</label>
          <select
            :value="modelValue.action"
            class="form-select form-select-sm"
            @change="onActionChange(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="action in commandActions"
              :key="action.value"
              :value="action.value"
            >
              {{ action.label }}
            </option>
          </select>
        </div>

        <div class="col-md-6 d-flex align-items-end">
          <div class="text-muted small">
            <span class="badge bg-teal-lt text-teal me-1">Docker</span>
            {{ dockerTargetHint }}
          </div>
        </div>
      </div>

      <DispatchStepEditor
        v-else
        v-model:module="local.module"
        v-model:action="local.action"
        v-model:target="local.target"
        :show-host="false"
        :modules="ALERT_TRIGGER_MODULES"
        :actions-for-module="alertActionsForModule"
        :target-config="alertTargetConfig"
      />

      <small
        id="command-target-hint"
        class="form-hint mt-1"
      >{{ t('alerts.commandTriggerHint') }}</small>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DispatchStepEditor from '../DispatchStepEditor.vue'
import type { DispatchOption } from '../../utils/dispatchStep'

type CommandModule = 'processes' | 'journal' | 'systemd' | 'docker'

interface CommandTrigger {
  module: CommandModule | string
  action: string
  target: string
}

interface DockerScope {
  scope_mode?: string
  container_id?: string
  project_name?: string
}

const props = withDefaults(defineProps<{
  enabled?: boolean
  modelValue: CommandTrigger
  dockerScope?: DockerScope | null
}>(), {
  enabled: false,
  dockerScope: null,
})

const emit = defineEmits<{
  (e: 'update:enabled', value: boolean): void
  (e: 'update:modelValue', value: CommandTrigger): void
}>()

const { t, te } = useI18n()

const isDockerRule = computed(() => !!props.dockerScope)

// When a docker rule enables the trigger, lock module to 'docker'.
watch(
  () => [props.enabled, isDockerRule.value] as const,
  ([enabled, isDocker]) => {
    if (enabled && isDocker && props.modelValue.module !== 'docker') {
      emit('update:modelValue', { module: 'docker', action: 'logs', target: '' })
    }
  },
  { immediate: true },
)

interface ActionOption {
  value: string
  label: string
}

// Labels are deliberately incomplete — some actions (status/read/list) never
// got a translated label and fall back to the raw value below, matching the
// original behavior rather than "fixing" copy nobody asked to change.
function actionLabel(action: string): string {
  const key = `alerts.actionLabels.${action}`
  return te(key) ? t(key) : action
}

const commandActions = computed((): ActionOption[] => {
  const isCompose = props.dockerScope?.scope_mode === 'compose_project'
  const actions = isCompose
    ? ['compose_up', 'compose_down', 'compose_pull', 'compose_logs', 'compose_restart', 'logs', 'restart', 'start', 'stop']
    : ['logs', 'restart', 'start', 'stop']
  return actions.map(v => ({ value: v, label: actionLabel(v) }))
})

const dockerTargetHint = computed((): string => {
  const scope = props.dockerScope
  if (!scope) return ''
  if (scope.scope_mode === 'compose_project') return t('alerts.dockerTargetComposeProject', { name: scope.project_name || '—' })
  if (scope.scope_mode === 'container') return t('alerts.dockerTargetContainer')
  return t('alerts.dockerTargetIncident')
})

function onActionChange(action: string): void {
  emit('update:modelValue', { ...props.modelValue, action })
}

// Non-docker-rule branch: module/action/target are all user-editable via the
// shared DispatchStepEditor. This module list is narrower than
// DISPATCH_MODULES (no apt, no custom) — a pre-existing product scoping this
// migration preserves rather than silently widens.
const ALERT_TRIGGER_MODULES = computed((): DispatchOption[] => [
  { value: 'processes', label: t('alerts.moduleProcesses') },
  { value: 'journal', label: t('alerts.moduleJournal') },
  { value: 'systemd', label: t('alerts.moduleSystemd') },
  { value: 'docker', label: t('alerts.moduleDocker') },
])

const ALERT_MODULE_ACTIONS: Record<string, string[]> = {
  processes: ['list'],
  journal: ['read'],
  systemd: ['status', 'start', 'stop', 'restart'],
  docker: ['logs', 'restart', 'start', 'stop'],
}

function alertActionsForModule(mod: string): DispatchOption[] {
  const actions = ALERT_MODULE_ACTIONS[mod] || ['list']
  return actions.map(v => ({ value: v, label: actionLabel(v) }))
}

function alertTargetConfig(mod: string): { label: string; placeholder?: string } | null {
  if (mod !== 'journal' && mod !== 'systemd' && mod !== 'docker') return null
  return { label: t('alerts.targetLabel'), placeholder: mod === 'docker' ? t('alerts.targetPlaceholderContainer') : t('alerts.targetPlaceholderService') }
}

// DispatchStepEditor expects real v-models bound to a locally-owned reactive
// object (the same shape RunbooksView's `step` and GlobalScheduledTasksView's
// `createForm` already give it) — but modelValue here is a fully controlled
// prop instead. `local` is a synchronous local mirror, bound to
// DispatchStepEditor directly (`v-model:module="local.module"`) so its own
// module-change handler's cascade of writes (module, then action, then
// target, all within one synchronous tick) lands on a plain reactive object
// with no round-trip through a computed setter re-reading a prop that only
// updates after the parent's next render pass — that earlier design clobbered
// each write with the still-stale `props.modelValue` from before the cascade
// started. A single deep watch pushes the settled object up as one update.
const local = reactive<CommandTrigger>({ ...props.modelValue })

watch(() => props.modelValue, (v) => {
  if (v.module !== local.module || v.action !== local.action || v.target !== local.target) {
    local.module = v.module
    local.action = v.action
    local.target = v.target
  }
})

watch(local, (v) => emit('update:modelValue', { ...v }), { deep: true })
</script>

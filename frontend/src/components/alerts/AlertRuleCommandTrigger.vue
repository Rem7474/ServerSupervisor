<template>
  <div class="mb-3">
    <label class="form-check mb-2">
      <input
        :checked="enabled"
        class="form-check-input"
        type="checkbox"
        @change="emit('update:enabled', ($event.target as HTMLInputElement).checked)"
      >
      <span class="form-check-label fw-medium">Déclencher une commande à l'alerte</span>
    </label>

    <div
      v-if="enabled"
      class="border rounded p-3 bg-dark-subtle"
    >
      <div class="row g-2 align-items-end">
        <div
          v-if="!isDockerRule"
          class="col-md-4"
        >
          <label class="form-label form-label-sm">Module</label>
          <select
            :value="modelValue.module"
            class="form-select form-select-sm"
            @change="onModuleChange(($event.target as HTMLSelectElement).value as CommandModule)"
          >
            <option value="processes">
              Processus (top)
            </option>
            <option value="journal">
              Journal systemd
            </option>
            <option value="systemd">
              Service systemd
            </option>
            <option value="docker">
              Conteneur Docker
            </option>
          </select>
        </div>

        <div :class="isDockerRule ? 'col-md-6' : 'col-md-4'">
          <label class="form-label form-label-sm">Action</label>
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

        <div
          v-if="!isDockerRule && commandNeedsTarget"
          class="col-md-4"
        >
          <label class="form-label form-label-sm">Cible</label>
          <input
            :value="modelValue.target"
            class="form-control form-control-sm"
            :placeholder="commandTargetPlaceholder"
            aria-describedby="command-target-hint"
            @input="onTargetChange(($event.target as HTMLInputElement).value)"
          >
        </div>

        <div
          v-if="isDockerRule"
          class="col-md-6 d-flex align-items-end"
        >
          <div class="text-muted small">
            <span class="badge bg-teal-lt text-teal me-1">Docker</span>
            {{ dockerTargetHint }}
          </div>
        </div>
      </div>

      <small
        id="command-target-hint"
        class="form-hint mt-1"
      >La commande sera créée automatiquement sur l'hôte concerné dès le déclenchement de l'alerte.</small>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'

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

const ACTION_LABELS: Record<string, string> = {
  logs: 'Voir les logs',
  restart: 'Redémarrer',
  start: 'Démarrer',
  stop: 'Arrêter',
  compose_up: 'Compose up',
  compose_down: 'Compose down',
  compose_pull: 'Mettre à jour les images',
  compose_logs: 'Voir les logs Compose',
  compose_restart: 'Redémarrer (Compose)',
}

const commandModuleActions: Record<CommandModule, string[]> = {
  processes: ['list'],
  journal: ['read'],
  systemd: ['status', 'start', 'stop', 'restart'],
  docker: ['logs', 'restart', 'start', 'stop'],
}

const commandActions = computed((): ActionOption[] => {
  if (isDockerRule.value) {
    const isCompose = props.dockerScope?.scope_mode === 'compose_project'
    const actions = isCompose
      ? ['compose_up', 'compose_down', 'compose_pull', 'compose_logs', 'compose_restart', 'logs', 'restart', 'start', 'stop']
      : ['logs', 'restart', 'start', 'stop']
    return actions.map(v => ({ value: v, label: ACTION_LABELS[v] || v }))
  }
  const moduleName = (props.modelValue.module || 'processes') as CommandModule
  const actions = commandModuleActions[moduleName] || ['list']
  return actions.map(v => ({ value: v, label: ACTION_LABELS[v] || v }))
})

const commandNeedsTarget = computed(() => {
  const moduleName = props.modelValue.module
  return moduleName === 'journal' || moduleName === 'systemd' || moduleName === 'docker'
})

const commandTargetPlaceholder = computed(() => {
  const moduleName = props.modelValue.module
  if (moduleName === 'journal' || moduleName === 'systemd') return 'nom du service (ex: nginx)'
  if (moduleName === 'docker') return 'nom du conteneur'
  return ''
})

const dockerTargetHint = computed((): string => {
  const scope = props.dockerScope
  if (!scope) return ''
  if (scope.scope_mode === 'compose_project') return `Projet « ${scope.project_name || '—'} » (résolu au déclenchement)`
  if (scope.scope_mode === 'container') return 'Conteneur ciblé par la règle (résolu au déclenchement)'
  return 'Conteneur concerné par l\'incident (résolu au déclenchement)'
})

function onModuleChange(mod: CommandModule): void {
  const nextActions = commandModuleActions[mod] || ['list']
  emit('update:modelValue', { module: mod, action: nextActions[0], target: '' })
}

function onActionChange(action: string): void {
  emit('update:modelValue', { ...props.modelValue, action })
}

function onTargetChange(target: string): void {
  emit('update:modelValue', { ...props.modelValue, target })
}
</script>

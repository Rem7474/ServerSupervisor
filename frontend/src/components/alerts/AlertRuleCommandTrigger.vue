<template>
  <div class="mb-3">
    <label class="form-check mb-2">
      <input
        :checked="enabled"
        class="form-check-input"
        type="checkbox"
        @change="$emit('update:enabled', $event.target.checked)"
      >
      <span class="form-check-label fw-medium">Declencher une commande a l'alerte</span>
    </label>

    <div
      v-if="enabled"
      class="border rounded p-3 bg-dark-subtle"
    >
      <div class="row g-2">
        <div class="col-md-4">
          <label class="form-label form-label-sm">Module</label>
          <select
            :value="modelValue.module"
            class="form-select form-select-sm"
            @change="onModuleChange($event.target.value)"
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

        <div class="col-md-4">
          <label class="form-label form-label-sm">Action</label>
          <select
            :value="modelValue.action"
            class="form-select form-select-sm"
            @change="onActionChange($event.target.value)"
          >
            <option
              v-for="action in commandActions"
              :key="action"
              :value="action"
            >
              {{ action }}
            </option>
          </select>
        </div>

        <div
          v-if="commandNeedsTarget"
          class="col-md-4"
        >
          <label class="form-label form-label-sm">Cible</label>
          <input
            :value="modelValue.target"
            class="form-control form-control-sm"
            :placeholder="commandTargetPlaceholder"
            aria-describedby="command-target-hint"
            @input="onTargetChange($event.target.value)"
          >
        </div>
      </div>
      <small
        id="command-target-hint"
        class="form-hint mt-1"
      >La commande sera creee automatiquement sur l'hote concerne des le declenchement de l'alerte.</small>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  enabled: {
    type: Boolean,
    default: false,
  },
  modelValue: {
    type: Object,
    required: true,
  },
})

const emit = defineEmits(['update:enabled', 'update:modelValue'])

const commandModuleActions = {
  processes: ['list'],
  journal: ['read'],
  systemd: ['status', 'start', 'stop', 'restart'],
  docker: ['logs', 'restart', 'start', 'stop'],
}

const commandActions = computed(() => {
  const moduleName = props.modelValue.module || 'processes'
  return commandModuleActions[moduleName] || ['list']
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

function onModuleChange(module) {
  const nextActions = commandModuleActions[module] || ['list']
  emit('update:modelValue', {
    module,
    action: nextActions[0],
    target: '',
  })
}

function onActionChange(action) {
  emit('update:modelValue', {
    ...props.modelValue,
    action,
  })
}

function onTargetChange(target) {
  emit('update:modelValue', {
    ...props.modelValue,
    target,
  })
}
</script>

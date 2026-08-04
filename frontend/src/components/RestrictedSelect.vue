<template>
  <select
    v-if="hasOptions"
    v-model="model"
    v-bind="$attrs"
    class="form-select form-select-sm"
  >
    <option value="">
      {{ emptyLabel }}
    </option>
    <template v-if="isGrouped(options)">
      <optgroup
        v-for="group in options"
        :key="group.label"
        :label="group.label"
      >
        <option
          v-for="opt in group.options"
          :key="optionValue(opt)"
          :value="optionValue(opt)"
        >
          {{ optionLabel(opt) }}
        </option>
      </optgroup>
    </template>
    <template v-else>
      <option
        v-for="opt in options"
        :key="optionValue(opt)"
        :value="optionValue(opt)"
      >
        {{ optionLabel(opt) }}
      </option>
    </template>
  </select>
  <input
    v-else
    v-model="model"
    type="text"
    v-bind="$attrs"
    class="form-control form-control-sm"
    :placeholder="placeholder"
  >
</template>

<script setup lang="ts">
import { computed } from 'vue'

// A <select> restricted to a known, discovered set of values (e.g. resticprofile
// profiles, custom task IDs) — falls back to a free-text <input> when the set
// is empty, so the field isn't unusable before the first agent report lands
// (or the host has never reported this data). Extracted from 3 near-identical
// blocks in HostBackupTab.vue / DispatchStepEditor.vue.
//
// An option can be a plain string (value === label, e.g. a restic profile
// name) or a {value, label} pair (e.g. a custom task: value is its id, label
// is "name (id)"). `options` can be flat or grouped under labelled sections
// (e.g. "Profils" / "Groupes") — see isGrouped below for how the two shapes
// are told apart.
export type SelectOption = string | { value: string; label: string }
export type OptionGroup = { label: string; options: SelectOption[] }

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  options: SelectOption[] | OptionGroup[]
  emptyLabel: string
  placeholder?: string
}>()

const model = defineModel<string>({ required: true })

function isGrouped(options: SelectOption[] | OptionGroup[]): options is OptionGroup[] {
  return options.length > 0 && typeof options[0] === 'object' && 'options' in (options[0] as OptionGroup)
}

const hasOptions = computed(() => {
  if (isGrouped(props.options)) return props.options.some((g) => g.options.length > 0)
  return props.options.length > 0
})

function optionValue(opt: SelectOption): string {
  return typeof opt === 'string' ? opt : opt.value
}

function optionLabel(opt: SelectOption): string {
  return typeof opt === 'string' ? opt : opt.label
}
</script>

<template>
  <span
    :class="badgeClass"
    :aria-label="resolvedAriaLabel"
    :title="resolvedTitle"
  >
    <slot>{{ text }}</slot>
  </span>
</template>

<script setup>
import { computed } from 'vue'

// Define valid tone values as reference (not used in validator to avoid hoisting issues)
const VALID_TONES = ['success', 'warning', 'danger', 'info', 'secondary', 'orange', 'cyan', 'blue', 'green', 'red', 'purple', 'azure', 'teal', 'indigo']

const toneClasses = {
  success: 'bg-success-lt text-success',
  warning: 'bg-yellow-lt text-yellow',
  danger: 'bg-danger-lt text-danger',
  info: 'bg-blue-lt text-blue',
  secondary: 'bg-secondary-lt text-secondary',
  orange: 'bg-orange-lt text-orange',
  cyan: 'bg-cyan-lt text-cyan',
  blue: 'bg-blue-lt text-blue',
  green: 'bg-green-lt text-green',
  red: 'bg-red-lt text-red',
  purple: 'bg-purple-lt text-purple',
  azure: 'bg-azure-lt text-azure',
  teal: 'bg-teal-lt text-teal',
  indigo: 'bg-indigo-lt text-indigo',
}

const props = defineProps({
  text: {
    type: String,
    default: '',
  },
  tone: {
    type: String,
    default: 'secondary',
  },
  title: {
    type: String,
    default: '',
  },
  ariaLabel: {
    type: String,
    default: '',
  },
  compact: {
    type: Boolean,
    default: false,
  },
  monospace: {
    type: Boolean,
    default: false,
  },
})

const badgeClass = computed(() => [
  'badge',
  'rounded-pill',
  'd-inline-flex',
  'align-items-center',
  'gap-1',
  props.compact ? 'px-2 py-1' : '',
  toneClasses[props.tone] || toneClasses.secondary,
  props.monospace ? 'font-monospace' : '',
].filter(Boolean).join(' '))

const resolvedTitle = computed(() => props.title || props.text)
const resolvedAriaLabel = computed(() => props.ariaLabel || props.title || props.text)
</script>
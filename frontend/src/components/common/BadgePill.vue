<template>
  <span
    :class="badgeClass"
    :aria-label="resolvedAriaLabel"
    :title="resolvedTitle"
  >
    <slot>{{ text }}</slot>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

type Tone =
  | 'success' | 'warning' | 'danger' | 'info' | 'secondary'
  | 'orange' | 'cyan' | 'blue' | 'green' | 'red'
  | 'purple' | 'azure' | 'teal' | 'indigo'

const toneClasses: Record<Tone, string> = {
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

const props = withDefaults(defineProps<{
  text?: string
  tone?: Tone
  title?: string
  ariaLabel?: string
  compact?: boolean
  monospace?: boolean
}>(), {
  text: '',
  tone: 'secondary',
  title: '',
  ariaLabel: '',
  compact: false,
  monospace: false,
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

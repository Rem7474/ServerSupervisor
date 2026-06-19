<template>
  <div class="text-center py-5 text-muted">
    <slot name="icon">
      <component
        :is="icon"
        :size="iconSize"
        class="mb-3"
        :stroke-width="1.5"
        style="opacity:.35"
      />
    </slot>
    <p class="h5 mt-2 mb-1">
      {{ title }}
    </p>
    <p
      v-if="subtitle"
      class="text-secondary small mt-1 mb-0"
    >
      {{ subtitle }}
    </p>
    <div
      v-if="ctaLabel"
      class="mt-3"
    >
      <router-link
        v-if="ctaTo"
        :to="ctaTo"
        class="btn btn-primary btn-sm"
      >
        {{ ctaLabel }}
      </router-link>
      <button
        v-else
        type="button"
        class="btn btn-primary btn-sm"
        @click="$emit('cta')"
      >
        {{ ctaLabel }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'
import { IconInbox } from '@tabler/icons-vue'

withDefaults(defineProps<{
  title: string
  subtitle?: string
  ctaLabel?: string
  ctaTo?: string
  iconSize?: number
  icon?: Component
}>(), {
  icon: IconInbox,
  subtitle: '',
  ctaLabel: '',
  ctaTo: '',
  iconSize: 48,
})
defineEmits<{
  (e: 'cta'): void
}>()
</script>

<template>
  <div class="text-center py-5 text-muted">
    <slot name="icon">
      <svg xmlns="http://www.w3.org/2000/svg" class="mb-3" :width="iconSize" :height="iconSize" fill="none" stroke="currentColor" viewBox="0 0 24 24" style="opacity:.35">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/>
      </svg>
    </slot>
    <p class="h5 mt-2 mb-1">{{ title }}</p>
    <p v-if="subtitle" class="text-secondary small mt-1 mb-0">{{ subtitle }}</p>
    <div v-if="ctaLabel" class="mt-3">
      <router-link v-if="ctaTo" :to="ctaTo" class="btn btn-primary btn-sm">{{ ctaLabel }}</router-link>
      <button v-else class="btn btn-primary btn-sm" @click="$emit('cta')">{{ ctaLabel }}</button>
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  ctaLabel: { type: String, default: '' },
  ctaTo: { type: String, default: '' },
  iconSize: { type: Number, default: 48 },
})
defineEmits(['cta'])
</script>

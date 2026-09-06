<template>
  <div class="page-refresh-bar">
    <div class="d-flex align-items-center gap-2 flex-wrap">
      <span
        class="live-dot"
        :class="{ paused: !modelValue }"
      />
      <span
        v-if="label"
        class="fw-semibold"
      >{{ label }}</span>
      <button
        class="badge border-0 cursor-pointer"
        :class="modelValue ? 'bg-success-lt text-success' : 'bg-secondary-lt text-secondary'"
        type="button"
        :title="modelValue ? t('common.pageRefreshPauseTooltip') : t('common.pageRefreshResumeTooltip')"
        @click="$emit('update:modelValue', !modelValue)"
      >
        {{ modelValue ? t('common.pageRefreshAutoLabel', { n: intervalSec }) : t('common.pageRefreshPausedLabel') }}
      </button>
      <span class="text-secondary small">{{ t('common.pageRefreshLastUpdateLabel', { time: lastUpdatedLabel }) }}</span>
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t, locale } = useI18n()

const props = defineProps<{
  modelValue: boolean
  intervalSec: number
  lastUpdatedAt: Date | null
  label?: string
}>()

defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const lastUpdatedLabel = computed(() => {
  if (!props.lastUpdatedAt) return t('common.never')
  return props.lastUpdatedAt.toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
})
</script>

<style scoped>
.page-refresh-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
  margin-bottom: 0.75rem;
}

.live-dot {
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  border-radius: 999px;
  background: var(--ss-status-online);
  animation: pulse-dot 1.6s infinite;
}

.live-dot.paused {
  animation: none;
  background: var(--tblr-secondary);
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}

.cursor-pointer { cursor: pointer; }
</style>

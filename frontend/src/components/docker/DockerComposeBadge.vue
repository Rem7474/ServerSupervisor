<template>
  <div
    v-if="composeInfo.project"
    class="small"
  >
    <div class="text-primary fw-semibold">
      {{ composeInfo.project }}
    </div>
    <div
      v-if="!redundant"
      class="text-secondary"
    >
      {{ composeInfo.service }}
    </div>
  </div>
  <span
    v-else
    class="text-secondary"
  >-</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { getComposeInfo, isComposeServiceRedundant } from '../../utils/dockerCompose'

const props = defineProps<{
  labels?: Record<string, string>
}>()

const composeInfo = computed(() => getComposeInfo(props.labels))
const redundant = computed(() => isComposeServiceRedundant(props.labels))
</script>

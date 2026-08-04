<template>
  <div v-if="errors.length || warnings.length">
    <div
      v-if="errors.length"
      class="alert alert-danger mb-2"
    >
      <div class="d-flex align-items-start gap-2">
        <IconAlertCircle
          :size="24"
          class="flex-shrink-0"
        />
        <div>
          <h4 class="alert-title">
            Configuration de l'agent incomplète
          </h4>
          <ul class="mb-0 ps-3">
            <li
              v-for="(issue, i) in errors"
              :key="`error-${i}`"
            >
              <strong>{{ issue.collector }}</strong> — {{ issue.message }}
            </li>
          </ul>
        </div>
      </div>
    </div>
    <div
      v-if="warnings.length"
      class="alert alert-warning mb-2"
    >
      <div class="d-flex align-items-start gap-2">
        <IconAlertTriangle
          :size="24"
          class="flex-shrink-0"
        />
        <div>
          <h4 class="alert-title">
            Fonctionnalités partiellement dégradées
          </h4>
          <ul class="mb-0 ps-3">
            <li
              v-for="(issue, i) in warnings"
              :key="`warning-${i}`"
            >
              <strong>{{ issue.collector }}</strong> — {{ issue.message }}
            </li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IconAlertCircle, IconAlertTriangle } from '@tabler/icons-vue'
import type { DiagnosticIssue } from '../../types/host'

const props = defineProps<{
  diagnostics: DiagnosticIssue[] | undefined
}>()

const errors = computed(() => (props.diagnostics || []).filter((d) => d.severity === 'error'))
const warnings = computed(() => (props.diagnostics || []).filter((d) => d.severity === 'warning'))
</script>

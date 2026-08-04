<template>
  <div v-if="errors.length || warnings.length">
    <div
      v-if="freshnessNote"
      class="text-secondary small mb-2"
    >
      {{ freshnessNote }}
    </div>
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
              <strong>{{ collectorLabel(issue.collector) }}</strong> — {{ issue.message }}
              <a
                v-if="issue.collector === 'restic'"
                :href="RESTIC_BACKUP_DOC_URL"
                target="_blank"
                rel="noopener noreferrer"
                class="ms-1"
              >Voir le guide de configuration →</a>
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
              <strong>{{ collectorLabel(issue.collector) }}</strong> — {{ issue.message }}
              <a
                v-if="issue.collector === 'restic'"
                :href="RESTIC_BACKUP_DOC_URL"
                target="_blank"
                rel="noopener noreferrer"
                class="ms-1"
              >Voir le guide de configuration →</a>
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
import { formatRelativeTime } from '../../composables/useDateFormatter'
import { RESTIC_BACKUP_DOC_URL } from '../../utils/docLinks'
import type { DiagnosticIssue } from '../../types/host'

const props = defineProps<{
  diagnostics: DiagnosticIssue[] | undefined
  lastSeen?: string
  hostStatus?: string
}>()

// issue.collector is the raw agent.yaml key (collect_<key>) — not meant for
// display. Falls back to the raw key for any collector added agent-side
// before this map is updated.
const COLLECTOR_LABELS: Record<string, string> = {
  apt: 'APT',
  docker: 'Docker',
  smart: 'SMART',
  cpu_temperature: 'Température CPU',
  web_logs: 'Logs web',
  crowdsec: 'CrowdSec',
  restic: 'Restic',
}

function collectorLabel(collector: string): string {
  return COLLECTOR_LABELS[collector] || collector
}

const errors = computed(() => (props.diagnostics || []).filter((d) => d.severity === 'error'))
const warnings = computed(() => (props.diagnostics || []).filter((d) => d.severity === 'warning'))

// Diagnostics are only recomputed on the agent's periodic report — a fixed
// issue stays visible until the next one lands, and a host that's currently
// offline is showing whatever it last reported, possibly hours old. Both
// share the same note so the freshness caveat isn't missed.
const freshnessNote = computed(() => {
  const since = props.lastSeen ? formatRelativeTime(props.lastSeen) : null
  if (props.hostStatus && props.hostStatus !== 'online') {
    return `Hôte hors ligne — ces informations datent du dernier rapport reçu${since ? ` (${since})` : ''}.`
  }
  if (since) {
    return `D'après le dernier rapport de l'agent, ${since}.`
  }
  return null
})
</script>

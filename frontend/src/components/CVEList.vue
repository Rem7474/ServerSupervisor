<template>
  <div v-if="cves.length > 0" class="my-2">
    <div v-if="showMaxSeverity" class="d-flex align-items-center mb-2">
      <span class="fw-semibold me-2">Criticité max:</span>
      <span :class="maxSeverityClass" class="badge">
        {{ maxSeverity }}
      </span>
      <span class="text-secondary small ms-2">({{ cves.length }} CVE{{ cves.length > 1 ? 's' : '' }})</span>
    </div>

    <div class="text-secondary small mb-2">
      {{ packageGroups.length }} paquet{{ packageGroups.length > 1 ? 's' : '' }} avec CVE
    </div>
    
    <div v-if="!collapsed || alwaysExpanded" class="cve-groups">
      <div
        v-for="group in displayedPackageGroups"
        :key="group.packageName"
        class="cve-group-row"
      >
        <div class="cve-group-package">
          <div class="fw-semibold">{{ group.packageName }}</div>
          <div class="text-secondary small">{{ group.cves.length }} CVE{{ group.cves.length > 1 ? 's' : '' }}</div>
        </div>
        <div class="cve-group-items">
          <div
            v-for="(cve, index) in group.cves"
            :key="`${group.packageName}-${cve.id}-${index}`"
            class="d-flex align-items-center gap-2"
          >
            <CVEBadge :cve="cve" :showIcon="true" />
            <span :class="severityClass(cve.severity)" class="badge">{{ normalizeSeverity(cve.severity) }}</span>
            <span v-if="cve.cvss_score" class="text-secondary small">CVSS {{ cve.cvss_score.toFixed(1) }}</span>
          </div>
        </div>
      </div>

      <button
        v-if="packageGroups.length > limit && !showAll"
        @click="showAll = true"
        class="btn btn-sm btn-link p-0 mt-2"
      >
        +{{ packageGroups.length - limit }} paquet{{ packageGroups.length - limit > 1 ? 's' : '' }}...
      </button>
    </div>
    
    <button 
      v-if="!alwaysExpanded"
      @click="collapsed = !collapsed"
      class="btn btn-sm btn-link p-0 mt-1"
    >
      {{ collapsed ? `Afficher ${cves.length} CVE${cves.length > 1 ? 's' : ''}` : 'Masquer' }}
    </button>
  </div>
  <div v-else class="text-secondary small">
    Aucune CVE détectée
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import CVEBadge from './CVEBadge.vue'

const props = defineProps({
  cveList: {
    type: [String, Array],
    required: true
  },
  showMaxSeverity: {
    type: Boolean,
    default: true
  },
  alwaysExpanded: {
    type: Boolean,
    default: false
  },
  limit: {
    type: Number,
    default: 5
  }
})

const collapsed = ref(!props.alwaysExpanded)
const showAll = ref(false)

const cves = computed(() => {
  try {
    if (Array.isArray(props.cveList)) {
      return props.cveList
    }
    if (typeof props.cveList === 'string') {
      return JSON.parse(props.cveList)
    }
    return []
  } catch (e) {
    console.error('Failed to parse CVE list:', e)
    return []
  }
})

const packageGroups = computed(() => {
  const grouped = new Map()

  for (const cve of cves.value) {
    const packageName = String(cve?.package || '').trim() || 'Paquet non specifie'
    if (!grouped.has(packageName)) {
      grouped.set(packageName, [])
    }
    grouped.get(packageName).push(cve)
  }

  return Array.from(grouped.entries()).map(([packageName, groupedCves]) => ({
    packageName,
    cves: groupedCves,
  }))
})

const displayedPackageGroups = computed(() => {
  if (showAll.value || props.alwaysExpanded) {
    return packageGroups.value
  }
  return packageGroups.value.slice(0, props.limit)
})

const severityOrder = {
  'CRITICAL': 5,
  'HIGH': 4,
  'MEDIUM': 3,
  'LOW': 2,
  'NEGLIGIBLE': 1,
  'UNKNOWN': 0
}

const maxSeverity = computed(() => {
  if (cves.value.length === 0) return 'NONE'
  
  let max = 'UNKNOWN'
  let maxValue = 0
  
  for (const cve of cves.value) {
    const severity = cve.severity?.toUpperCase() || 'UNKNOWN'
    const value = severityOrder[severity] || 0
    if (value > maxValue) {
      maxValue = value
      max = severity
    }
  }
  
  return max
})

const maxSeverityClass = computed(() => {
  return severityClass(maxSeverity.value)
})

function normalizeSeverity(severity) {
  return severity?.toUpperCase() || 'UNKNOWN'
}

function severityClass(severity) {
  const normalized = normalizeSeverity(severity)
  const classes = {
    'CRITICAL': 'bg-red-lt text-red',
    'HIGH': 'bg-orange-lt text-orange',
    'MEDIUM': 'bg-yellow-lt text-yellow',
    'LOW': 'bg-blue-lt text-blue',
    'NEGLIGIBLE': 'bg-secondary-lt text-secondary',
    'UNKNOWN': 'bg-secondary-lt text-secondary'
  }
  return classes[normalized] || classes.UNKNOWN
}
</script>

<style scoped>
.cve-groups {
  display: grid;
  gap: 0.5rem;
}

.cve-group-row {
  display: grid;
  grid-template-columns: minmax(140px, 220px) 1fr;
  border: 1px solid var(--tblr-border-color, #e6e7e9);
  border-radius: 0.5rem;
  overflow: hidden;
}

.cve-group-package {
  background: var(--tblr-bg-surface-secondary, #f8fafc);
  padding: 0.625rem 0.75rem;
  border-right: 1px solid var(--tblr-border-color, #e6e7e9);
}

.cve-group-items {
  padding: 0.625rem 0.75rem;
  display: grid;
  gap: 0.35rem;
}

@media (max-width: 768px) {
  .cve-group-row {
    grid-template-columns: 1fr;
  }

  .cve-group-package {
    border-right: 0;
    border-bottom: 1px solid var(--tblr-border-color, #e6e7e9);
  }
}
</style>



<template>
  <div
    v-if="cves.length > 0"
    class="my-2"
  >
    <div
      v-if="showMaxSeverity"
      class="d-flex align-items-center mb-2"
    >
      <span class="fw-semibold me-2">Criticité max:</span>
      <span
        :class="maxSeverityClass"
        class="badge"
      >
        {{ maxSeverity }}
      </span>
      <span class="text-secondary small ms-2">({{ cves.length }} CVE{{ cves.length > 1 ? 's' : '' }})</span>
    </div>

    <div class="text-secondary small mb-2">
      {{ cveGroups.length }} CVE • {{ impactedPackageCount }} paquet{{ impactedPackageCount > 1 ? 's' : '' }} impacté{{ impactedPackageCount > 1 ? 's' : '' }}
    </div>
    
    <div
      v-if="!collapsed || alwaysExpanded"
      class="cve-groups"
    >
      <div
        v-for="group in displayedCveGroups"
        :key="group.id"
        class="cve-group-row"
      >
        <div class="cve-group-package">
          <div class="fw-semibold">
            {{ group.id }}
          </div>
          <div class="text-secondary small">
            {{ group.packages.length }} paquet{{ group.packages.length > 1 ? 's' : '' }} impacté{{ group.packages.length > 1 ? 's' : '' }}
          </div>
        </div>
        <div class="cve-group-items">
          <div class="cve-group-meta">
            <CVEBadge
              :cve="group"
              :show-icon="true"
            />
            <span
              :class="severityClass(group.severity)"
              class="badge"
            >{{ normalizeSeverity(group.severity) }}</span>
            <span
              v-if="group.cvss_score"
              class="text-secondary small"
            >CVSS {{ group.cvss_score.toFixed(1) }}</span>
          </div>
          <div class="cve-group-packages text-secondary small">
            <span>Paquets:</span>
            <div class="cve-package-chips">
              <span
                v-for="pkg in group.packages"
                :key="`${group.id}-${pkg}`"
                class="badge bg-secondary-lt text-secondary"
              >{{ pkg }}</span>
            </div>
          </div>
        </div>
      </div>

      <button
        v-if="cveGroups.length > limit && !showAll"
        class="btn btn-sm btn-link p-0 mt-2"
        @click="showAll = true"
      >
        Afficher plus
      </button>
    </div>
    
    <button 
      v-if="!alwaysExpanded"
      class="btn btn-sm btn-link p-0 mt-1"
      @click="collapsed = !collapsed"
    >
      {{ collapsed ? `Afficher ${cves.length} CVE${cves.length > 1 ? 's' : ''}` : 'Masquer' }}
    </button>
  </div>
  <div
    v-else
    class="text-secondary small"
  >
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
  },
  initiallyCollapsed: {
    type: Boolean,
    default: null
  }
})

const collapsed = ref(props.initiallyCollapsed ?? !props.alwaysExpanded)
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

const severityOrder = {
  'CRITICAL': 5,
  'HIGH': 4,
  'MEDIUM': 3,
  'LOW': 2,
  'NEGLIGIBLE': 1,
  'UNKNOWN': 0
}

const cveGroups = computed(() => {
  const grouped = new Map()

  for (const cve of cves.value) {
    const cveId = String(cve?.id || '').trim() || 'CVE-UNKNOWN'
    const packageName = String(cve?.package || '').trim() || 'Paquet non specifie'
    if (!grouped.has(cveId)) {
      grouped.set(cveId, {
        id: cveId,
        severity: cve?.severity || 'UNKNOWN',
        cvss_score: Number(cve?.cvss_score || 0),
        packages: new Set(),
      })
    }
    const entry = grouped.get(cveId)
    entry.packages.add(packageName)

    const currentRank = severityOrder[String(entry.severity || '').toUpperCase()] || 0
    const nextRank = severityOrder[String(cve?.severity || '').toUpperCase()] || 0
    if (nextRank > currentRank) {
      entry.severity = cve?.severity || entry.severity
    }

    const nextScore = Number(cve?.cvss_score || 0)
    if (nextScore > Number(entry.cvss_score || 0)) {
      entry.cvss_score = nextScore
    }
  }

  return Array.from(grouped.values())
    .map((group) => ({
      ...group,
      packages: Array.from(group.packages).sort((a, b) => a.localeCompare(b)),
    }))
    .sort((a, b) => {
      const rankA = severityOrder[String(a.severity || '').toUpperCase()] || 0
      const rankB = severityOrder[String(b.severity || '').toUpperCase()] || 0
      if (rankA !== rankB) return rankB - rankA
      const scoreA = Number(a.cvss_score || 0)
      const scoreB = Number(b.cvss_score || 0)
      if (scoreA !== scoreB) return scoreB - scoreA
      return String(a.id).localeCompare(String(b.id))
    })
})

const displayedCveGroups = computed(() => {
  if (showAll.value || props.alwaysExpanded) {
    return cveGroups.value
  }
  return cveGroups.value.slice(0, props.limit)
})

const impactedPackageCount = computed(() => {
  const uniquePackages = new Set()
  for (const group of cveGroups.value) {
    for (const pkg of group.packages) uniquePackages.add(pkg)
  }
  return uniquePackages.size
})

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
  align-items: stretch;
  border: 1px solid var(--tblr-border-color, #e6e7e9);
  border-radius: 0.5rem;
  overflow: hidden;
}

.cve-group-package {
  background: var(--tblr-bg-surface-secondary, #f8fafc);
  padding: 0.625rem 0.75rem;
  border-right: 1px solid var(--tblr-border-color, #e6e7e9);
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.cve-group-items {
  padding: 0.625rem 0.75rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.35rem;
}

.cve-group-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.cve-group-packages {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
}

.cve-package-chips {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
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



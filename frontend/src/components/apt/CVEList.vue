<template>
  <div
    v-if="cves.length > 0"
    class="my-2"
  >
    <div
      v-if="showMaxSeverity"
      class="d-flex align-items-center mb-2"
    >
      <span class="fw-semibold me-2">{{ t('apt.maxSeverityLabel') }}</span>
      <span
        :class="maxSeverityClass"
        class="badge"
      >
        {{ maxSeverity }}
      </span>
      <span class="text-secondary small ms-2">{{ t('apt.cveCountParenWithPlural', { count: cves.length }, cves.length) }}</span>
    </div>

    <div class="text-secondary small mb-2">
      {{ t('apt.cveCountLabel', { count: cveGroups.length }) }} • {{ t('apt.impactedPackagesLabel', { count: impactedPackageCount }, impactedPackageCount) }}
    </div>
    
    <div class="cve-groups">
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
            {{ t('apt.impactedPackagesLabel', { count: group.packages.length }, group.packages.length) }}
          </div>
        </div>
        <div class="cve-group-items">
          <div class="cve-group-meta">
            <CVEBadge
              :cve="group"
              :show-icon="true"
            />
            <span
              :class="cveSeverityClass(group.severity)"
              class="badge"
            >{{ normalizeCveSeverity(group.severity) }}</span>
            <span
              v-if="group.cvss_score"
              class="text-secondary small"
            >CVSS {{ group.cvss_score.toFixed(1) }}</span>
          </div>
          <div class="cve-group-packages text-secondary small">
            <span>{{ t('apt.packagesLabel') }}</span>
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
    </div>

    <button
      v-if="cveGroups.length > limit && !alwaysExpanded"
      type="button"
      class="btn btn-link btn-sm p-0 small text-secondary mt-1"
      @click="showAll = !showAll"
    >
      {{ showAll ? t('apt.collapse') : t('apt.seeAllWithCount', { count: cveGroups.length }) }}
    </button>
  </div>
  <div
    v-else
    class="text-secondary small"
  >
    {{ t('apt.noCveDetected') }}
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CVEBadge from './CVEBadge.vue'
import { cveSeverityClass, cveSeverityOrder, normalizeCveSeverity } from '../../utils/cveSeverity'

interface CVE {
  id?: string
  severity?: string
  package?: string
  cvss_score?: number
}

interface CVEGroup {
  id: string
  severity: string
  cvss_score: number
  packages: string[]
}

const props = withDefaults(defineProps<{
  cveList: string | CVE[]
  showMaxSeverity?: boolean
  alwaysExpanded?: boolean
  limit?: number
  initiallyCollapsed?: boolean | null
}>(), {
  showMaxSeverity: true,
  alwaysExpanded: false,
  limit: 5,
  initiallyCollapsed: null,
})

const { t } = useI18n()

const showAll = ref(false)

const cves = computed<CVE[]>(() => {
  try {
    if (Array.isArray(props.cveList)) {
      return props.cveList
    }
    if (typeof props.cveList === 'string') {
      return JSON.parse(props.cveList) as CVE[]
    }
    return []
  } catch (e) {
    console.error('Failed to parse CVE list:', e)
    return []
  }
})

interface CVEGroupInternal {
  id: string
  severity: string
  cvss_score: number
  packages: Set<string>
}

const cveGroups = computed<CVEGroup[]>(() => {
  const grouped = new Map<string, CVEGroupInternal>()

  for (const cve of cves.value) {
    const cveId = String(cve?.id || '').trim() || 'CVE-UNKNOWN'
    const packageName = String(cve?.package || '').trim() || t('apt.unspecifiedPackage')
    if (!grouped.has(cveId)) {
      grouped.set(cveId, {
        id: cveId,
        severity: cve?.severity || 'UNKNOWN',
        cvss_score: Number(cve?.cvss_score || 0),
        packages: new Set<string>(),
      })
    }
    const entry = grouped.get(cveId)!
    entry.packages.add(packageName)

    const currentRank = cveSeverityOrder(entry.severity)
    const nextRank = cveSeverityOrder(cve?.severity)
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
      const rankA = cveSeverityOrder(a.severity)
      const rankB = cveSeverityOrder(b.severity)
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
  const uniquePackages = new Set<string>()
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
    const severity = normalizeCveSeverity(cve.severity)
    const value = cveSeverityOrder(severity)
    if (value > maxValue) {
      maxValue = value
      max = severity
    }
  }

  return max
})

const maxSeverityClass = computed(() => cveSeverityClass(maxSeverity.value))
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
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.5rem;
  overflow: hidden;
}

.cve-group-package {
  background: var(--tblr-bg-surface-secondary);
  padding: 0.625rem 0.75rem;
  border-right: 1px solid var(--tblr-border-color);
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
    border-bottom: 1px solid var(--tblr-border-color);
  }
}
</style>



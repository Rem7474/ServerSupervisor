<template>
  <div v-if="cves.length > 0" class="cve-container">
    <div v-if="showMaxSeverity" class="d-flex align-items-center mb-2">
      <span class="fw-semibold me-2">Criticité max:</span>
      <span :class="maxSeverityClass" class="badge">
        {{ maxSeverity }}
      </span>
      <span class="text-secondary small ms-2">({{ cves.length }} CVE{{ cves.length > 1 ? 's' : '' }})</span>
    </div>
    
    <div v-if="!collapsed || alwaysExpanded" class="cve-list">
      <CVEBadge 
        v-for="(cve, index) in displayedCves" 
        :key="`${cve.id}-${index}`"
        :cve="cve"
        :showIcon="true"
      />
      <button 
        v-if="cves.length > limit && !showAll" 
        @click="showAll = true"
        class="btn btn-sm btn-link p-0 ms-1"
      >
        +{{ cves.length - limit }} plus...
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

const displayedCves = computed(() => {
  if (showAll.value || props.alwaysExpanded) {
    return cves.value
  }
  return cves.value.slice(0, props.limit)
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
  const severity = maxSeverity.value
  
  const classes = {
    'CRITICAL': 'bg-red text-white',
    'HIGH': 'bg-orange text-white',
    'MEDIUM': 'bg-yellow text-dark',
    'LOW': 'bg-blue-lt text-blue',
    'NEGLIGIBLE': 'bg-secondary-lt text-secondary',
    'UNKNOWN': 'bg-secondary-lt text-secondary'
  }
  
  return classes[severity] || classes['UNKNOWN']
})
</script>

<style scoped>
.cve-container {
  margin: 0.5rem 0;
}

.cve-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.bg-orange {
  background-color: #fd7e14 !important;
}
</style>

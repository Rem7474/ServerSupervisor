<template>
  <a 
    :href="cveUrl" 
    target="_blank" 
    rel="noopener noreferrer"
    :class="badgeClass"
    class="badge text-decoration-none me-1 mb-1"
    :title="`${cve.id} - ${cve.severity} - Package: ${cve.package}`"
  >
    {{ cve.id }}
    <svg v-if="showIcon" xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler ms-1" width="14" height="14" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
      <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
      <path d="M12 6h-6a2 2 0 0 0 -2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-6" />
      <path d="M11 13l9 -9" />
      <path d="M15 4h5v5" />
    </svg>
  </a>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  cve: {
    type: Object,
    required: true,
    validator: (value) => {
      return value.id && value.severity
    }
  },
  showIcon: {
    type: Boolean,
    default: true
  }
})

const cveUrl = computed(() => {
  // Link to Ubuntu CVE database
  if (props.cve.id === 'SECURITY-UPDATE') {
    return '#'
  }
  return `https://ubuntu.com/security/${props.cve.id}`
})

const badgeClass = computed(() => {
  const severity = props.cve.severity?.toUpperCase() || 'UNKNOWN'
  
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
.badge {
  font-size: 0.75rem;
  padding: 0.25rem 0.5rem;
  cursor: pointer;
  transition: all 0.2s;
  display: inline-flex;
  align-items: center;
}

.badge:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0,0,0,0.2);
  opacity: 0.9;
}

.bg-orange {
  background-color: #fd7e14 !important;
}
</style>

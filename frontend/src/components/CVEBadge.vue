<template>
  <a 
    :href="cveUrl" 
    target="_blank" 
    rel="noopener noreferrer"
    :class="badgeClass"
    class="badge text-decoration-none me-1 mb-1"
    :title="cveTitle"
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
    'CRITICAL': 'bg-red-lt text-red',
    'HIGH': 'bg-orange-lt text-orange',
    'MEDIUM': 'bg-yellow-lt text-yellow',
    'LOW': 'bg-blue-lt text-blue',
    'NEGLIGIBLE': 'bg-secondary-lt text-secondary',
    'UNKNOWN': 'bg-secondary-lt text-secondary'
  }
  
  return classes[severity] || classes['UNKNOWN']
})

const cveTitle = computed(() => {
  const packageName = String(props.cve.package || '').trim() || 'N/A'
  return `${props.cve.id} - ${props.cve.severity} - Package: ${packageName}`
})
</script>



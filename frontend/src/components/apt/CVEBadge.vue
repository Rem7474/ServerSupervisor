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
    <IconExternalLink
      v-if="showIcon"
      :size="14"
      class="icon icon-tabler ms-1"
    />
  </a>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IconExternalLink } from '@tabler/icons-vue'
import { cveSeverityClass } from '../../utils/cveSeverity'

interface CVE {
  id: string
  severity?: string
  package?: string
  ubuntu_priority?: string
  cvss_score?: number
}

const props = withDefaults(defineProps<{
  cve: CVE
  showIcon?: boolean
}>(), {
  showIcon: true,
})

const cveUrl = computed(() => {
  // Link to Ubuntu CVE database
  if (props.cve.id === 'SECURITY-UPDATE') {
    return '#'
  }
  return `https://ubuntu.com/security/${props.cve.id}`
})

const badgeClass = computed(() => cveSeverityClass(props.cve.severity))

const cveTitle = computed(() => {
  const packageName = String(props.cve.package || '').trim() || 'N/A'
  const parts = [`${props.cve.id}`, `Package: ${packageName}`]
  if (props.cve.ubuntu_priority) parts.push(`Ubuntu priority: ${props.cve.ubuntu_priority}`)
  if (props.cve.cvss_score) parts.push(`CVSS: ${props.cve.cvss_score.toFixed(1)}`)
  return parts.join(' — ')
})
</script>



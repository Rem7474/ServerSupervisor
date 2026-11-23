import { computed, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProbeForm } from './useUptimeProbes'

// Label and placeholder of a probe's "target" field, which change meaning with
// the probe type (URL for http, host for icmp, host:port otherwise).
export function useProbeTargetHints(probeForm: Ref<ProbeForm>) {
  const { t } = useI18n()
  const probeTargetLabel = computed(() => {
    if (probeForm.value.type === 'http') return t('monitoring.probeTargetLabelUrl')
    if (probeForm.value.type === 'icmp') return t('monitoring.probeTargetLabelHost')
    return t('monitoring.probeTargetLabelHostPort')
  })
  const probeTargetPlaceholder = computed(() => {
    if (probeForm.value.type === 'http') return t('monitoring.probeTargetPlaceholderUrl')
    if (probeForm.value.type === 'icmp') return t('monitoring.probeTargetPlaceholderHost')
    return t('monitoring.probeTargetPlaceholderHostPort')
  })
  return { probeTargetLabel, probeTargetPlaceholder }
}

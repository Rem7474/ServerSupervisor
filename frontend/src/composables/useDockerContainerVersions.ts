import { computed, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import type { Container, VersionComparison } from '../types/dockerContainersTab'

type VersionComparisonRef = Ref<VersionComparison[]>

/**
 * Version-tracking state of the containers table: which comparison row belongs
 * to a container, whether its tracker can be triggered, and the trigger itself.
 */
export function useDockerContainerVersions(
  versionComparisons: VersionComparisonRef,
  canRunDocker: () => boolean,
) {
  const { t } = useI18n()

  // Two kinds of rows land here: ambient ones (one per host+image+tag group,
  // carrying image_tag) and tracker ones (aggregated across tags, image_tag
  // empty). Indexing both under distinct keys keeps the exact-tag ambient row
  // preferred while the tracker row stays reachable as the fallback — the server
  // never emits both for the same container group.
  const versionMap = computed<Record<string, VersionComparison>>(() => {
    const m: Record<string, VersionComparison> = {}
    for (const vc of versionComparisons.value) {
      if (vc.image_tag) m[`${vc.host_id}|${vc.docker_image}|${vc.image_tag}`] = vc
      else m[`${vc.host_id}|${vc.docker_image}`] = vc
    }
    return m
  })

  function containerVersion(c: Container): VersionComparison | null {
    return versionMap.value[`${c.host_id}|${c.image}|${c.image_tag || 'latest'}`] ||
           versionMap.value[`${c.host_id}|${c.image}`] ||
           versionMap.value[`${c.host_id}|${c.image}:${c.image_tag}`] ||
           null
  }

  const trackerRunLoading = ref<Record<string, boolean>>({})
  const trackerFeedback = ref('')
  const trackerFeedbackIsError = ref(false)

  // The ambient engine records why it couldn't classify an image (private
  // registry with no matching credential, registry unreachable, not swept yet) —
  // surface it rather than leaving "Version inconnue" unexplained.
  function unknownVersionTitle(vc: VersionComparison | null | undefined): string {
    return vc?.last_error || t('docker.unknownVersionTitle')
  }

  function canRunTracker(vc: VersionComparison | null | undefined): boolean {
    return canRunDocker() && !!vc?.tracker_id
  }

  function hasManualTrackerData(vc: VersionComparison | null | undefined): boolean {
    return !!(vc?.latest_version && String(vc.latest_version).trim())
  }

  function isTrackerRunDisabled(vc: VersionComparison | null | undefined): boolean {
    if (!vc) return true
    if (!canRunTracker(vc)) return true
    if (!hasManualTrackerData(vc)) return true
    return !!trackerRunLoading.value[vc.tracker_id!]
  }

  function trackerRunTooltip(vc: VersionComparison | null | undefined): string {
    if (!canRunDocker()) return t('docker.adminOperatorOnly')
    if (!hasManualTrackerData(vc)) return t('docker.waitFirstCheck')
    return t('docker.triggerTrackerNow')
  }

  async function runTracker(vc: VersionComparison | null | undefined, container?: Container): Promise<void> {
    if (!vc || isTrackerRunDisabled(vc)) return
    const id = vc.tracker_id!
    trackerRunLoading.value = { ...trackerRunLoading.value, [id]: true }
    trackerFeedback.value = ''
    trackerFeedbackIsError.value = false
    try {
      await apiClient.runReleaseTracker(id)
      trackerFeedback.value = t('docker.triggerLaunchedFor', { name: container?.image || t('docker.trackerFallbackName') })
    } catch (e: unknown) {
      trackerFeedback.value = getApiErrorMessage(e, t('docker.triggerFailed'))
      trackerFeedbackIsError.value = true
    } finally {
      const next = { ...trackerRunLoading.value }
      delete next[id]
      trackerRunLoading.value = next
    }
  }

  return {
    containerVersion,
    trackerRunLoading,
    trackerFeedback,
    trackerFeedbackIsError,
    unknownVersionTitle,
    isTrackerRunDisabled,
    trackerRunTooltip,
    runTracker,
  }
}

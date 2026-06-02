import { computed, ref, Ref } from 'vue'
import { defineStore } from 'pinia'

interface HostSummary {
  id: string
  name?: string
  hostname?: string
  ip_address?: string
  os?: string
  status?: string
  last_seen?: string | number | Date | null
  agent_version?: string
}

interface VersionComparison {
  docker_image?: string
  host_id?: string
  hostname?: string
  container_count?: number
  custom_task_id?: string
  tracker_id?: string
  is_up_to_date?: boolean
  running_version?: string
  latest_version?: string
  release_url?: string
  update_confirmed?: boolean
}

interface ProxmoxSummary {
  connection_count?: number
  node_count?: number
  nodes_down?: number
  vm_count?: number
  lxc_count?: number
  storage_used?: number
  storage_total?: number
  recent_failed_tasks?: number
  storage_near_full?: number
  storage_offline?: number
}

export const useDashboardStore = defineStore('dashboard', () => {
  const hosts: Ref<HostSummary[]> = ref([])
  const aptPending: Ref<number> = ref(0)
  const versionComparisons: Ref<VersionComparison[]> = ref([])
  const proxmoxSummary: Ref<ProxmoxSummary | null> = ref(null)

  const hasProxmox = computed(() => (proxmoxSummary.value?.connection_count ?? 0) > 0)
  const onlineCount = computed(() => hosts.value.filter((h) => h.status === 'online').length)
  const offlineCount = computed(() => hosts.value.filter((h) => h.status !== 'online').length)
  const outdatedDockerImages = computed(() =>
    versionComparisons.value.filter((v) => !v.is_up_to_date && (v.running_version || v.update_confirmed)).length
  )
  const outdatedVersions = computed(() => outdatedDockerImages.value + aptPending.value)
  const proxmoxStoragePct = computed(() => {
    const s = proxmoxSummary.value
    if (!s || !s.storage_total) return 0
    return (s.storage_used! / s.storage_total) * 100
  })

  function setHosts(nextHosts: HostSummary[]): void {
    hosts.value = nextHosts
  }

  function setAptPending(nextAptPending: number): void {
    aptPending.value = nextAptPending
  }

  function setVersionComparisons(nextVersionComparisons: VersionComparison[]): void {
    versionComparisons.value = nextVersionComparisons
  }

  function setProxmoxSummary(nextProxmoxSummary: ProxmoxSummary | null): void {
    proxmoxSummary.value = nextProxmoxSummary
  }

  return {
    hosts,
    aptPending,
    versionComparisons,
    proxmoxSummary,
    hasProxmox,
    onlineCount,
    offlineCount,
    outdatedDockerImages,
    outdatedVersions,
    proxmoxStoragePct,
    setHosts,
    setAptPending,
    setVersionComparisons,
    setProxmoxSummary,
  }
})
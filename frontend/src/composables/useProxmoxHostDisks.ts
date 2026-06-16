import { ref, toValue, type MaybeRef } from 'vue'
import api from '../api'

export interface ProxmoxDisk {
  id: string
  node_name: string
  dev_path: string
  model?: string
  serial?: string
  size_bytes: number
  disk_type?: string
  health: string
  wearout: number
}

export function useProxmoxHostDisks(hostId: MaybeRef<string>) {
  const disks = ref<ProxmoxDisk[]>([])
  const loading = ref(true)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const res = await api.getHostProxmoxDisks(toValue(hostId))
      disks.value = res.data || []
    } catch (err) {
      console.error('Failed to load Proxmox node disks:', err)
    } finally {
      loading.value = false
    }
  }

  return { disks, loading, load }
}

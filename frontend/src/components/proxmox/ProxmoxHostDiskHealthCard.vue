<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        État SMART des disques
        <span
          v-if="nodeName"
          class="text-muted fw-normal ms-1"
        >· nœud Proxmox {{ nodeName }}</span>
      </h3>
    </div>
    <div
      v-if="loading"
      class="card-body text-center py-4"
    >
      <LoadingSkeleton
        variant="card"
        :lines="3"
      />
    </div>
    <div
      v-else-if="disks.length === 0"
      class="card-body text-center text-muted py-5"
    >
      <div class="small fw-medium">
        Aucune donnée disque côté nœud Proxmox
      </div>
      <div class="mt-1 opacity-75 small">
        Le poller Proxmox n'a pas encore remonté de disque pour ce nœud (rôle PVEAuditor requis).
      </div>
    </div>
    <div
      v-else
      class="card-body"
    >
      <div class="mb-3 small text-muted">
        Cet hôte ne peut pas lire le SMART localement (conteneur/VM). Santé remontée par le nœud Proxmox qui l'héberge.
      </div>
      <div class="d-flex flex-column gap-3">
        <div
          v-for="disk in disks"
          :key="disk.id"
          class="border rounded-3 p-3 shadow-sm"
          :class="getCardClass(disk.health)"
        >
          <div class="d-flex flex-wrap align-items-start justify-content-between gap-2">
            <div class="min-w-0">
              <div class="fw-semibold text-truncate">
                {{ disk.dev_path }}
                <span
                  v-if="disk.disk_type"
                  class="badge bg-secondary-lt text-uppercase ms-2"
                >{{ disk.disk_type }}</span>
              </div>
              <div class="text-muted small text-truncate">
                {{ disk.model }}
                <span
                  v-if="disk.serial"
                  class="ms-2"
                >{{ disk.serial }}</span>
              </div>
            </div>
            <BadgePill
              :tone="getStatusBadgeClass(disk.health)"
              :text="disk.health"
              compact
            />
          </div>

          <div class="row mt-3 g-3 text-sm">
            <div class="col-6">
              <div class="text-muted small">
                Taille
              </div>
              <div class="fw-bold">
                {{ formatBytes(disk.size_bytes) }}
              </div>
            </div>
            <div class="col-6">
              <div class="text-muted small">
                Durée de vie restante
              </div>
              <div
                class="fw-bold"
                :class="{ 'text-danger': disk.wearout >= 0 && disk.wearout <= 20, 'text-warning': disk.wearout > 20 && disk.wearout <= 50 }"
              >
                <span v-if="disk.wearout >= 0">{{ disk.wearout }}%</span>
                <span
                  v-else
                  class="text-muted"
                >N/A</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import apiClient from '../../api'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import BadgePill from '../common/BadgePill.vue'

interface ProxmoxDisk {
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

const props = defineProps<{
  hostId: string
  nodeName?: string | null
}>()

const disks = ref<ProxmoxDisk[]>([])
const loading = ref(true)

onMounted(loadDisks)

async function loadDisks(): Promise<void> {
  try {
    loading.value = true
    const res = await apiClient.getHostProxmoxDisks(props.hostId)
    disks.value = res.data || []
  } catch (err) {
    console.error('Failed to load Proxmox node disks:', err)
  } finally {
    loading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return 'N/A'
  const units = ['o', 'Ko', 'Mo', 'Go', 'To', 'Po']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(value >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

type Tone = 'success' | 'danger' | 'warning' | 'secondary'

function getStatusBadgeClass(status: string): Tone {
  switch (status) {
    case 'PASSED': return 'success'
    case 'FAILED': return 'danger'
    case 'UNKNOWN': return 'warning'
    default: return 'secondary'
  }
}

function getCardClass(status: string): string {
  switch (status) {
    case 'FAILED': return 'bg-danger-lt border-danger'
    case 'UNKNOWN': return 'bg-warning-lt border-warning'
    case 'PASSED': return 'bg-success-lt border-success'
    default: return 'bg-secondary-lt border-secondary'
  }
}
</script>

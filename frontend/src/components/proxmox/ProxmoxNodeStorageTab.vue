<template>
  <div class="table-responsive">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>Stockage</th>
          <th>Type</th>
          <th>Total</th>
          <th>Utilisé</th>
          <th>Disponible</th>
          <th>Utilisation</th>
          <th>Partagé</th>
          <th>Statut</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!storages.length">
          <td
            colspan="8"
            class="text-center text-muted py-4"
          >
            Aucun stockage sur ce nœud.
          </td>
        </tr>
        <tr
          v-for="s in storages"
          :key="s.id"
        >
          <td class="fw-medium">
            {{ s.storage_name }}
          </td>
          <td><span class="badge bg-secondary-lt text-secondary">{{ s.storage_type }}</span></td>
          <td>{{ formatBytes(s.total) }}</td>
          <td>{{ formatBytes(s.used) }}</td>
          <td>{{ formatBytes(s.avail) }}</td>
          <td>
            <div class="d-flex align-items-center gap-2">
              <div class="progress progress-xs flex-grow-1 proxmox-progress-min-80">
                <div
                  class="progress-bar"
                  :class="storageColor(s.used, s.total)"
                  :style="`width:${storagePct(s)}%`"
                />
              </div>
              <span class="text-muted small">{{ storagePct(s) }}%</span>
            </div>
          </td>
          <td>
            <span
              v-if="s.shared"
              class="badge bg-azure-lt text-azure"
            >Oui</span>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
          <td>
            <span
              v-if="s.active && s.enabled"
              class="badge bg-success-lt text-success"
            >Actif</span>
            <span
              v-else
              class="badge bg-danger-lt text-danger"
            >Inactif</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
type Storage = Record<string, any>

defineProps<{ storages: Storage[] }>()

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function storagePct(s: Storage): string {
  if (!s.total) return '0'
  return ((s.used / s.total) * 100).toFixed(1)
}

function storageColor(used: number, total: number): string {
  if (!total) return 'bg-secondary'
  const pct = used / total
  if (pct > 0.85) return 'bg-danger'
  if (pct > 0.6) return 'bg-warning'
  return 'bg-primary'
}
</script>

<style scoped>
.proxmox-progress-min-80 {
  min-width: 80px;
}
</style>

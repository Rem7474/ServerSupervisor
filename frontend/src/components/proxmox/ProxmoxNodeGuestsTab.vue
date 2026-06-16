<template>
  <div class="table-responsive">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>
            <SortableHeader
              :label="idLabel"
              :active="sortKey === 'vmid'"
              :direction="sortDir"
              @toggle="toggleSort('vmid')"
            />
          </th>
          <th>
            <SortableHeader
              label="Nom"
              :active="sortKey === 'name'"
              :direction="sortDir"
              @toggle="toggleSort('name')"
            />
          </th>
          <th>
            <SortableHeader
              label="Statut"
              :active="sortKey === 'status'"
              :direction="sortDir"
              @toggle="toggleSort('status')"
            />
          </th>
          <th>
            <SortableHeader
              label="IP"
              :active="sortKey === 'ip'"
              :direction="sortDir"
              @toggle="toggleSort('ip')"
            />
          </th>
          <th>
            <SortableHeader
              label="CPU alloué"
              :active="sortKey === 'cpu_alloc'"
              :direction="sortDir"
              @toggle="toggleSort('cpu_alloc')"
            />
          </th>
          <th>
            <SortableHeader
              label="CPU utilisé"
              :active="sortKey === 'cpu_used'"
              :direction="sortDir"
              @toggle="toggleSort('cpu_used')"
            />
          </th>
          <th>
            <SortableHeader
              label="RAM allouée"
              :active="sortKey === 'mem_alloc'"
              :direction="sortDir"
              @toggle="toggleSort('mem_alloc')"
            />
          </th>
          <th>
            <SortableHeader
              label="RAM utilisée"
              :active="sortKey === 'mem_used'"
              :direction="sortDir"
              @toggle="toggleSort('mem_used')"
            />
          </th>
          <th>
            <SortableHeader
              label="Disque"
              :active="sortKey === 'disk_alloc'"
              :direction="sortDir"
              @toggle="toggleSort('disk_alloc')"
            />
          </th>
          <th>
            <SortableHeader
              label="Uptime"
              :active="sortKey === 'uptime'"
              :direction="sortDir"
              @toggle="toggleSort('uptime')"
            />
          </th>
          <th v-if="showTags">
            <SortableHeader
              label="Tags"
              :active="sortKey === 'tags'"
              :direction="sortDir"
              @toggle="toggleSort('tags')"
            />
          </th>
          <th>
            <SortableHeader
              label="Hôte lié"
              :active="sortKey === 'linked_host'"
              :direction="sortDir"
              @toggle="toggleSort('linked_host')"
            />
          </th>
          <th v-if="showMigrate" />
        </tr>
      </thead>
      <tbody>
        <tr v-if="sortedGuests.length === 0">
          <td
            :colspan="colspan"
            class="text-center text-muted py-4"
          >
            {{ emptyText }}
          </td>
        </tr>
        <tr
          v-for="g in sortedGuests"
          :key="g.id"
        >
          <td class="text-muted">
            {{ g.vmid }}
          </td>
          <td class="fw-medium">
            <router-link
              :to="`/proxmox/guests/${g.id}?nodeId=${nodeId}`"
              class="text-decoration-none"
            >
              {{ g.name || '—' }}
            </router-link>
          </td>
          <td><span :class="guestStatusClass(g.status)">{{ g.status }}</span></td>
          <td>
            <span
              v-if="guestNetworksLoading"
              class="text-muted small"
            >…</span>
            <template v-else-if="guestNetworks[g.vmid]?.length">
              <div
                v-for="iface in guestNetworks[g.vmid]"
                :key="iface.name"
                class="small lh-sm"
              >
                <span class="text-muted me-1">{{ iface.name }}</span>
                <span
                  v-for="ip in iface.ips.filter((i: string) => !i.startsWith('fe80'))"
                  :key="ip"
                >{{ ip.split('/')[0] }}</span>
              </div>
            </template>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
          <td>{{ g.cpu_alloc }}{{ cpuSuffix }}</td>
          <td>{{ (g.cpu_usage * 100).toFixed(1) }}%</td>
          <td>{{ formatBytes(g.mem_alloc) }}</td>
          <td>{{ formatBytes(g.mem_usage) }}</td>
          <td>{{ formatBytes(g.disk_alloc) }}</td>
          <td>{{ g.status === 'running' ? formatUptime(g.uptime) : '—' }}</td>
          <td v-if="showTags">
            <template v-if="g.tags">
              <span
                v-for="tag in g.tags.split(';').filter(Boolean)"
                :key="tag"
                class="badge bg-blue-lt text-blue me-1"
              >{{ tag.trim() }}</span>
            </template>
          </td>
          <td>
            <GuestLinkCell
              :link="linkForGuest(g)"
              @confirm="emit('confirm-link', g)"
              @ignore="emit('ignore-link', g)"
              @go="emit('go-host', linkForGuest(g))"
            />
          </td>
          <td v-if="showMigrate">
            <button
              v-if="peerNodes.length > 0"
              type="button"
              class="btn btn-sm btn-ghost-secondary"
              title="Migrer vers un autre nœud"
              @click="emit('migrate', g)"
            >
              Migrer
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import SortableHeader from '../common/SortableHeader.vue'
import GuestLinkCell from './GuestLinkCell.vue'

type Guest = Record<string, any>
type LinkMap = Record<string, any>

const props = defineProps<{
  kind: 'vm' | 'lxc'
  guests: Guest[]
  guestNetworks: Record<string, any[]>
  guestNetworksLoading?: boolean
  links: LinkMap
  peerNodes: Guest[]
  nodeId: string
}>()

const emit = defineEmits<{
  (e: 'confirm-link', guest: Guest): void
  (e: 'ignore-link', guest: Guest): void
  (e: 'go-host', link: any): void
  (e: 'migrate', guest: Guest): void
}>()

const showTags = computed(() => props.kind === 'vm')
const showMigrate = computed(() => props.kind === 'vm')
const idLabel = computed(() => (props.kind === 'vm' ? 'VMID' : 'CT ID'))
const cpuSuffix = computed(() => (props.kind === 'vm' ? ' vCPU' : ''))
const emptyText = computed(() => (props.kind === 'vm' ? 'Aucune VM sur ce nœud.' : 'Aucun conteneur LXC sur ce nœud.'))
const colspan = computed(() => (props.kind === 'vm' ? 13 : 11))

const sortKey = ref('vmid')
const sortDir = ref<'asc' | 'desc'>('asc')

function toggleSort(key: string) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

function compareValues(a: unknown, b: unknown, direction: 'asc' | 'desc' = 'asc'): number {
  const dir = direction === 'asc' ? 1 : -1
  if (a == null && b == null) return 0
  if (a == null) return 1 * dir
  if (b == null) return -1 * dir
  if (typeof a === 'string' || typeof b === 'string') {
    return String(a).localeCompare(String(b), 'fr', { sensitivity: 'base' }) * dir
  }
  if (a < b) return -1 * dir
  if (a > b) return 1 * dir
  return 0
}

function linkForGuest(g: Guest) {
  return props.links[g.id] ?? null
}

function guestPrimaryIp(guest: Guest): string {
  const ifaces = props.guestNetworks?.[guest.vmid]
  if (!Array.isArray(ifaces)) return ''
  for (const iface of ifaces) {
    const ips = Array.isArray(iface?.ips) ? iface.ips : []
    const first = ips.find((ip: string) => typeof ip === 'string' && !ip.startsWith('fe80'))
    if (first) return first.split('/')[0]
  }
  return ''
}

function linkedHostLabel(guest: Guest): string {
  const link = linkForGuest(guest)
  if (!link) return ''
  return link.host_hostname || link.host_name || ''
}

const sortedGuests = computed(() => {
  const list = [...(props.guests ?? [])]
  list.sort((a, b) => {
    switch (sortKey.value) {
      case 'vmid': return compareValues(a.vmid, b.vmid, sortDir.value)
      case 'name': return compareValues(a.name || '', b.name || '', sortDir.value)
      case 'status': return compareValues(a.status || '', b.status || '', sortDir.value)
      case 'ip': return compareValues(guestPrimaryIp(a), guestPrimaryIp(b), sortDir.value)
      case 'cpu_alloc': return compareValues(a.cpu_alloc, b.cpu_alloc, sortDir.value)
      case 'cpu_used': return compareValues(a.cpu_usage, b.cpu_usage, sortDir.value)
      case 'mem_alloc': return compareValues(a.mem_alloc, b.mem_alloc, sortDir.value)
      case 'mem_used': return compareValues(a.mem_usage, b.mem_usage, sortDir.value)
      case 'disk_alloc': return compareValues(a.disk_alloc, b.disk_alloc, sortDir.value)
      case 'uptime': return compareValues(a.status === 'running' ? a.uptime : -1, b.status === 'running' ? b.uptime : -1, sortDir.value)
      case 'tags': return compareValues(a.tags || '', b.tags || '', sortDir.value)
      case 'linked_host': return compareValues(linkedHostLabel(a), linkedHostLabel(b), sortDir.value)
      default: return 0
    }
  })
  return list
})

function guestStatusClass(status: string): string {
  const map: Record<string, string> = {
    running: 'badge bg-green-lt text-green',
    stopped: 'badge bg-secondary-lt text-secondary',
    paused: 'badge bg-yellow-lt text-yellow',
  }
  return map[status] ?? 'badge bg-secondary-lt text-secondary'
}

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

function formatUptime(seconds: number): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}j ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}
</script>

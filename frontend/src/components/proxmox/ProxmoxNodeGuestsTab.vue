<template>
  <div class="table-responsive scroll-table">
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
              label="Domaines"
              :active="sortKey === 'domains'"
              :direction="sortDir"
              @toggle="toggleSort('domains')"
            />
          </th>
          <th>
            <SortableHeader
              label="CPU"
              :active="sortKey === 'cpu_used'"
              :direction="sortDir"
              @toggle="toggleSort('cpu_used')"
            />
          </th>
          <th>
            <SortableHeader
              label="RAM"
              :active="sortKey === 'mem_used'"
              :direction="sortDir"
              @toggle="toggleSort('mem_used')"
            />
          </th>
          <th>
            <SortableHeader
              label="Disque"
              :active="sortKey === 'disk_used'"
              :direction="sortDir"
              @toggle="toggleSort('disk_used')"
            />
          </th>
          <th v-if="showActionsCol" />
        </tr>
      </thead>
      <tbody>
        <tr v-if="sortedGuests.length === 0">
          <td :colspan="colspan">
            <EmptyState :title="emptyText" />
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
          <td><span :class="getEntityStateClass(g.status)">{{ getEntityStateLabel(g.status) }}</span></td>
          <td>
            <span
              v-if="guestNetworksLoading"
              class="text-muted small"
            >…</span>
            <span
              v-else-if="guestPrimaryIp(g)"
              class="small"
            >{{ guestPrimaryIp(g) }}</span>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
          <td>
            <span
              v-if="guestExposureLoading"
              class="text-muted small"
            >…</span>
            <router-link
              v-else-if="guestDomains(g).length"
              :to="`/proxmox/guests/${g.id}?nodeId=${nodeId}`"
              class="text-decoration-none"
              :title="guestDomains(g).join(', ')"
            >
              <span
                v-for="name in guestDomains(g).slice(0, 2)"
                :key="name"
                class="badge bg-azure-lt text-azure me-1 font-monospace"
              >{{ name }}</span>
              <span
                v-if="guestDomains(g).length > 2"
                class="badge bg-secondary-lt text-secondary"
              >+{{ guestDomains(g).length - 2 }}</span>
            </router-link>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
          <td>
            <span :class="getMetricColorClass(g.cpu_usage * 100)">{{ (g.cpu_usage * 100).toFixed(1) }}%</span>
          </td>
          <td>
            <span
              v-if="ramPct(g) != null"
              :class="getMetricColorClass(ramPct(g))"
            >{{ ramPct(g)!.toFixed(1) }}%</span>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
          <td>
            <span
              v-if="diskPct(g) != null"
              :class="getMetricColorClass(diskPct(g))"
            >{{ diskPct(g)!.toFixed(1) }}%</span>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
          <td v-if="showActionsCol">
            <div class="d-flex align-items-center gap-1">
              <template v-if="auth.isAdmin">
                <button
                  v-if="g.status === 'stopped'"
                  type="button"
                  class="btn btn-sm btn-icon btn-ghost-success"
                  title="Démarrer"
                  :disabled="actionLoadingFor(g) !== null"
                  @click="emit('guest-action', g, 'start')"
                >
                  <span
                    v-if="actionLoadingFor(g) === 'start'"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconPlayerPlay
                    v-else
                    :size="16"
                  />
                </button>
                <template v-else>
                  <button
                    type="button"
                    class="btn btn-sm btn-icon btn-ghost-warning"
                    title="Redémarrer"
                    :disabled="actionLoadingFor(g) !== null"
                    @click="emit('guest-action', g, 'reboot')"
                  >
                    <span
                      v-if="actionLoadingFor(g) === 'reboot'"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconRefresh
                      v-else
                      :size="16"
                    />
                  </button>
                  <button
                    type="button"
                    class="btn btn-sm btn-icon btn-ghost-danger"
                    title="Arrêter"
                    :disabled="actionLoadingFor(g) !== null"
                    @click="emit('guest-action', g, 'shutdown')"
                  >
                    <span
                      v-if="actionLoadingFor(g) === 'shutdown'"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerStop
                      v-else
                      :size="16"
                    />
                  </button>
                </template>
              </template>
              <button
                v-if="showMigrate && peerNodes.length > 0"
                type="button"
                class="btn btn-sm btn-ghost-secondary"
                title="Migrer vers un autre nœud"
                @click="emit('migrate', g)"
              >
                Migrer
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IconPlayerPlay, IconPlayerStop, IconRefresh } from '@tabler/icons-vue'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import { useAuthStore } from '../../stores/auth'
import type { GuestPowerAction } from '../../composables/useProxmoxGuestActions'
import { compareValues } from '../../utils/sort'
import { getEntityStateClass, getEntityStateLabel } from '../../utils/statusClasses'
import { getMetricColorClass } from '../../utils/metricColor'

type Guest = Record<string, any>

const props = defineProps<{
  kind: 'vm' | 'lxc'
  guests: Guest[]
  guestNetworks: Record<string, any[]>
  guestNetworksLoading?: boolean
  guestExposure?: Record<string, any>
  guestExposureLoading?: boolean
  peerNodes: Guest[]
  nodeId: string
  actionLoading?: Record<string, GuestPowerAction | undefined>
}>()

const emit = defineEmits<{
  (e: 'migrate', guest: Guest): void
  (e: 'guest-action', guest: Guest, action: GuestPowerAction): void
}>()

const auth = useAuthStore()

const showMigrate = computed(() => props.kind === 'vm')
// Migrate is open to any authenticated user (PVE-token-scoped, see the
// Proxmox integration note in root CLAUDE.md), so the VM tab already shows
// this column regardless of role. Power actions are admin-only, so the LXC
// tab only gains the column for admins — it never had a migrate button.
const showActionsCol = computed(() => showMigrate.value || auth.isAdmin)
const idLabel = computed(() => (props.kind === 'vm' ? 'VMID' : 'CT ID'))
const emptyText = computed(() => (props.kind === 'vm' ? 'Aucune VM sur ce nœud.' : 'Aucun conteneur LXC sur ce nœud.'))
const colspan = computed(() => (showActionsCol.value ? 9 : 8))

function actionLoadingFor(guest: Guest): GuestPowerAction | null {
  return props.actionLoading?.[guest.id] ?? null
}

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

// Only the ethX interface(s) are shown here — full interface/IP detail
// (including non-eth interfaces) lives on the guest's own detail page.
function guestPrimaryIp(guest: Guest): string {
  const ifaces = props.guestNetworks?.[guest.vmid]
  if (!Array.isArray(ifaces)) return ''
  const ethIfaces = ifaces.filter((iface) => /^eth\d+$/i.test(iface?.name ?? ''))
  for (const iface of ethIfaces) {
    const ips = Array.isArray(iface?.ips) ? iface.ips : []
    const first = ips.find((ip: string) => typeof ip === 'string' && !ip.startsWith('fe80'))
    if (first) return first.split('/')[0]
  }
  return ''
}

function ramPct(guest: Guest): number | null {
  if (!guest.mem_alloc) return null
  return ((guest.mem_usage ?? 0) / guest.mem_alloc) * 100
}

function diskPct(guest: Guest): number | null {
  if (!guest.disk_alloc) return null
  return ((guest.disk_usage ?? 0) / guest.disk_alloc) * 100
}

// Flattens the guest's NPM exposure (map keyed by vmid, see useProxmoxNode's
// guestExposure) into a flat, deduped domain-name list for display/sort. One
// exposure.domains entry is already one domain name (see HostExposedDomain).
function guestDomains(guest: Guest): string[] {
  const exposure = props.guestExposure?.[String(guest.vmid)]
  const domains = exposure?.domains
  if (!Array.isArray(domains)) return []
  const names = new Set<string>()
  for (const d of domains) {
    if (d?.domain_name) names.add(d.domain_name)
  }
  return [...names]
}

const sortedGuests = computed(() => {
  const list = [...(props.guests ?? [])]
  list.sort((a, b) => {
    switch (sortKey.value) {
      case 'vmid': return compareValues(a.vmid, b.vmid, sortDir.value)
      case 'name': return compareValues(a.name || '', b.name || '', sortDir.value)
      case 'status': return compareValues(a.status || '', b.status || '', sortDir.value)
      case 'ip': return compareValues(guestPrimaryIp(a), guestPrimaryIp(b), sortDir.value)
      case 'domains': return compareValues(guestDomains(a).length, guestDomains(b).length, sortDir.value)
      case 'cpu_used': return compareValues(a.cpu_usage, b.cpu_usage, sortDir.value)
      case 'mem_used': return compareValues(ramPct(a) ?? -1, ramPct(b) ?? -1, sortDir.value)
      case 'disk_used': return compareValues(diskPct(a) ?? -1, diskPct(b) ?? -1, sortDir.value)
      default: return 0
    }
  })
  return list
})
</script>

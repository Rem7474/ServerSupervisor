<template>
  <div>
    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3">
          <div class="col-md-6 col-lg-3">
            <input v-model="search" type="text" class="form-control" placeholder="Rechercher un port, conteneur, image...">
          </div>
          <div class="col-md-6 col-lg-3">
            <select v-model="protocolFilter" class="form-select">
              <option value="">Tous les protocoles</option>
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
            </select>
          </div>
          <div class="col-md-6 col-lg-3">
            <select v-model="hostFilter" class="form-select">
              <option value="">Tous les hôtes</option>
              <option v-for="h in hosts" :key="h.id" :value="h.id">
                {{ h.name || h.hostname || h.id }}
              </option>
            </select>
          </div>
          <div class="col-md-6 col-lg-3">
            <label class="form-check form-switch">
              <input v-model="onlyPublished" class="form-check-input" type="checkbox">
              <span class="form-check-label">Ports publies seulement</span>
            </label>
          </div>
        </div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Hote</th>
              <th>Conteneur</th>
              <th>Image</th>
              <th>Port hote</th>
              <th>Port conteneur</th>
              <th>Proto</th>
              <th>IPv4</th>
              <th>IPv6</th>
              <th>Etat</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in portRows" :key="row.key">
              <td>
                <router-link :to="`/hosts/${row.host_id}`" class="text-decoration-none">
                  {{ row.host_name || row.host_id }}
                </router-link>
              </td>
              <td class="fw-semibold">{{ row.container_name }}</td>
              <td>
                <div>{{ row.image }}</div>
                <div class="text-secondary small"><code>{{ row.image_tag || '-' }}</code></div>
              </td>
              <td class="fw-semibold">{{ row.host_port || '-' }}</td>
              <td class="text-secondary">{{ row.container_port || '-' }}</td>
              <td class="text-secondary text-uppercase">{{ row.protocol || '-' }}</td>
              <td class="text-secondary small font-monospace">
                <span v-if="row.ipv4" class="badge bg-blue-lt text-blue">{{ row.ipv4 }}</span>
                <span v-else class="text-muted">-</span>
              </td>
              <td class="text-secondary small font-monospace">
                <span v-if="row.ipv6" class="badge bg-purple-lt text-purple">{{ row.ipv6 }}</span>
                <span v-else class="text-muted">-</span>
              </td>
              <td>
                <span :class="row.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                  {{ { running: 'En cours', exited: 'Arrete', paused: 'En pause', created: 'Cree', restarting: 'Redemarrage', dead: 'Mort' }[row.state] || row.state || 'inconnu' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="portRows.length === 0" class="text-center text-secondary py-4">
        Aucun port visible
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <h3 class="card-title">Trafic par hote</h3>
        <div class="card-options">
          <span class="badge bg-azure-lt text-azure ms-1">
            {{ hosts.length }} hote{{ hosts.length > 1 ? 's' : '' }}
          </span>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Hote</th>
              <th>IP</th>
              <th class="text-end">↓ Rx</th>
              <th class="text-end">↑ Tx</th>
              <th>Statut</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="h in hosts" :key="h.id">
              <td>
                <router-link :to="`/hosts/${h.id}`" class="fw-semibold text-decoration-none">
                  {{ h.name || h.hostname || h.id }}
                </router-link>
              </td>
              <td class="text-secondary">{{ h.ip_address }}</td>
              <td class="text-end font-monospace small text-info">{{ formatBytes(h.network_rx_bytes || 0) }}</td>
              <td class="text-end font-monospace small text-warning">{{ formatBytes(h.network_tx_bytes || 0) }}</td>
              <td>
                <span :class="h.status === 'online' ? 'badge bg-green-lt text-green' : h.status === 'warning' ? 'badge bg-yellow-lt text-yellow' : 'badge bg-red-lt text-red'">
                  {{ h.status || 'unknown' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="hosts.length === 0" class="text-center text-secondary py-4">
        Aucun hote trouve
      </div>
    </div>

    <div v-if="containersWithNetStats.length" class="card mt-4">
      <div class="card-header">
        <h3 class="card-title">Trafic reseau par conteneur</h3>
        <div class="card-options">
          <span class="badge bg-azure-lt text-azure ms-1">
            {{ containersWithNetStats.length }} conteneur{{ containersWithNetStats.length > 1 ? 's' : '' }}
          </span>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Conteneur</th>
              <th>Hote</th>
              <th class="text-end">↓ Rx</th>
              <th class="text-end">↑ Tx</th>
              <th class="text-end">Total</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containersWithNetStats" :key="c.id">
              <td class="fw-semibold">{{ c.name }}</td>
              <td class="text-secondary">{{ c.hostname }}</td>
              <td class="text-end font-monospace small text-info">{{ formatBytes(c.net_rx_bytes) }}</td>
              <td class="text-end font-monospace small text-warning">{{ formatBytes(c.net_tx_bytes) }}</td>
              <td class="text-end font-monospace small fw-semibold">{{ formatBytes(c.net_rx_bytes + c.net_tx_bytes) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  hosts: {
    type: Array,
    default: () => [],
  },
  containers: {
    type: Array,
    default: () => [],
  },
})

const search = ref('')
const protocolFilter = ref('')
const hostFilter = ref('')
const onlyPublished = ref(true)

const portRows = computed(() => {
  const grouped = new Map()

  for (const container of props.containers) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostPort = Number(mapping.host_port || 0)
      const isPublished = hostPort > 0
      if (onlyPublished.value && !isPublished) continue

      const groupKey = `${container.id}-${hostPort}-${mapping.container_port}-${mapping.protocol}`

      if (!grouped.has(groupKey)) {
        grouped.set(groupKey, {
          key: groupKey,
          host_id: container.host_id,
          host_name: container.hostname,
          container_name: container.name,
          image: container.image,
          image_tag: container.image_tag,
          state: container.state,
          host_port: hostPort,
          container_port: mapping.container_port,
          protocol: mapping.protocol,
          ipv4: null,
          ipv6: null,
        })
      }

      const row = grouped.get(groupKey)
      const ip = mapping.host_ip || ''
      if (ip.includes(':')) {
        row.ipv6 = ip
      } else {
        row.ipv4 = ip || '0.0.0.0'
      }
    }
  }

  const rows = [...grouped.values()]
  const query = search.value.trim().toLowerCase()

  return rows.filter((row) => {
    const matchHost = !hostFilter.value || row.host_id === hostFilter.value
    const matchProto = !protocolFilter.value || row.protocol === protocolFilter.value
    const matchSearch =
      !query ||
      row.container_name?.toLowerCase().includes(query) ||
      row.image?.toLowerCase().includes(query) ||
      row.image_tag?.toLowerCase().includes(query) ||
      row.host_name?.toLowerCase().includes(query) ||
      String(row.host_port || '').includes(query) ||
      String(row.container_port || '').includes(query) ||
      row.protocol?.toLowerCase().includes(query) ||
      (row.ipv4 || '').includes(query) ||
      (row.ipv6 || '').includes(query)

    return matchHost && matchProto && matchSearch
  })
})

const containersWithNetStats = computed(() =>
  [...props.containers]
    .filter((c) => c.state === 'running' && (c.net_rx_bytes > 0 || c.net_tx_bytes > 0))
    .sort((a, b) => (b.net_rx_bytes + b.net_tx_bytes) - (a.net_rx_bytes + a.net_tx_bytes))
)

function formatBytes(bytes) {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(1)} ${units[idx]}`
}
</script>


<template>
  <div
    v-if="show"
    class="traffic-modal-backdrop"
    @click.self="$emit('close')"
  >
    <div class="traffic-modal card shadow-lg">
      <div class="card-header d-flex align-items-center justify-content-between">
        <div>
          <h3 class="card-title mb-0">
            Détails domaine: <span class="font-monospace">{{ domain }}</span>
          </h3>
          <div class="text-secondary small">
            Fenêtre de logs détaillée sur {{ period }}
          </div>
        </div>
        <button
          type="button"
          class="btn btn-sm btn-outline-secondary"
          @click="$emit('close')"
        >
          Fermer
        </button>
      </div>

      <div class="card-body traffic-modal-body">
        <div
          v-if="loading"
          class="text-center py-4 text-secondary"
        >
          <span class="spinner-border spinner-border-sm me-2" />
          Chargement des détails...
        </div>

        <template v-else>
          <div class="row row-cards mb-3">
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 text-center">
                <div class="text-secondary small">
                  Hits
                </div>
                <div class="h3 mb-0">
                  {{ details.hits || 0 }}
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 text-center">
                <div class="text-secondary small">
                  Bytes
                </div>
                <div class="h3 mb-0">
                  {{ formatBytes(details.bytes || 0) }}
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 text-center">
                <div class="text-secondary small">
                  4xx
                </div>
                <div class="h3 mb-0 text-yellow">
                  {{ details.status_4xx || 0 }}
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 text-center">
                <div class="text-secondary small">
                  5xx
                </div>
                <div class="h3 mb-0 text-red">
                  {{ details.status_5xx || 0 }}
                </div>
              </div>
            </div>
          </div>

          <div class="row row-cards mb-3">
            <div class="col-lg-6">
              <div class="card h-100">
                <div class="card-header">
                  <h4 class="card-title mb-0">
                    Top chemins
                  </h4>
                </div>
                <div class="card-body p-0">
                  <div
                    v-if="!(details.top_paths || []).length"
                    class="text-center py-3 text-secondary small"
                  >
                    Aucun chemin
                  </div>
                  <div
                    v-for="p in details.top_paths"
                    v-else
                    :key="p.path"
                    class="d-flex justify-content-between border-bottom px-3 py-2"
                  >
                    <span
                      class="font-monospace small text-truncate me-2"
                      style="max-width: 75%;"
                    >{{ p.path }}</span>
                    <span class="badge bg-azure-lt text-azure">{{ p.hits }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="col-lg-6">
              <div class="card h-100">
                <div class="card-header">
                  <h4 class="card-title mb-0">
                    Top IPs clientes
                  </h4>
                </div>
                <div class="card-body p-0">
                  <div
                    v-if="!(details.top_clients || []).length"
                    class="text-center py-3 text-secondary small"
                  >
                    Aucune IP
                  </div>
                  <div
                    v-for="ip in details.top_clients"
                    v-else
                    :key="ip.ip"
                    class="d-flex justify-content-between border-bottom px-3 py-2"
                  >
                    <span class="font-monospace small">{{ ip.ip }}</span>
                    <span class="badge bg-purple-lt text-purple">{{ ip.hits }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <h4 class="card-title mb-0">
                Logs récents
              </h4>
            </div>
            <div
              class="table-responsive scroll-table"
              style="max-height: 360px;"
            >
              <table class="table table-sm table-vcenter mb-0">
                <thead>
                  <tr>
                    <th>Heure</th>
                    <th>IP</th>
                    <th>Méthode</th>
                    <th>Chemin</th>
                    <th>Status</th>
                    <th>Bytes</th>
                    <th>UA</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!(details.requests || []).length">
                    <td
                      colspan="7"
                      class="text-center text-secondary py-3"
                    >
                      Aucune requête disponible
                    </td>
                  </tr>
                  <tr
                    v-for="(r, idx) in details.requests || []"
                    :key="`${r.timestamp}-${idx}`"
                  >
                    <td class="small">
                      {{ formatDate(r.timestamp) }}
                    </td>
                    <td class="font-monospace small">
                      {{ r.ip }}
                    </td>
                    <td><span class="badge bg-blue-lt text-blue">{{ r.method }}</span></td>
                    <td
                      class="font-monospace small text-truncate domain-path"
                      :title="r.path"
                      style="max-width: 18rem;"
                    >
                      {{ r.path }}
                    </td>
                    <td>
                      <span
                        class="badge"
                        :class="statusClass(r.status)"
                      >{{ r.status }}</span>
                    </td>
                    <td class="small">
                      {{ formatBytes(r.bytes || 0) }}
                    </td>
                    <td
                      class="small text-truncate domain-ua"
                      :title="r.user_agent || '-'"
                      style="max-width: 20rem;"
                    >
                      {{ r.user_agent || '-' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- display-layer shim for the ad-hoc GetDomainDetails aggregate (no Go model)
type AnyRecord = Record<string, any>

withDefaults(defineProps<{
  show: boolean
  domain: string
  loading: boolean
  details?: AnyRecord
  period: string
}>(), {
  details: () => ({}),
})

defineEmits<{
  (e: 'close'): void
}>()

function formatBytes(bytes: number): string {
  const value = Number(bytes) || 0
  if (value < 1024) return `${value} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  return `${size.toFixed(1)} ${units[unit]}`
}

function formatDate(v: string): string {
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v || '-'
  return d.toLocaleString()
}

function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'bg-green-lt text-green'
  if (status >= 300 && status < 400) return 'bg-yellow-lt text-yellow'
  if (status >= 400) return 'bg-red-lt text-red'
  return 'bg-secondary-lt text-secondary'
}
</script>

<style scoped>
.traffic-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1060;
  padding: 1rem;
}

.traffic-modal {
  width: min(1200px, 96vw);
  max-height: 92vh;
  overflow: auto;
}

@media (max-width: 992px) {
  .traffic-modal-backdrop {
    padding: 0;
  }

  .traffic-modal {
    width: 100vw;
    max-height: 100dvh;
    height: 100dvh;
    border-radius: 0;
  }

  .traffic-modal-body {
    padding: 0.75rem;
  }

  .domain-path,
  .domain-ua {
    max-width: 12rem !important;
  }
}
</style>

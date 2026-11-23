<template>
  <div class="card">
    <div class="table-responsive scroll-table">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>
              <SortableHeader
                :label="t('monitoring.nameTargetColumn')"
                :active="sort.col === 'name'"
                :direction="sort.dir"
                @toggle="emit('toggleSort', 'name')"
              />
            </th>
            <th>
              <SortableHeader
                :label="t('monitoring.statusColumn')"
                :active="sort.col === 'status'"
                :direction="sort.dir"
                @toggle="emit('toggleSort', 'status')"
              />
            </th>
            <th>{{ t('monitoring.availabilityColumn') }}</th>
            <th>
              <SortableHeader
                :label="t('monitoring.uptime24hColumn')"
                :active="sort.col === 'uptime'"
                :direction="sort.dir"
                @toggle="emit('toggleSort', 'uptime')"
              />
            </th>
            <th>
              <SortableHeader
                :label="t('monitoring.sslColumn')"
                :active="sort.col === 'ssl_days'"
                :direction="sort.dir"
                @toggle="emit('toggleSort', 'ssl_days')"
              />
            </th>
            <th>
              <SortableHeader
                :label="t('monitoring.lastCheckedColumn')"
                :active="sort.col === 'last_checked'"
                :direction="sort.dir"
                @toggle="emit('toggleSort', 'last_checked')"
              />
            </th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.id"
            :class="{ 'opacity-60': row.probe && !row.probe.enabled }"
          >
            <td>
              <router-link
                :to="rowLink(row)"
                class="fw-semibold text-decoration-none"
              >
                {{ row.name }}
              </router-link>
              <div
                v-if="row.probe"
                class="text-secondary small"
              >
                <code>{{ row.probe.target }}</code>
              </div>
              <div
                v-else-if="row.cert"
                class="text-secondary small"
              >
                <code>{{ row.cert.host }}:{{ row.cert.port }}</code>
              </div>
              <span
                v-if="row.npmProxyHostId"
                class="badge bg-azure-lt text-azure mt-1"
              >NPM</span>
            </td>
            <td>
              <template v-if="row.probe">
                <span :class="['badge', probeBadge(row.probe)]">
                  {{ probeStatusLabel(row.probe) }}
                </span>
                <span
                  v-if="!row.probe.enabled"
                  class="badge bg-secondary-lt text-secondary ms-1"
                >{{ t('monitoring.probeDisabledBadge') }}</span>
              </template>
              <span
                v-else
                class="text-secondary small"
              >—</span>
            </td>
            <td>
              <div
                v-if="row.probe && probeHistory[row.probe.id]?.length"
                class="tracking"
                style="height: 20px; min-width: 110px;"
              >
                <div
                  v-for="tick in probeHistory[row.probe.id]"
                  :key="tick.id"
                  class="flex-fill rounded-1"
                  :class="tick.success ? 'bg-success' : 'bg-danger'"
                  style="height: 100%; min-width: 2px;"
                  :title="`${formatDateTime(tick.checked_at)} — ${tick.success ? t('monitoring.tickOkLabel') : t('monitoring.tickKoLabel')}`"
                />
              </div>
              <span
                v-else
                class="text-secondary small"
              >—</span>
            </td>
            <td>
              <template v-if="row.probe && probeStats[row.probe.id]">
                <span :class="['badge', uptimeBadgeClass(probeStats[row.probe.id].uptime_percent)]">
                  {{ probeStats[row.probe.id].uptime_percent.toFixed(1) }}%
                </span>
              </template>
              <span
                v-else
                class="text-secondary small"
              >—</span>
            </td>
            <td>
              <span
                v-if="row.cert"
                :class="['badge', daysBadge(row.cert.days_remaining)]"
              >
                {{ daysLabel(row.cert.days_remaining) }}
              </span>
              <span
                v-else
                class="text-secondary small"
              >—</span>
            </td>
            <td class="text-secondary small">
              <RelativeTime
                v-if="row.probe?.last_checked_at || row.cert?.last_checked_at"
                :date="(row.probe?.last_checked_at || row.cert?.last_checked_at) as string"
              />
              <span
                v-else
                class="text-secondary"
              >{{ t('monitoring.neverLabel') }}</span>
            </td>
            <td class="text-end">
              <div class="btn-list flex-nowrap justify-content-end">
                <button
                  v-if="isAdmin"
                  type="button"
                  class="btn btn-sm btn-outline-secondary"
                  :disabled="checkingProbeId === row.probe?.id || checkingCertId === row.cert?.id"
                  @click="emit('check', row)"
                >
                  {{ t('monitoring.verifyButton') }}
                </button>
                <div
                  v-if="isAdmin && row.probe"
                  class="btn-group btn-group-sm"
                >
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    :title="t('monitoring.editProbeTooltip')"
                    :aria-label="t('monitoring.editProbeTooltip')"
                    @click="emit('editProbe', row.probe)"
                  >
                    <IconActivity :size="14" />
                  </button>
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    :title="t('monitoring.deleteProbeTooltip')"
                    :aria-label="t('monitoring.deleteProbeTooltip')"
                    @click="emit('deleteProbe', row.probe)"
                  >
                    <IconTrash :size="14" />
                  </button>
                </div>
                <div
                  v-if="isAdmin && row.cert"
                  class="btn-group btn-group-sm"
                >
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    :title="t('monitoring.editCertTooltip')"
                    :aria-label="t('monitoring.editCertTooltip')"
                    @click="emit('editCert', row.cert)"
                  >
                    <IconLock :size="14" />
                  </button>
                  <button
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    :title="t('monitoring.deleteCertTooltip')"
                    :aria-label="t('monitoring.deleteCertTooltip')"
                    @click="emit('deleteCert', row.cert)"
                  >
                    <IconTrash :size="14" />
                  </button>
                </div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-if="totalPages > 1"
      class="card-footer d-flex align-items-center justify-content-between"
    >
      <div class="text-secondary small">
        {{ t('monitoring.paginationRangeLabel', { start: (page - 1) * pageSize + 1, end: Math.min(page * pageSize, filteredCount), total: filteredCount }) }}
      </div>
      <PaginationNav
        :current-page="page"
        :total-pages="totalPages"
        @select="emit('selectPage', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconActivity, IconLock, IconTrash } from '@tabler/icons-vue'
import RelativeTime from '../RelativeTime.vue'
import PaginationNav from '../PaginationNav.vue'
import SortableHeader from '../common/SortableHeader.vue'
import { formatDateTime } from '../../utils/formatters'
import type { MonitoringRow } from '../../composables/useMonitoringOverview'
import type { SSLCertificate } from '../../types/ssl'
import type { UptimeProbe } from '../../types/uptime'

type SortCol = 'name' | 'status' | 'uptime' | 'ssl_days' | 'last_checked'

defineProps<{
  rows: MonitoringRow[]
  sort: { col: string; dir: 'asc' | 'desc' }
  probeStats: Record<string, { uptime_percent: number }>
  probeHistory: Record<string, { id: string | number; checked_at: string; success: boolean }[]>
  checkingProbeId: string | null
  checkingCertId: string | null
  isAdmin: boolean
  page: number
  totalPages: number
  pageSize: number
  filteredCount: number
  probeBadge: (p: UptimeProbe) => string
  probeStatusLabel: (p: UptimeProbe) => string
  uptimeBadgeClass: (pct: number) => string
  daysLabel: (d: number | null | undefined) => string
  daysBadge: (d: number | null | undefined) => string
}>()

const emit = defineEmits<{
  (e: 'toggleSort', col: SortCol): void
  (e: 'selectPage', page: number): void
  (e: 'check', row: MonitoringRow): void
  (e: 'editProbe', probe: UptimeProbe): void
  (e: 'deleteProbe', probe: UptimeProbe): void
  (e: 'editCert', cert: SSLCertificate): void
  (e: 'deleteCert', cert: SSLCertificate): void
}>()

const { t } = useI18n()

function rowLink(row: MonitoringRow): string {
  if (row.npmProxyHostId) return `/monitoring/host/${row.npmProxyHostId}`
  if (row.probe) return `/monitoring/probes/${row.probe.id}`
  return `/monitoring/ssl/${row.cert!.id}`
}
</script>

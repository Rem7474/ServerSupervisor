<template>
  <div
    v-if="job.status === 'failed'"
    class="alert alert-danger mb-0"
    role="alert"
  >
    {{ job.error === 'cancelled' ? t('discovery.jobCancelled') : t('discovery.jobFailed', { error: job.error || '—' }) }}
  </div>

  <!-- ARP table -->
  <template v-else-if="job.arp">
    <EmptyState
      v-if="!job.arp.entries.length"
      :title="t('discovery.arpEmptyTitle')"
      :subtitle="t('discovery.arpEmptySubtitle')"
    />
    <template v-else>
      <div class="text-secondary small mb-2">
        {{ t('discovery.arpSummary', { count: job.arp.entries.length }, job.arp.entries.length) }}
        <span v-if="job.arp.truncated"> — {{ t('discovery.arpTruncated') }}</span>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table table-sm">
          <thead>
            <tr>
              <th>{{ t('discovery.colIp') }}</th>
              <th>{{ t('discovery.colMac') }}</th>
              <th>{{ t('discovery.colVendor') }}</th>
              <th>{{ t('discovery.colInventory') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="e in job.arp.entries"
              :key="`${e.ip}|${e.interface}`"
            >
              <td>
                <div class="font-monospace">
                  {{ e.ip }}
                </div>
                <div class="text-secondary small">
                  {{ e.interface }}
                </div>
              </td>
              <td class="font-monospace text-nowrap">
                {{ e.mac }}
              </td>
              <td>
                <span v-if="e.vendor_hint">{{ e.vendor_hint }}</span>
                <span
                  v-else-if="e.locally_administered"
                  class="text-secondary"
                  :title="t('discovery.localMacHint')"
                >{{ t('discovery.localMac') }}</span>
                <span
                  v-else
                  class="text-secondary"
                >—</span>
              </td>
              <td>
                <RegisteredBadge
                  :registered="e.already_registered"
                  :name="e.existing_host_name"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </template>

  <!-- Port scan -->
  <template v-else-if="job.port_scan">
    <div
      v-if="job.port_scan.cancelled || job.port_scan.timed_out"
      class="alert alert-warning"
      role="status"
    >
      {{ job.port_scan.cancelled ? t('discovery.scanPartialCancelled', { done: job.port_scan.probes_done, total: job.port_scan.probes_total }) : t('discovery.scanPartialTimedOut', { done: job.port_scan.probes_done, total: job.port_scan.probes_total }) }}
    </div>
    <div class="text-secondary small mb-2">
      {{ t('discovery.scanSummary', { hosts: job.port_scan.hosts.length, addresses: job.port_scan.hosts_scanned, ports: job.port_scan.ports_scanned, duration: formatDurationSecs(Math.round(job.port_scan.duration_ms / 1000)) }) }}
    </div>
    <EmptyState
      v-if="!job.port_scan.hosts.length"
      :title="t('discovery.scanEmptyTitle')"
      :subtitle="t('discovery.scanEmptySubtitle')"
    />
    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table table-sm">
        <thead>
          <tr>
            <th>{{ t('discovery.colIp') }}</th>
            <th>{{ t('discovery.colOpenPorts') }}</th>
            <th>{{ t('discovery.colInventory') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in job.port_scan.hosts"
            :key="row.ip"
          >
            <td class="font-monospace">
              {{ row.ip }}
            </td>
            <td>
              <div class="d-flex flex-wrap gap-1">
                <span
                  v-for="p in row.open_ports"
                  :key="p"
                  class="badge bg-blue-lt font-monospace"
                >{{ p }}</span>
              </div>
            </td>
            <td>
              <RegisteredBadge
                :registered="row.already_registered"
                :name="row.existing_host_name"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </template>

  <!-- SNMP -->
  <template v-else-if="job.snmp">
    <dl class="row mb-3 discovery-snmp-system">
      <template
        v-for="row in snmpRows"
        :key="row.label"
      >
        <dt class="col-sm-3 text-secondary fw-normal">
          {{ row.label }}
        </dt>
        <dd
          class="col-sm-9"
          :class="{ 'font-monospace': row.mono }"
        >
          {{ row.value || '—' }}
        </dd>
      </template>
    </dl>
    <div
      v-if="job.snmp.interfaces_error"
      class="alert alert-warning"
      role="status"
    >
      {{ t('discovery.snmpInterfacesError', { error: job.snmp.interfaces_error }) }}
    </div>
    <template v-if="job.snmp.interfaces?.length">
      <div class="text-secondary small mb-2">
        {{ t('discovery.snmpInterfacesCount', { count: job.snmp.interfaces.length }, job.snmp.interfaces.length) }}
        <span v-if="job.snmp.interfaces_truncated"> — {{ t('discovery.snmpInterfacesTruncated') }}</span>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table table-sm">
          <thead>
            <tr>
              <th>#</th>
              <th>{{ t('discovery.colInterface') }}</th>
              <th>{{ t('discovery.colSpeed') }}</th>
              <th>{{ t('discovery.colStatus') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="i in job.snmp.interfaces"
              :key="i.index"
            >
              <td>{{ i.index }}</td>
              <td>
                <div>{{ i.descr }}</div>
                <div
                  v-if="i.phys_address"
                  class="text-secondary small font-monospace"
                >
                  {{ i.phys_address }}
                </div>
              </td>
              <td class="text-nowrap">
                {{ formatBitRate(i.speed_bps) }}
              </td>
              <td class="text-nowrap">
                <span
                  class="badge"
                  :class="i.oper_status === 'up' ? 'bg-success-lt' : 'bg-secondary-lt'"
                >{{ t('discovery.ifStatus', { oper: i.oper_status, admin: i.admin_status }) }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </template>
</template>

<script setup lang="ts">
import { computed, defineComponent, h } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'
import { formatBitRate, formatDurationSecs, formatUptime } from '../../utils/formatters'
import type { DiscoveryJob } from '../../types/discovery'

const props = defineProps<{ job: DiscoveryJob }>()
const { t } = useI18n()

const RegisteredBadge = defineComponent({
  props: { registered: Boolean, name: { type: String, default: '' } },
  setup(p) {
    return () =>
      p.registered
        ? h('span', { class: 'badge bg-azure-lt' }, p.name ? t('discovery.registeredAs', { name: p.name }) : t('discovery.registered'))
        : h('span', { class: 'badge bg-success-lt' }, t('discovery.newAddress'))
  },
})

const snmpRows = computed(() => {
  const s = props.job.snmp
  if (!s) return []
  return [
    { label: t('discovery.sysName'), value: s.sys_name },
    { label: t('discovery.sysDescr'), value: s.sys_descr },
    { label: t('discovery.sysObjectId'), value: s.sys_object_id, mono: true },
    { label: t('discovery.sysUptime'), value: formatUptime(s.sys_uptime_seconds, '') },
    { label: t('discovery.sysContact'), value: s.sys_contact },
    { label: t('discovery.sysLocation'), value: s.sys_location },
    { label: t('discovery.snmpVersion'), value: `v${s.version} — ${s.target}`, mono: true },
  ]
})
</script>

<style scoped>
.discovery-snmp-system dd {
  overflow-wrap: anywhere;
}
</style>

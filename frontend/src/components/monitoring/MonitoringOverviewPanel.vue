<template>
  <div>
    <MonitoringKpiStrip
      :down-count="downCount"
      :expiring-count="expiringCount"
      :total-monitored="totalMonitored"
    />

    <PageRefreshBar
      v-model="autoRefresh"
      :label="t('monitoring.pageTitle')"
      :interval-sec="REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <DataToolbar
      searchable
      :search="search"
      :search-placeholder="t('monitoring.searchHostPlaceholder')"
      @update:search="search = $event"
    />

    <div
      v-if="loading && !pagedRows.length"
      class="row row-cards"
    >
      <div class="col-12">
        <LoadingSkeleton
          variant="table"
          :lines="5"
        />
      </div>
    </div>

    <EmptyState
      v-else-if="!pagedRows.length"
      :title="search ? t('monitoring.noResultsTitle') : t('monitoring.noProbesTitle')"
      :subtitle="search ? t('monitoring.noResultsSubtitle') : t('monitoring.noProbesSubtitle')"
      :cta-label="!search && auth.role === 'admin' ? t('monitoring.newTrackerButton') : ''"
      @cta="openCreateProbe"
    />

    <MonitoringRowsTable
      v-else
      :rows="pagedRows"
      :sort="rowSort"
      :probe-stats="probeStats"
      :probe-history="probeHistory"
      :checking-probe-id="checkingProbeId"
      :checking-cert-id="checkingCertId"
      :is-admin="auth.role === 'admin'"
      :page="rowPage"
      :total-pages="rowTotalPages"
      :page-size="PAGE_SIZE"
      :filtered-count="filteredCount"
      :probe-badge="probeBadge"
      :probe-status-label="probeStatusLabel"
      :uptime-badge-class="uptimeBadgeClass"
      :days-label="daysLabel"
      :days-badge="daysBadge"
      @toggle-sort="toggleRowSort"
      @select-page="setRowPage"
      @check="checkRowNow"
      @edit-probe="openEditProbe"
      @delete-probe="confirmDeleteProbe"
      @edit-cert="openEditCert"
      @delete-cert="confirmDeleteCert"
    />

    <MonitoringTrackerModal
      v-model:probe-form="probeForm"
      v-model:cert-form="certForm"
      :probe-modal-open="probeModalOpen"
      :cert-modal-open="certModalOpen"
      :saving-probe="savingProbe"
      :saving-cert="savingCert"
      :probe-form-error="probeFormError"
      :cert-form-error="certFormError"
      :is-probe-modal-open="() => probeModalOpen"
      :is-cert-modal-open="() => certModalOpen"
      :save-probe="saveProbe"
      :save-cert="saveCert"
      :close-probe-modal="closeProbeModal"
      :close-cert-modal="closeCertModal"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../../stores/auth'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PageRefreshBar from '../PageRefreshBar.vue'
import DataToolbar from '../common/DataToolbar.vue'
import MonitoringKpiStrip from './MonitoringKpiStrip.vue'
import MonitoringRowsTable from './MonitoringRowsTable.vue'
import MonitoringTrackerModal from './MonitoringTrackerModal.vue'
import { useMonitoringOverview } from '../../composables/useMonitoringOverview'

const auth = useAuthStore()
const { t } = useI18n()

const {
  PAGE_SIZE,
  loading,
  error,
  downCount,
  expiringCount,
  totalMonitored,
  filteredCount,
  search,
  rowSort,
  toggleRowSort,
  pagedRows,
  rowPage,
  rowTotalPages,
  setRowPage,
  checkingProbeId,
  checkingCertId,
  checkRowNow,
  probeStats,
  probeHistory,
  probeBadge,
  probeStatusLabel,
  uptimeBadgeClass,
  daysLabel,
  daysBadge,
  autoRefresh,
  lastUpdatedAt,
  REFRESH_SEC,
  probeModalOpen,
  savingProbe,
  probeFormError,
  probeForm,
  openCreateProbe,
  openEditProbe,
  closeProbeModal,
  saveProbe,
  confirmDeleteProbe,
  certModalOpen,
  savingCert,
  certFormError,
  certForm,
  openEditCert,
  closeCertModal,
  saveCert,
  confirmDeleteCert,
} = useMonitoringOverview()

// Only openCreateProbe is called from outside (MonitoringView's single
// "Nouveau suivi" button) — the cert-only entry point was removed when the
// two header buttons were merged into one (the modal's own toggle covers
// it), don't re-expose openCreateCert here without a real caller again.
defineExpose({ openCreateProbe })
</script>

<template>
  <div>
    <div class="page-header d-print-none mb-4">
      <div class="row g-2 align-items-center">
        <div class="col">
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              Dashboard
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>{{ TAB_TITLES[alertsTab] || 'Alertes' }}</span>
          </div>
          <h2 class="page-title">
            {{ TAB_TITLES[alertsTab] || 'Alertes' }}
          </h2>
        </div>
        <div class="col-auto ms-auto d-flex gap-2">
          <button
            v-if="alertsTab === 'rules' && auth.isAdmin"
            type="button"
            class="btn btn-primary btn-sm"
            @click="startAddAlert"
          >
            <IconPlus
              :size="14"
              class="icon me-1"
            />
            Nouvelle alerte
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="fetchError"
      class="alert alert-danger mb-3"
    >
      <IconAlertTriangle class="icon alert-icon me-2" />
      Erreur de chargement des règles : {{ fetchError }}
    </div>

    <EntityTabShell
      :model-value="alertsTab"
      :tabs="alertsTabs"
      nav-margin-class="mb-4"
      @update:model-value="onTabClick"
    >
      <template #rules>
        <AlertRuleList
          :rules="(rules as any)"
          :hosts="(hosts as any)"
          :loading="loading"
          :fetched="fetched"
          :error="saveError"
          :is-admin="auth.isAdmin"
          :format-date="(formatDate as any)"
          @add="startAddAlert"
          @edit="(startEditAlert as any)"
          @toggle="(onToggleEnabled as any)"
          @delete="(deleteAlert as any)"
        />
      </template>

      <template #releases>
        <AlertReleaseSummary
          :trackers="(trackers as any)"
          :loading="trackersLoading"
          :error="trackersError"
        />
      </template>

      <template #incidents>
        <AlertIncidentList
          :incidents="(incidents as any)"
          :loading="incidentsLoading"
          :error="incidentsError"
          :active-incident-count="activeIncidentCount"
          :is-admin="auth.isAdmin"
          :initial-search="hostFilterFromQuery"
          :marking-read="markingRead"
          :resolving-id="resolvingId"
          @mark-all-read="markAllRead"
          @resolve="resolveIncident"
        />
      </template>
    </EntityTabShell>

    <ErrorBoundary title="Erreur lors du chargement du formulaire de règle d'alerte">
      <AlertRuleModal
        :visible="showModal"
        :rule="(editingRule as any)"
        :hosts="(hosts as any)"
        :capabilities="(capabilities as any)"
        :capabilities-loading="capabilitiesLoading"
        :capabilities-error="capabilitiesError"
        :saving="saving"
        :error="saveError"
        @close="closeModal"
        @submit="(saveAlert as any)"
      />
    </ErrorBoundary>
  </div>
</template>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AlertIncidentList from '../components/alerts/AlertIncidentList.vue'
import AlertReleaseSummary from '../components/alerts/AlertReleaseSummary.vue'
import AlertRuleList from '../components/alerts/AlertRuleList.vue'
import AlertRuleModal from '../components/alerts/AlertRuleModal.vue'
import ErrorBoundary from '../components/common/ErrorBoundary.vue'
import EntityTabShell from '../components/EntityTabShell.vue'
import type { EntityTab } from '../components/EntityTabShell.vue'
import { IconAlertTriangle, IconPlus } from '@tabler/icons-vue'
import { useAlertsPage } from '../composables/useAlertsPage'
import { useNotificationHistory } from '../composables/useNotificationHistory'
import { onNotificationsMessage } from '../composables/useNotifications'
import { useAuthStore } from '../stores/auth'
import type { AlertRule } from '../types/alert'

const auth = useAuthStore()

const TAB_TITLES: Record<string, string> = {
  rules: 'Alertes',
  releases: 'Suivi de versions',
  incidents: 'Historique de notifications',
}

const route = useRoute()
const router = useRouter()
const {
  alertsTab,
  trackers,
  trackersLoading,
  trackersError,
  rules,
  hosts,
  loading,
  fetched,
  fetchError,
  showModal,
  saving,
  saveError,
  editingRule,
  capabilities,
  capabilitiesLoading,
  capabilitiesError,
  init,
  switchToTrackers,
  startAddAlert,
  startEditAlert,
  saveAlert,
  toggleEnabled,
  deleteAlert,
  closeModal,
  formatDate,
  onWebSocketAlert,
} = useAlertsPage()

const {
  incidents,
  loading: incidentsLoading,
  error: incidentsError,
  loaded: incidentsLoaded,
  activeIncidentCount,
  loadIncidents,
  markingRead,
  markAllRead,
  resolvingId,
  resolveIncident,
  onWebSocketAlert: onNotificationHistoryWSAlert,
} = useNotificationHistory()

async function switchToIncidents(): Promise<void> {
  alertsTab.value = 'incidents'
  if (!incidentsLoaded.value) await loadIncidents()
}

// Disabling a rule can auto-resolve its active incidents server-side — mirror
// that in the incidents tab without coupling useAlertsPage.ts to
// useNotificationHistory.ts (see useAlertsPage.ts's toggleEnabled comment).
async function onToggleEnabled(rule: AlertRule): Promise<void> {
  const nextEnabled = !rule.enabled
  await toggleEnabled(rule)
  if (!nextEnabled) await loadIncidents()
}

// `?host=` (set by HostDetailView's incident deep links) seeds the incidents
// tab's search box so arriving from a specific host lands pre-filtered
// instead of on the full fleet-wide list. Read once at mount, not reactively,
// so clearing the search box afterwards doesn't get stomped back by the
// still-present query param.
const hostFilterFromQuery = typeof route.query.host === 'string' ? route.query.host : ''

const alertsTabs = computed<EntityTab[]>(() => [
  {
    key: 'rules',
    label: 'Règles',
    badges: [{ value: rules.value.length, badgeClass: 'badge bg-azure-lt text-azure ms-1' }],
    lazy: true,
  },
  {
    key: 'releases',
    label: 'Suivi de versions',
    badges: trackers.value.length > 0 ? [{ value: trackers.value.length, badgeClass: 'badge bg-azure-lt text-azure ms-1' }] : [],
    lazy: true,
  },
  {
    key: 'incidents',
    label: 'Historique notifications',
    badges: activeIncidentCount.value > 0 ? [{ value: activeIncidentCount.value, badgeClass: 'badge bg-danger-lt text-danger ms-1' }] : [],
    lazy: true,
  },
])

// 'releases'/'incidents' each own the actual tab switch (alertsTab.value=...)
// as part of their lazy-load-on-first-visit logic; 'rules' has no such
// loader, so it's a plain assignment.
function onTabClick(key: string): void {
  if (key === 'releases') { switchToTrackers(); return }
  if (key === 'incidents') { switchToIncidents(); return }
  alertsTab.value = key
}

let incidentsPollTimer: ReturnType<typeof setInterval> | null = null

watch(alertsTab, (tab) => {
  router.replace({ query: { ...route.query, tab } })
})

onMounted(async () => {
  await init()

  // Default landing tab is the active-incidents triage view, not rule
  // configuration — an ops opening /alerts wants to see what's actually
  // firing right now. `?tab=rules`/`?tab=releases` (used by deep links, e.g.
  // the command palette's alert-rule search results) are honored explicitly.
  if (route.query.tab === 'rules') {
    // stays on the 'rules' default from useAlertsPage()
  } else if (route.query.tab === 'releases') {
    await switchToTrackers()
  } else {
    await switchToIncidents()
  }

  incidentsPollTimer = setInterval(loadIncidents, 300_000)
})

// Shares the single app-wide notifications WebSocket connection (owned by
// NotificationBell/useNotifications) instead of opening a second connection
// to the same route — this view only needs the raw messages to refresh its
// own trackers state; useNotificationHistory() subscribes independently
// below for incidents (onNotificationsMessage supports multiple listeners).
const unsubscribeNotifications = onNotificationsMessage(onWebSocketAlert)
const unsubscribeIncidentsNotifications = onNotificationsMessage(onNotificationHistoryWSAlert)

onUnmounted(() => {
  unsubscribeNotifications()
  unsubscribeIncidentsNotifications()
  if (incidentsPollTimer) {
    clearInterval(incidentsPollTimer)
    incidentsPollTimer = null
  }
})
</script>

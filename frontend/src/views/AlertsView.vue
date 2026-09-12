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
              {{ t('nav.sections.control.items.dashboard') }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>{{ TAB_TITLES[alertsTab] || t('alerts.rulesPageTitle') }}</span>
          </div>
          <h2 class="page-title">
            {{ TAB_TITLES[alertsTab] || t('alerts.rulesPageTitle') }}
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
            {{ t('alerts.newAlertButton') }}
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="fetchError"
      class="alert alert-danger mb-3"
    >
      <IconAlertTriangle class="icon alert-icon me-2" />
      {{ t('alerts.loadRulesErrorPrefix', { error: fetchError }) }}
    </div>

    <EntityTabShell
      :model-value="alertsTab"
      :tabs="alertsTabs"
      nav-margin-class="mb-4"
      @update:model-value="onTabClick"
    >
      <template #warroom>
        <WarRoomPanel
          :incidents="(incidents as any)"
          :loading="incidentsLoading"
          :error="incidentsError"
          :is-admin="auth.isAdmin"
          :resolving-id="resolvingId"
          :acknowledging-id="acknowledgingId"
          @resolve="resolveIncident"
          @acknowledge="acknowledgeIncident"
        />
      </template>

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
          :acknowledging-id="acknowledgingId"
          @mark-all-read="markAllRead"
          @resolve="resolveIncident"
          @acknowledge="acknowledgeIncident"
        />
      </template>

      <template #maintenance>
        <MaintenanceWindowsPanel :is-admin="auth.isAdmin" />
      </template>

      <template #templates>
        <AlertRuleTemplateList
          :templates="templates"
          :loading="templatesLoading"
          :fetched="templatesFetched"
          :is-admin="auth.isAdmin"
          @add="startAddTemplate"
          @edit="startEditTemplate"
          @delete="deleteTemplate"
          @apply="startApplyTemplate"
        />
      </template>
    </EntityTabShell>

    <AlertRuleTemplateModal
      :visible="showTemplateModal"
      :template="editingTemplate"
      :agent-metrics="(capabilities?.agent_metrics as any) || []"
      :saving="templateSaving"
      :error="templateSaveError"
      @close="closeTemplateModal"
      @submit="submitTemplate"
    />
    <AlertRuleTemplateApplyModal
      :visible="showApplyModal"
      :template="applyingTemplate"
      :hosts="(hosts as any)"
      :applying="applying"
      :error="applyError"
      :result="applyResult"
      @close="closeApplyModal"
      @apply="onApplyTemplate"
    />

    <ErrorBoundary :title="t('alerts.ruleFormLoadErrorTitle')">
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
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AlertIncidentList from '../components/alerts/AlertIncidentList.vue'
import AlertReleaseSummary from '../components/alerts/AlertReleaseSummary.vue'
import AlertRuleList from '../components/alerts/AlertRuleList.vue'
import AlertRuleModal from '../components/alerts/AlertRuleModal.vue'
import AlertRuleTemplateList from '../components/alerts/AlertRuleTemplateList.vue'
import AlertRuleTemplateModal from '../components/alerts/AlertRuleTemplateModal.vue'
import AlertRuleTemplateApplyModal from '../components/alerts/AlertRuleTemplateApplyModal.vue'
import MaintenanceWindowsPanel from '../components/alerts/MaintenanceWindowsPanel.vue'
import WarRoomPanel from '../components/alerts/WarRoomPanel.vue'
import ErrorBoundary from '../components/common/ErrorBoundary.vue'
import EntityTabShell from '../components/EntityTabShell.vue'
import type { EntityTab } from '../components/EntityTabShell.vue'
import { IconAlertTriangle, IconPlus } from '@tabler/icons-vue'
import { useAlertsPage } from '../composables/useAlertsPage'
import { useNotificationHistory } from '../composables/useNotificationHistory'
import { useAlertRuleTemplates } from '../composables/useAlertRuleTemplates'
import { onNotificationsMessage } from '../composables/useNotifications'
import { useAlertRulesStore } from '../stores/alertRules'
import { useAuthStore } from '../stores/auth'
import type { AlertRule } from '../types/alert'
import type { AlertRuleTemplate, AlertRuleTemplateRequest } from '../types/generated'

const auth = useAuthStore()
const { t } = useI18n()

const TAB_TITLES = computed<Record<string, string>>(() => ({
  warroom: t('alerts.warRoomPageTitle'),
  rules: t('alerts.rulesPageTitle'),
  releases: t('alerts.versionTrackingTitle'),
  incidents: t('alerts.notificationHistoryPageTitle'),
  maintenance: t('alerts.maintenanceTitle'),
  templates: t('alerts.templatesTitle'),
}))

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
  acknowledgingId,
  acknowledgeIncident,
  onWebSocketAlert: onNotificationHistoryWSAlert,
} = useNotificationHistory()

const {
  templates,
  loading: templatesLoading,
  fetched: templatesFetched,
  saving: templateSaving,
  saveError: templateSaveError,
  applying,
  applyError,
  applyResult,
  loadTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  applyTemplate,
  clearApplyResult,
} = useAlertRuleTemplates()

const showTemplateModal = ref(false)
const editingTemplate = ref<AlertRuleTemplate | null>(null)
const showApplyModal = ref(false)
const applyingTemplate = ref<AlertRuleTemplate | null>(null)
let templatesLoaded = false

async function switchToTemplates(): Promise<void> {
  alertsTab.value = 'templates'
  if (!templatesLoaded) {
    templatesLoaded = true
    await loadTemplates()
  }
}

function startAddTemplate(): void {
  editingTemplate.value = null
  showTemplateModal.value = true
}

function startEditTemplate(template: AlertRuleTemplate): void {
  editingTemplate.value = template
  showTemplateModal.value = true
}

function closeTemplateModal(): void {
  showTemplateModal.value = false
  editingTemplate.value = null
}

async function submitTemplate(payload: AlertRuleTemplateRequest): Promise<void> {
  const ok = editingTemplate.value
    ? await updateTemplate(editingTemplate.value.id, payload)
    : await createTemplate(payload)
  if (ok) closeTemplateModal()
}

function startApplyTemplate(template: AlertRuleTemplate): void {
  applyingTemplate.value = template
  clearApplyResult()
  showApplyModal.value = true
}

function closeApplyModal(): void {
  showApplyModal.value = false
  applyingTemplate.value = null
}

const alertRulesStore = useAlertRulesStore()

async function onApplyTemplate(hostIds: string[], enabled: boolean): Promise<void> {
  if (!applyingTemplate.value) return
  const ok = await applyTemplate(applyingTemplate.value.id, hostIds, enabled)
  // Applying stamps out new rules via a path this view's own `rules` list
  // (useAlertsPage's alertRulesStore) doesn't know about — force a refetch so
  // the Rules tab isn't stuck showing stale data (or an empty state) until
  // a full page reload.
  if (ok) await alertRulesStore.fetchRules(true)
}

async function ensureIncidentsLoaded(): Promise<void> {
  if (!incidentsLoaded.value) await loadIncidents()
}

async function switchToWarRoom(): Promise<void> {
  alertsTab.value = 'warroom'
  await ensureIncidentsLoaded()
}

async function switchToIncidents(): Promise<void> {
  alertsTab.value = 'incidents'
  await ensureIncidentsLoaded()
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
    key: 'warroom',
    label: t('alerts.warRoomPageTitle'),
    badges: activeIncidentCount.value > 0 ? [{ value: activeIncidentCount.value, badgeClass: 'badge bg-danger-lt text-danger ms-1' }] : [],
    lazy: true,
  },
  {
    key: 'rules',
    label: t('alerts.rulesTabLabel'),
    badges: [{ value: rules.value.length, badgeClass: 'badge bg-azure-lt text-azure ms-1' }],
    lazy: true,
  },
  {
    key: 'releases',
    label: t('alerts.versionTrackingTitle'),
    badges: trackers.value.length > 0 ? [{ value: trackers.value.length, badgeClass: 'badge bg-azure-lt text-azure ms-1' }] : [],
    lazy: true,
  },
  {
    key: 'incidents',
    label: t('alerts.notificationHistoryTabLabel'),
    badges: activeIncidentCount.value > 0 ? [{ value: activeIncidentCount.value, badgeClass: 'badge bg-danger-lt text-danger ms-1' }] : [],
    lazy: true,
  },
  {
    key: 'maintenance',
    label: t('alerts.maintenanceTabLabel'),
    badges: [],
    lazy: true,
  },
  {
    key: 'templates',
    label: t('alerts.templatesTabLabel'),
    badges: templates.value.length > 0 ? [{ value: templates.value.length, badgeClass: 'badge bg-azure-lt text-azure ms-1' }] : [],
    lazy: true,
  },
])

// 'releases'/'incidents'/'warroom' each own the actual tab switch
// (alertsTab.value=...) as part of their lazy-load-on-first-visit logic;
// 'rules' has no such loader, so it's a plain assignment.
function onTabClick(key: string): void {
  if (key === 'releases') { switchToTrackers(); return }
  if (key === 'incidents') { switchToIncidents(); return }
  if (key === 'warroom') { switchToWarRoom(); return }
  if (key === 'templates') { switchToTemplates(); return }
  alertsTab.value = key
}

let incidentsPollTimer: ReturnType<typeof setInterval> | null = null

watch(alertsTab, (tab) => {
  router.replace({ query: { ...route.query, tab } })
})

onMounted(async () => {
  await init()

  // Default landing tab is the war-room triage view, not rule configuration
  // — an ops opening /alerts wants to see what's actually firing right now,
  // grouped by severity, before anything else. `?tab=rules`/`?tab=releases`/
  // `?tab=incidents` (used by deep links, e.g. the command palette's
  // alert-rule search results, or HostDetailView's incident history link)
  // are honored explicitly.
  if (route.query.tab === 'rules') {
    // stays on the 'rules' default from useAlertsPage()
  } else if (route.query.tab === 'releases') {
    await switchToTrackers()
  } else if (route.query.tab === 'incidents') {
    await switchToIncidents()
  } else if (route.query.tab === 'templates') {
    await switchToTemplates()
  } else {
    await switchToWarRoom()
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

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
            v-if="alertsTab === 'rules'"
            class="btn btn-primary"
            @click="startAddAlert"
          >
            <svg
              class="icon me-1"
              width="24"
              height="24"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 4v16m8-8H4"
              />
            </svg>
            Nouvelle alerte
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="fetchError"
      class="alert alert-danger mb-3"
    >
      <AppIcon
        name="warning"
        css-class="icon alert-icon me-2"
      />
      Erreur de chargement des règles : {{ fetchError }}
    </div>

    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: alertsTab === 'rules' }"
          href="#"
          @click.prevent="alertsTab = 'rules'"
        >
          Règles
          <span class="badge bg-azure-lt text-azure ms-1">{{ rules.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: alertsTab === 'releases' }"
          href="#"
          @click.prevent="switchToTrackers"
        >
          Suivi de versions
          <span
            v-if="trackers.length > 0"
            class="badge bg-azure-lt text-azure ms-1"
          >{{ trackers.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: alertsTab === 'incidents' }"
          href="#"
          @click.prevent="switchToIncidents"
        >
          Historique notifications
          <span
            v-if="activeIncidentCount > 0"
            class="badge bg-red-lt text-red ms-1"
          >{{ activeIncidentCount }}</span>
        </a>
      </li>
    </ul>

    <div v-show="alertsTab === 'rules'">
      <AlertRuleList
        :rules="(rules as any)"
        :hosts="(hosts as any)"
        :loading="loading"
        :fetched="fetched"
        :error="saveError"
        :format-date="(formatDate as any)"
        @add="startAddAlert"
        @edit="(startEditAlert as any)"
        @toggle="(toggleEnabled as any)"
        @delete="(deleteAlert as any)"
      />
    </div>

    <div v-show="alertsTab === 'releases'">
      <AlertReleaseSummary
        :trackers="(trackers as any)"
        :loading="trackersLoading"
        :error="trackersError"
      />
    </div>

    <div v-show="alertsTab === 'incidents'">
      <AlertIncidentList
        :incidents="(incidents as any)"
        :loading="incidentsLoading"
        :error="incidentsError"
        :active-incident-count="activeIncidentCount"
        @refresh="loadIncidents"
      />
    </div>

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
  </div>
</template>

<script setup lang="ts">
import { watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AlertIncidentList from '../components/alerts/AlertIncidentList.vue'
import AlertReleaseSummary from '../components/alerts/AlertReleaseSummary.vue'
import AlertRuleList from '../components/alerts/AlertRuleList.vue'
import AlertRuleModal from '../components/alerts/AlertRuleModal.vue'
import AppIcon from '../components/AppIcon.vue'
import { useAlertsPage } from '../composables/useAlertsPage'
import { useWebSocket } from '../composables/useWebSocket'

const TAB_TITLES: Record<string, string> = {
  rules: 'Alertes',
  releases: 'Suivi de versions',
  incidents: 'Historique de notifications',
}

const route = useRoute()
const router = useRouter()
const {
  alertsTab,
  incidents,
  incidentsLoading,
  incidentsError,
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
  activeIncidentCount,
  init,
  loadIncidents,
  switchToIncidents,
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

let incidentsPollTimer: ReturnType<typeof setInterval> | null = null

watch(alertsTab, (tab) => {
  router.replace({ query: { ...route.query, tab } })
})

onMounted(async () => {
  await init()

  if (route.query.tab === 'incidents') {
    await switchToIncidents()
  } else if (route.query.tab === 'releases') {
    await switchToTrackers()
  }

  incidentsPollTimer = setInterval(loadIncidents, 300_000)
})

onUnmounted(() => {
  if (incidentsPollTimer) {
    clearInterval(incidentsPollTimer)
    incidentsPollTimer = null
  }
})

useWebSocket('/api/v1/ws/notifications', onWebSocketAlert)
</script>

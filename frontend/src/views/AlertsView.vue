<template>
  <div class="page-wrapper">
    <div class="page-header d-print-none">
      <div class="container-xl">
        <div class="row g-2 align-items-center">
          <div class="col">
            <div class="page-pretitle">
              <router-link to="/" class="text-decoration-none">Dashboard</router-link>
              <span class="text-muted mx-1">/</span>
              <span>Alertes</span>
            </div>
            <h2 class="page-title">Alertes</h2>
          </div>
          <div class="col-auto ms-auto d-flex gap-2">
            <button v-if="alertsTab === 'rules'" @click="startAddAlert" class="btn btn-primary">
              <svg class="icon me-1" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
              </svg>
              Nouvelle alerte
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="page-body">
      <div class="container-xl">
        <ul class="nav nav-tabs mb-4">
          <li class="nav-item">
            <a class="nav-link" :class="{ active: alertsTab === 'rules' }" href="#" @click.prevent="alertsTab = 'rules'">
              Regles
              <span class="badge bg-azure-lt text-azure ms-1">{{ rules.length }}</span>
            </a>
          </li>
          <li class="nav-item">
            <a class="nav-link" :class="{ active: alertsTab === 'incidents' }" href="#" @click.prevent="switchToIncidents">
              Incidents
              <span v-if="activeIncidentCount > 0" class="badge bg-red-lt text-red ms-1">{{ activeIncidentCount }}</span>
            </a>
          </li>
        </ul>

        <div v-show="alertsTab === 'rules'">
          <AlertRuleList
            :rules="rules"
            :hosts="hosts"
            :loading="loading"
            :error="saveError"
            :format-date="formatDate"
            @add="startAddAlert"
            @edit="startEditAlert"
            @toggle="toggleEnabled"
            @delete="deleteAlert"
          />
        </div>

        <div v-show="alertsTab === 'incidents'">
          <AlertIncidentList
            :incidents="incidents"
            :loading="incidentsLoading"
            :error="incidentsError"
            :active-incident-count="activeIncidentCount"
            @refresh="loadIncidents"
          />
        </div>
      </div>
    </div>

    <AlertRuleModal
      :visible="showModal"
      :rule="editingRule"
      :hosts="hosts"
      :saving="saving"
      :error="saveError"
      @close="closeModal"
      @submit="saveAlert"
    />
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AlertIncidentList from '../components/alerts/AlertIncidentList.vue'
import AlertRuleList from '../components/alerts/AlertRuleList.vue'
import AlertRuleModal from '../components/alerts/AlertRuleModal.vue'
import { useAlertsPage } from '../composables/useAlertsPage'
import { useWebSocket } from '../composables/useWebSocket'

const route = useRoute()
const {
  alertsTab,
  incidents,
  incidentsLoading,
  incidentsError,
  incidentsLoaded,
  rules,
  hosts,
  loading,
  showModal,
  saving,
  saveError,
  editingRule,
  activeIncidentCount,
  init,
  loadIncidents,
  switchToIncidents,
  startAddAlert,
  startEditAlert,
  saveAlert,
  toggleEnabled,
  deleteAlert,
  closeModal,
  formatDate,
  onWebSocketAlert,
} = useAlertsPage()

let incidentsPollTimer = null

onMounted(async () => {
  if (route.query.tab === 'incidents') {
    await switchToIncidents()
  }
  await init()

  // Fallback safety net — incidents are now event-driven via WS (alert_incident_update).
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



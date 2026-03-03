<template>
  <div class="page-wrapper">
    <div class="page-header d-print-none">
      <div class="container-xl">
        <div class="row g-2 align-items-center">
          <div class="col">
            <div class="page-pretitle">
              <router-link to="/" class="text-decoration-none">Dashboard</router-link>
              <span class="text-muted mx-1">/</span>
              <router-link to="/alerts" class="text-decoration-none">Alertes</router-link>
              <span class="text-muted mx-1">/</span>
              <span>Incidents</span>
            </div>
            <h2 class="page-title">Historique des incidents</h2>
            <div class="text-muted mt-1">Liste des alertes déclenchées sur vos hôtes</div>
          </div>
          <div class="col-auto ms-auto d-flex gap-2 align-items-center">
            <span v-if="activeCount > 0" class="badge bg-red-lt text-red">
              {{ activeCount }} actif{{ activeCount > 1 ? 's' : '' }}
            </span>
            <button class="btn btn-ghost-secondary" @click="load" :disabled="loading">
              <svg class="icon" width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
              </svg>
              Actualiser
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="page-body">
      <div class="container-xl">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">Incidents récents</h3>
            <span class="text-muted small">{{ incidents.length }} incident{{ incidents.length !== 1 ? 's' : '' }}</span>
          </div>

          <!-- Loading -->
          <div v-if="loading" class="card-body text-center py-5">
            <div class="spinner-border text-primary" role="status"></div>
            <div class="mt-2 text-muted">Chargement...</div>
          </div>

          <!-- Error -->
          <div v-else-if="error" class="card-body text-center py-5 text-danger">
            <svg class="icon icon-lg mb-2" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
            </svg>
            <div>{{ error }}</div>
            <button class="btn btn-outline-secondary mt-3" @click="load">Réessayer</button>
          </div>

          <!-- Empty -->
          <div v-else-if="incidents.length === 0" class="card-body text-center py-5 text-muted">
            <svg class="icon icon-lg mb-3" width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
            </svg>
            <div>Aucun incident enregistré</div>
            <div class="text-muted small mt-1">Les incidents apparaîtront ici lorsqu'une règle d'alerte se déclenchera</div>
            <router-link to="/alerts" class="btn btn-outline-primary mt-3">Configurer des règles d'alertes</router-link>
          </div>

          <!-- Table -->
          <div v-else class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th style="width: 90px;">État</th>
                  <th>Règle</th>
                  <th>Hôte</th>
                  <th>Valeur</th>
                  <th>Déclenché</th>
                  <th>Résolu</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="inc in incidents" :key="inc.id">
                  <!-- État -->
                  <td>
                    <span v-if="inc.resolved_at" class="badge bg-success-lt text-success">Résolu</span>
                    <span v-else class="badge bg-red-lt text-red">Actif</span>
                  </td>

                  <!-- Règle -->
                  <td>
                    <div class="fw-semibold text-truncate" style="max-width: 220px;" :title="inc.rule_name">
                      {{ inc.rule_name }}
                    </div>
                    <div class="text-muted small">{{ metricLabel(inc.metric) }}</div>
                  </td>

                  <!-- Hôte -->
                  <td>
                    <router-link :to="`/hosts/${inc.host_id}`" class="text-decoration-none">
                      {{ inc.host_name }}
                    </router-link>
                  </td>

                  <!-- Valeur -->
                  <td>
                    <code>{{ formatValue(inc.value, inc.metric) }}</code>
                  </td>

                  <!-- Déclenché -->
                  <td class="text-muted">
                    <RelativeTime :date="inc.triggered_at" />
                  </td>

                  <!-- Résolu -->
                  <td class="text-muted">
                    <RelativeTime v-if="inc.resolved_at" :date="inc.resolved_at" />
                    <span v-else class="text-secondary">—</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import RelativeTime from '../components/RelativeTime.vue'
import apiClient from '../api'

const loading = ref(false)
const error = ref('')
const incidents = ref([])

const activeCount = computed(() => incidents.value.filter(i => !i.resolved_at).length)

function metricLabel(metric) {
  const labels = {
    cpu: 'CPU', cpu_percent: 'CPU',
    memory: 'RAM', ram_percent: 'RAM',
    disk: 'Disque', disk_percent: 'Disque',
    load: 'Load avg',
    status_offline: 'Statut hôte',
  }
  return labels[metric] || metric || ''
}

function formatValue(value, metric) {
  if (metric === 'status_offline') return value === 1 ? 'offline' : 'online'
  const unit = ['cpu', 'cpu_percent', 'memory', 'ram_percent', 'disk', 'disk_percent'].includes(metric) ? '%' : ''
  return `${Number(value).toFixed(2)}${unit}`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await apiClient.getNotifications()
    incidents.value = res.data?.notifications || []
  } catch (e) {
    error.value = 'Impossible de charger les incidents'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

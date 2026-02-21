<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Audit APT</h2>
        <div class="text-secondary">Historique des actions APT</div>
      </div>
      <div class="d-flex align-items-center gap-2">
        <button class="btn btn-outline-secondary" @click="refresh" :disabled="loading">Actualiser</button>
      </div>
    </div>

    <div class="card">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Utilisateur</th>
              <th>Action</th>
              <th>Hote</th>
              <th>IP</th>
              <th>Statut</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in logs" :key="log.id">
              <td>{{ formatDate(log.created_at) }}</td>
              <td class="fw-semibold">{{ log.username }}</td>
              <td><code>{{ log.action }}</code></td>
              <td>
                <router-link
                  v-if="log.host_id"
                  :to="`/hosts/${log.host_id}`"
                  class="text-decoration-none fw-semibold"
                >
                  {{ log.host_name || log.host_id }}
                </router-link>
                <span v-else class="text-secondary">-</span>
              </td>
              <td class="text-secondary small">{{ log.ip_address || '-' }}</td>
              <td>
                <span :class="statusClass(log.status)">{{ log.status }}</span>
              </td>
            </tr>
            <tr v-if="!logs.length && !loading">
              <td colspan="6" class="text-center text-secondary py-4">Aucun log disponible</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="card-footer d-flex align-items-center justify-content-between">
        <div class="text-secondary small">Page {{ page }}</div>
        <div class="btn-group">
          <button class="btn btn-outline-secondary" @click="prevPage" :disabled="page <= 1 || loading">Precedent</button>
          <button class="btn btn-outline-secondary" @click="nextPage" :disabled="logs.length < limit || loading">Suivant</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import apiClient from '../api'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'

const logs = ref([])
const page = ref(1)
const limit = ref(50)
const loading = ref(false)

dayjs.extend(utc)

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

function formatDate(date) {
  if (!date) return '-'
  return dayjs.utc(date).local().format('YYYY-MM-DD HH:mm')
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await apiClient.getAuditLogs(page.value, limit.value)
    logs.value = res.data?.logs || []
  } catch (e) {
    logs.value = []
  } finally {
    loading.value = false
  }
}

function refresh() {
  fetchLogs()
}

function nextPage() {
  if (logs.value.length < limit.value) return
  page.value += 1
  fetchLogs()
}

function prevPage() {
  if (page.value <= 1) return
  page.value -= 1
  fetchLogs()
}

onMounted(fetchLogs)
</script>

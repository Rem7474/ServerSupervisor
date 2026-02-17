<template>
  <div>
    <h1 class="text-2xl font-bold mb-2">APT - Mises à jour système</h1>
    <p class="text-gray-400 mb-8">Gérer les mises à jour APT sur tous les hôtes</p>

    <!-- Bulk Actions -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold mb-4">Actions groupées</h3>
      <div class="flex items-center gap-4 flex-wrap">
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" v-model="selectAll" @change="toggleSelectAll" class="rounded" />
          Sélectionner tous les hôtes
        </label>
        <div class="flex gap-2 ml-auto">
          <button @click="bulkAptCmd('update')" class="btn-secondary text-sm" :disabled="selectedHosts.length === 0">
            apt update ({{ selectedHosts.length }})
          </button>
          <button @click="bulkAptCmd('upgrade')" class="btn-primary text-sm" :disabled="selectedHosts.length === 0">
            apt upgrade ({{ selectedHosts.length }})
          </button>
          <button @click="bulkAptCmd('dist-upgrade')" class="btn-danger text-sm" :disabled="selectedHosts.length === 0">
            apt dist-upgrade ({{ selectedHosts.length }})
          </button>
        </div>
      </div>
    </div>

    <!-- Per-host APT Status -->
    <div class="space-y-4">
      <div v-for="host in hosts" :key="host.id" class="card">
        <div class="flex items-center gap-4 mb-4">
          <input type="checkbox" :value="host.id" v-model="selectedHosts" class="rounded" />
          <div class="flex-1">
            <h3 class="font-semibold">{{ host.hostname }}</h3>
            <p class="text-gray-400 text-sm">{{ host.ip_address }}</p>
          </div>
          <span :class="host.status === 'online' ? 'badge-online' : 'badge-offline'">
            {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
          </span>
        </div>

        <div v-if="aptStatuses[host.id]" class="grid grid-cols-4 gap-4 mb-4">
          <div class="bg-dark-900 rounded-lg p-3 text-center">
            <div class="text-xl font-bold" :class="aptStatuses[host.id].pending_packages > 0 ? 'text-yellow-400' : 'text-emerald-400'">
              {{ aptStatuses[host.id].pending_packages }}
            </div>
            <div class="text-gray-500 text-xs">En attente</div>
          </div>
          <div class="bg-dark-900 rounded-lg p-3 text-center">
            <div class="text-xl font-bold text-red-400">{{ aptStatuses[host.id].security_updates }}</div>
            <div class="text-gray-500 text-xs">Sécurité</div>
          </div>
          <div class="bg-dark-900 rounded-lg p-3 text-center">
            <div class="text-xs font-medium text-gray-300">{{ formatDate(aptStatuses[host.id].last_update) }}</div>
            <div class="text-gray-500 text-xs">Dernier update</div>
          </div>
          <div class="bg-dark-900 rounded-lg p-3 text-center">
            <div class="text-xs font-medium text-gray-300">{{ formatDate(aptStatuses[host.id].last_upgrade) }}</div>
            <div class="text-gray-500 text-xs">Dernier upgrade</div>
          </div>
        </div>

        <!-- Command History -->
        <div v-if="aptHistories[host.id]?.length" class="mt-4">
          <button @click="toggleHistory(host.id)" class="text-sm text-primary-400 hover:underline mb-2">
            {{ expandedHistories[host.id] ? 'Masquer' : 'Voir' }} l'historique ({{ aptHistories[host.id].length }})
          </button>
          <div v-if="expandedHistories[host.id]" class="space-y-2 mt-2">
            <div v-for="cmd in aptHistories[host.id]" :key="cmd.id"
              class="bg-dark-900 rounded-lg p-3 text-sm">
              <div class="flex items-center justify-between">
                <span class="font-medium">apt {{ cmd.command }}</span>
                <span :class="{
                  'text-emerald-400': cmd.status === 'completed',
                  'text-yellow-400': cmd.status === 'pending' || cmd.status === 'running',
                  'text-red-400': cmd.status === 'failed'
                }">{{ cmd.status }}</span>
              </div>
              <div class="text-gray-500 text-xs mt-1">{{ formatDate(cmd.created_at) }}</div>
              <pre v-if="cmd.output" class="mt-2 text-xs text-gray-400 bg-dark-950 p-2 rounded max-h-32 overflow-y-auto">{{ cmd.output }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import apiClient from '../api'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.locale('fr')

const hosts = ref([])
const selectedHosts = ref([])
const selectAll = ref(false)
const aptStatuses = ref({})
const aptHistories = ref({})
const expandedHistories = ref({})

function toggleSelectAll() {
  if (selectAll.value) {
    selectedHosts.value = hosts.value.map(h => h.id)
  } else {
    selectedHosts.value = []
  }
}

function toggleHistory(hostId) {
  expandedHistories.value[hostId] = !expandedHistories.value[hostId]
}

async function bulkAptCmd(command) {
  const hostnames = hosts.value.filter(h => selectedHosts.value.includes(h.id)).map(h => h.hostname).join(', ')
  if (!confirm(`Exécuter 'apt ${command}' sur: ${hostnames} ?`)) return
  try {
    await apiClient.sendAptCommand(selectedHosts.value, command)
    alert(`Commande envoyée à ${selectedHosts.value.length} hôte(s)`)
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs(date).fromNow()
}

onMounted(async () => {
  try {
    const res = await apiClient.getHosts()
    hosts.value = res.data

    for (const host of hosts.value) {
      try {
        const [aptRes, histRes] = await Promise.all([
          apiClient.getAptStatus(host.id).catch(() => null),
          apiClient.getAptHistory(host.id).catch(() => ({ data: [] })),
        ])
        if (aptRes?.data) aptStatuses.value[host.id] = aptRes.data
        aptHistories.value[host.id] = histRes?.data || []
      } catch (e) { /* ignore */ }
    }
  } catch (e) {
    console.error('Failed to fetch data:', e)
  }
})
</script>

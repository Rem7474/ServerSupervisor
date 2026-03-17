<template>
  <div v-if="aptStatus" class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">APT - Mises a jour systeme</h3>
      <div class="btn-group btn-group-sm" v-if="canRunApt">
        <button @click="$emit('run-apt-command', 'update')" class="btn btn-outline-secondary" :disabled="!!aptCmdLoading">
          <span v-if="aptCmdLoading === 'update'" class="spinner-border spinner-border-sm me-1"></span>
          apt update
        </button>
        <button @click="$emit('run-apt-command', 'upgrade')" class="btn btn-primary" :disabled="!!aptCmdLoading">
          <span v-if="aptCmdLoading === 'upgrade'" class="spinner-border spinner-border-sm me-1"></span>
          apt upgrade
        </button>
        <button @click="$emit('run-apt-command', 'dist-upgrade')" class="btn btn-outline-danger" :disabled="!!aptCmdLoading">
          <span v-if="aptCmdLoading === 'dist-upgrade'" class="spinner-border spinner-border-sm me-1"></span>
          apt dist-upgrade
        </button>
      </div>
      <span v-else class="text-secondary small">Mode lecture seule</span>
    </div>
    <div class="card-body">
      <div class="row row-cards">
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="h2 mb-0" :class="aptStatus.pending_packages > 0 ? 'text-yellow' : 'text-green'">
                {{ aptStatus.pending_packages }}
              </div>
              <div class="text-secondary small">Paquets en attente</div>
            </div>
          </div>
        </div>
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="h2 mb-0 text-red">{{ aptStatus.security_updates }}</div>
              <div class="text-secondary small">Mises a jour securite</div>
            </div>
          </div>
        </div>
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="fw-semibold">{{ formatDate(aptStatus.last_update) }}</div>
              <div class="text-secondary small">Derniere mise a jour</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="aptStatus.cve_list" class="mt-3">
        <CVEList
          :cveList="aptStatus.cve_list"
          :showMaxSeverity="true"
          :alwaysExpanded="true"
        />
      </div>
    </div>
  </div>
  <div v-else class="card"><div class="card-body text-secondary">Donnees APT non disponibles pour cet hote.</div></div>
</template>

<script setup>
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'
import CVEList from '../CVEList.vue'

defineEmits(['run-apt-command'])

const props = defineProps({
  aptStatus: {
    type: Object,
    default: null,
  },
  canRunApt: {
    type: Boolean,
    default: false,
  },
  aptCmdLoading: {
    type: String,
    default: '',
  },
})

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}
</script>

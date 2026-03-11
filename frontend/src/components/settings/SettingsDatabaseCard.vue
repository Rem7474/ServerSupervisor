<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">Base de donnees</h3>
    </div>
    <div class="card-body">
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">Connexion</div>
        <span :class="dbStatus.connected ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
          {{ dbStatus.connected ? 'Connectee' : 'Deconnectee' }}
        </span>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">Logs audit</div>
        <div>{{ formatNumber(dbStatus.auditLogCount) }} entrees</div>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">Metriques stockees</div>
        <div>{{ formatNumber(dbStatus.metricsCount) }} points</div>
      </div>
      <div>
        <div class="text-secondary small">Hotes enregistres</div>
        <div>{{ dbStatus.hostsCount }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  dbStatus: {
    type: Object,
    required: true,
  },
  formatNumber: {
    type: Function,
    required: true,
  },
})
</script>

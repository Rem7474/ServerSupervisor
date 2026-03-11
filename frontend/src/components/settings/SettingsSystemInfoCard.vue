<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">Systeme</h3>
    </div>
    <div class="card-body">
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">URL de base</div>
        <div class="font-monospace">{{ settings.baseUrl || 'Non configure' }}</div>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">Base de donnees</div>
        <div class="font-monospace">{{ settings.dbHost }}:{{ settings.dbPort }}</div>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">Mode HTTPS/TLS</div>
        <span :class="settings.tlsEnabled ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
          {{ settings.tlsEnabled ? 'Active' : 'Desactive' }}
        </span>
      </div>
      <div class="mb-3">
        <div class="text-secondary small">Version agent recommandee</div>
        <div class="font-monospace">{{ settings.latestAgentVersion || '-' }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  settings: {
    type: Object,
    required: true,
  },
})
</script>

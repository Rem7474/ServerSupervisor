<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ mode === 'threats' ? 'Menaces web' : 'Stats web' }}</span>
        </div>
        <h2 class="page-title">
          {{ mode === 'threats' ? 'Menaces web' : 'Stats web' }}
        </h2>
        <div class="text-secondary">
          {{ mode === 'threats'
            ? 'IPs suspectes, chemins scannés, corrélation multi-hôtes et chronologie détaillée'
            : 'Trafic HTTP, erreurs, endpoints, géographie des clients et actualisation automatique' }}
        </div>
      </div>
      <div
        class="btn-group"
        role="group"
        aria-label="Vue d'ensemble ou Menaces"
      >
        <router-link
          to="/traffic"
          class="btn"
          :class="mode === 'overview' ? 'btn-primary' : 'btn-outline-secondary'"
        >
          Vue d'ensemble
        </router-link>
        <router-link
          to="/threats"
          class="btn"
          :class="mode === 'threats' ? 'btn-primary' : 'btn-outline-secondary'"
        >
          Menaces
        </router-link>
      </div>
    </div>

    <TrafficOverviewPanel v-if="mode === 'overview'" />
    <ThreatsPanel v-else />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import TrafficOverviewPanel from '../components/security/TrafficOverviewPanel.vue'
import ThreatsPanel from '../components/security/ThreatsPanel.vue'

const route = useRoute()
const mode = computed(() => (route.path === '/threats' ? 'threats' : 'overview'))
</script>

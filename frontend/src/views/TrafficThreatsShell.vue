<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            {{ t('nav.sections.control.items.dashboard') }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ mode === 'threats' ? t('security.threatsPageTitle') : t('security.trafficPageTitle') }}</span>
        </div>
        <h2 class="page-title">
          {{ mode === 'threats' ? t('security.threatsPageTitle') : t('security.trafficPageTitle') }}
        </h2>
        <div class="text-secondary">
          {{ mode === 'threats' ? t('security.threatsPageDescription') : t('security.trafficPageDescription') }}
        </div>
      </div>
      <div
        class="btn-group"
        role="group"
        :aria-label="t('security.modeToggleAriaLabel')"
      >
        <router-link
          to="/traffic"
          class="btn"
          :class="mode === 'overview' ? 'btn-primary' : 'btn-outline-secondary'"
        >
          {{ t('security.overviewModeButton') }}
        </router-link>
        <router-link
          to="/threats"
          class="btn"
          :class="mode === 'threats' ? 'btn-primary' : 'btn-outline-secondary'"
        >
          {{ t('security.threatsModeButton') }}
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
import { useI18n } from 'vue-i18n'
import TrafficOverviewPanel from '../components/security/TrafficOverviewPanel.vue'
import ThreatsPanel from '../components/security/ThreatsPanel.vue'

const route = useRoute()
const { t } = useI18n()
const mode = computed(() => (route.path === '/threats' ? 'threats' : 'overview'))
</script>

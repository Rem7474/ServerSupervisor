<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ t('settings.pageTitle') }}</span>
      </div>
      <h2 class="page-title">
        {{ t('settings.pageTitle') }}
      </h2>
    </div>

    <div class="row">
      <!-- Sidebar navigation -->
      <div class="col-12 col-md-3 mb-4 mb-md-0">
        <div class="list-group mb-3">
          <button
            v-for="tabItem in SETTINGS_TABS"
            :key="tabItem.key"
            type="button"
            class="list-group-item list-group-item-action d-flex align-items-center gap-2"
            :class="{ active: tab === tabItem.key }"
            @click="tab = tabItem.key"
          >
            <component
              :is="tabItem.icon"
              :size="16"
              class="icon"
            />
            {{ tabItem.label }}
          </button>
        </div>

        <!-- Danger zone: visually isolated so destructive maintenance actions
             aren't one indistinguishable tab among six equally-weighted ones. -->
        <div class="text-danger small fw-medium mb-1 px-1">
          {{ t('settings.dangerZone') }}
        </div>
        <div class="list-group border-danger">
          <button
            type="button"
            class="list-group-item list-group-item-action d-flex align-items-center gap-2 text-danger"
            :class="{ active: tab === 'maintenance' }"
            @click="tab = 'maintenance'"
          >
            <IconAlertTriangle
              :size="16"
              class="icon"
            />
            {{ t('settings.tabs.maintenance') }}
          </button>
        </div>
      </div>

      <!-- Content -->
      <div class="col-12 col-md-9">
        <!-- General -->
        <div
          v-show="tab === 'general'"
          class="row row-cards"
        >
          <div class="col-lg-6">
            <SettingsSystemInfoCard :settings="settings" />
          </div>
          <div class="col-lg-6">
            <SettingsDatabaseCard
              :db-status="dbStatus"
              :format-number="formatNumber"
            />
          </div>
        </div>

        <!-- Notifications -->
        <div v-show="tab === 'notifications'">
          <SettingsSmtpCard
            :form="form"
            :auth-is-admin="auth.isAdmin"
            :show-smtp-pass="showSmtpPass"
            :saving-smtp="savingSmtp"
            :smtp-save-msg="smtpSaveMsg"
            :smtp-save-ok="smtpSaveOk"
            :testing-smtp="testingSmtp"
            :smtp-test-message="smtpTestMessage"
            :smtp-test-success="smtpTestSuccess"
            @update:show-smtp-pass="showSmtpPass = $event"
            @save="saveSmtp"
            @test="testSmtp"
          />
          <div class="mt-4">
            <SettingsNotificationsCard
              :form="form"
              :auth-is-admin="auth.isAdmin"
              :show-git-hub-token="showGitHubToken"
              :saving-notif="savingNotif"
              :notif-save-msg="notifSaveMsg"
              :notif-save-ok="notifSaveOk"
              :testing-ntfy="testingNtfy"
              :ntfy-test-message="ntfyTestMessage"
              :ntfy-test-success="ntfyTestSuccess"
              @update:show-github-token="showGitHubToken = $event"
              @save="saveNotifications"
              @test="testNtfy"
            />
          </div>
        </div>

        <!-- Integrations -->
        <div v-show="tab === 'integrations'">
          <SettingsProxmoxCard :auth-is-admin="auth.isAdmin" />
          <SettingsNPMCard :auth-is-admin="auth.isAdmin" />
          <SettingsRegistryCredentialsCard :auth-is-admin="auth.isAdmin" />
        </div>

        <!-- Retention -->
        <div v-show="tab === 'retention'">
          <SettingsRetentionCard
            :form="form"
            :audit-categories="settings.auditCategories"
            :auth-is-admin="auth.isAdmin"
            :saving-retention="savingRetention"
            :retention-save-msg="retentionSaveMsg"
            :retention-save-ok="retentionSaveOk"
            @save="saveRetention"
          />
        </div>

        <!-- Threat detection -->
        <div v-show="tab === 'threats'">
          <SettingsThreatDetectionCard
            v-model:form="form"
            :auth-is-admin="auth.isAdmin"
            :saving-threat-detection="savingThreatDetection"
            :threat-detection-save-msg="threatDetectionSaveMsg"
            :threat-detection-save-ok="threatDetectionSaveOk"
            @save="saveThreatDetection"
          />
        </div>

        <!-- Maintenance -->
        <div v-show="tab === 'maintenance'">
          <SettingsMaintenanceCard
            :settings="settings"
            :cleaning-metrics="cleaningMetrics"
            :clean-message="cleanMessage"
            :clean-success="cleanSuccess"
            :cleaning-audit-logs="cleaningAuditLogs"
            :audit-clean-message="auditCleanMessage"
            :audit-clean-success="auditCleanSuccess"
            @clean-metrics="cleanMetrics"
            @clean-audit="cleanAuditLogs"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  IconAdjustments, IconAlertTriangle, IconBell, IconDatabase, IconPlugConnected, IconShieldSearch,
} from '@tabler/icons-vue'
import SettingsDatabaseCard from '../components/settings/SettingsDatabaseCard.vue'
import SettingsMaintenanceCard from '../components/settings/SettingsMaintenanceCard.vue'
import SettingsNotificationsCard from '../components/settings/SettingsNotificationsCard.vue'
import SettingsRetentionCard from '../components/settings/SettingsRetentionCard.vue'
import SettingsSmtpCard from '../components/settings/SettingsSmtpCard.vue'
import SettingsThreatDetectionCard from '../components/settings/SettingsThreatDetectionCard.vue'
import SettingsSystemInfoCard from '../components/settings/SettingsSystemInfoCard.vue'
import SettingsProxmoxCard from '../components/settings/SettingsProxmoxCard.vue'
import SettingsNPMCard from '../components/settings/SettingsNPMCard.vue'
import SettingsRegistryCredentialsCard from '../components/settings/SettingsRegistryCredentialsCard.vue'
import { useSettings } from '../composables/useSettings'

const { t } = useI18n()

const SETTINGS_TABS = computed(() => [
  { key: 'general', label: t('settings.tabs.general'), icon: IconAdjustments },
  { key: 'notifications', label: t('settings.tabs.notifications'), icon: IconBell },
  { key: 'integrations', label: t('settings.tabs.integrations'), icon: IconPlugConnected },
  { key: 'retention', label: t('settings.tabs.retention'), icon: IconDatabase },
  { key: 'threats', label: t('settings.tabs.threats'), icon: IconShieldSearch },
])

const {
  auth,
  tab,
  settings,
  dbStatus,
  form,
  showSmtpPass,
  showGitHubToken,
  savingSmtp,
  smtpSaveMsg,
  smtpSaveOk,
  testingSmtp,
  smtpTestMessage,
  smtpTestSuccess,
  savingNotif,
  notifSaveMsg,
  notifSaveOk,
  testingNtfy,
  ntfyTestMessage,
  ntfyTestSuccess,
  savingRetention,
  retentionSaveMsg,
  retentionSaveOk,
  savingThreatDetection,
  threatDetectionSaveMsg,
  threatDetectionSaveOk,
  cleaningMetrics,
  cleanMessage,
  cleanSuccess,
  cleaningAuditLogs,
  auditCleanMessage,
  auditCleanSuccess,
  formatNumber,
  saveSmtp,
  saveNotifications,
  saveRetention,
  saveThreatDetection,
  testSmtp,
  testNtfy,
  cleanMetrics,
  cleanAuditLogs,
} = useSettings()
</script>

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
        <span>Paramètres</span>
      </div>
      <h2 class="page-title">
        Paramètres
      </h2>
    </div>

    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: tab === 'general' }"
          @click="tab = 'general'"
        >
          Général
        </button>
      </li>
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: tab === 'notifications' }"
          @click="tab = 'notifications'"
        >
          Notifications
        </button>
      </li>
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: tab === 'integrations' }"
          @click="tab = 'integrations'"
        >
          Intégrations
        </button>
      </li>
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: tab === 'retention' }"
          @click="tab = 'retention'"
        >
          Rétention
        </button>
      </li>
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: tab === 'maintenance' }"
          @click="tab = 'maintenance'"
        >
          Maintenance
        </button>
      </li>
    </ul>

    <!-- Général -->
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

    <!-- Intégrations -->
    <div v-show="tab === 'integrations'">
      <SettingsProxmoxCard :auth-is-admin="auth.isAdmin" />
      <SettingsNPMCard :auth-is-admin="auth.isAdmin" />
      <SettingsRegistryCredentialsCard :auth-is-admin="auth.isAdmin" />
    </div>

    <!-- Rétention -->
    <div v-show="tab === 'retention'">
      <SettingsRetentionCard
        :form="form"
        :auth-is-admin="auth.isAdmin"
        :saving-retention="savingRetention"
        :retention-save-msg="retentionSaveMsg"
        :retention-save-ok="retentionSaveOk"
        @save="saveRetention"
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
</template>

<script setup lang="ts">
import SettingsDatabaseCard from '../components/settings/SettingsDatabaseCard.vue'
import SettingsMaintenanceCard from '../components/settings/SettingsMaintenanceCard.vue'
import SettingsNotificationsCard from '../components/settings/SettingsNotificationsCard.vue'
import SettingsRetentionCard from '../components/settings/SettingsRetentionCard.vue'
import SettingsSmtpCard from '../components/settings/SettingsSmtpCard.vue'
import SettingsSystemInfoCard from '../components/settings/SettingsSystemInfoCard.vue'
import SettingsProxmoxCard from '../components/settings/SettingsProxmoxCard.vue'
import SettingsNPMCard from '../components/settings/SettingsNPMCard.vue'
import SettingsRegistryCredentialsCard from '../components/settings/SettingsRegistryCredentialsCard.vue'
import { useSettings } from '../composables/useSettings'

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
  testSmtp,
  testNtfy,
  cleanMetrics,
  cleanAuditLogs,
} = useSettings()
</script>

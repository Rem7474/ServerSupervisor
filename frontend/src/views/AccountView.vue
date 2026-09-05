<template>
  <div>
    <!-- Forced password change banner -->
    <div
      v-if="auth.mustChangePassword"
      class="alert alert-warning alert-dismissible mb-4"
      role="alert"
    >
      <div class="d-flex align-items-center">
        <IconAlertTriangle
          :size="24"
          class="icon alert-icon me-2"
        />
        <strong>{{ t('account.passwordChangeRequiredTitle') }}</strong>&nbsp;{{ t('account.passwordChangeRequiredMessage') }}
      </div>
    </div>

    <div class="page-header mb-4">
      <div class="row align-items-center">
        <div class="col-auto">
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              {{ t('account.dashboardBreadcrumb') }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>{{ t('common.myAccount') }}</span>
          </div>
          <h2 class="page-title">
            {{ t('common.myAccount') }}
          </h2>
          <div class="text-secondary">
            {{ t('account.manageAccountSubtitle') }}
          </div>
        </div>
      </div>
    </div>

    <!-- Tabs nav -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: activeTab === 'profil' }"
          @click="activeTab = 'profil'"
        >
          {{ t('account.profileTab') }}
        </button>
      </li>
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: activeTab === 'historique' }"
          @click="switchToHistorique"
        >
          {{ t('account.historyTab') }}
          <span
            v-if="myCommands.length"
            class="badge bg-azure-lt text-azure ms-1"
          >{{ myCommands.length }}</span>
        </button>
      </li>
      <li class="nav-item">
        <router-link
          to="/account/security"
          class="nav-link"
        >
          {{ t('account.connectionsTab') }}
        </router-link>
      </li>
    </ul>

    <!-- ── Onglet Profil ── -->
    <div v-show="activeTab === 'profil'">
      <div class="row g-4">
        <!-- Profile info card -->
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-body text-center py-4">
              <div
                class="avatar avatar-xl mb-3"
                style="width:64px;height:64px;font-size:1.6rem;background:var(--tblr-azure-lt);color:var(--tblr-azure);border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto;"
              >
                {{ auth.username?.slice(0, 2).toUpperCase() }}
              </div>
              <div class="h3 mb-1">
                {{ profile?.username || auth.username }}
              </div>
              <div class="mb-3">
                <span
                  class="badge"
                  :class="roleBadgeClass"
                >{{ roleLabel }}</span>
              </div>
              <div
                v-if="profile?.created_at"
                class="text-secondary small"
              >
                {{ t('account.memberSinceLabel', { date: formatDate(profile.created_at) }) }}
              </div>
            </div>
            <div class="card-footer text-center py-3">
              <div class="row g-3">
                <div class="col-6 border-end">
                  <div class="text-secondary small">
                    {{ t('account.mfaLabel') }}
                  </div>
                  <div
                    class="fw-bold"
                    :class="profile?.mfa_enabled ? 'text-success' : 'text-secondary'"
                  >
                    {{ profile?.mfa_enabled ? t('account.enabledWord') : t('account.disabledWord') }}
                  </div>
                </div>
                <div class="col-6">
                  <div class="text-secondary small">
                    {{ t('common.status') }}
                  </div>
                  <div class="fw-bold text-success">
                    {{ t('account.activeWord') }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- MFA card -->
          <div class="card mt-4">
            <div class="card-header">
              <h3 class="card-title">
                <IconLock
                  :size="24"
                  class="icon me-2"
                />
                {{ t('account.twoFactorAuthTitle') }}
              </h3>
            </div>
            <div class="card-body">
              <div class="d-flex align-items-center justify-content-between mb-3">
                <div>
                  <div class="fw-bold">
                    TOTP (Authenticator)
                  </div>
                  <div class="text-secondary small">
                    Google Authenticator, Authy, etc.
                  </div>
                </div>
                <span
                  class="badge"
                  :class="profile?.mfa_enabled ? 'bg-success-lt text-success' : 'bg-warning-lt text-warning'"
                >
                  {{ profile?.mfa_enabled ? t('account.activeWord') : t('account.inactiveWord') }}
                </span>
              </div>
              <router-link
                to="/account/security"
                class="btn btn-outline-secondary w-100"
              >
                {{ t('account.manageMfaButton') }}
              </router-link>
            </div>
          </div>
        </div>

        <!-- Change password -->
        <div class="col-12 col-lg-8">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title">
                <IconKey
                  :size="24"
                  class="icon me-2"
                />
                {{ t('account.changePasswordTitle') }}
              </h3>
            </div>
            <div class="card-body">
              <form @submit.prevent="submitChangePassword">
                <div class="mb-3">
                  <label class="form-label required">{{ t('account.currentPasswordLabel') }}</label>
                  <input
                    v-model="pwForm.current"
                    type="password"
                    class="form-control"
                    :class="{ 'is-invalid': pwErrors.current }"
                    placeholder="••••••••"
                    required
                  >
                  <div
                    v-if="pwErrors.current"
                    class="invalid-feedback"
                  >
                    {{ pwErrors.current }}
                  </div>
                </div>
                <div class="mb-3">
                  <label class="form-label required">{{ t('account.newPasswordLabel') }}</label>
                  <input
                    v-model="pwForm.next"
                    type="password"
                    class="form-control"
                    :class="{ 'is-invalid': pwErrors.next }"
                    placeholder="••••••••"
                    required
                  >
                  <div
                    v-if="pwErrors.next"
                    class="invalid-feedback"
                  >
                    {{ pwErrors.next }}
                  </div>
                  <div
                    v-if="pwStrengthMeta"
                    class="mt-2"
                  >
                    <div
                      class="progress"
                      style="height: 4px;"
                    >
                      <div
                        class="progress-bar"
                        :class="pwStrengthMeta.cls"
                        :style="{ width: pwStrengthMeta.width, transition: 'width 0.3s' }"
                      />
                    </div>
                    <div class="form-hint mt-1">
                      {{ t('account.passwordStrengthPrefix') }} <span :class="{ 'text-danger': pwStrength <= 1, 'text-warning': pwStrength === 2, 'text-success': pwStrength >= 4 }">{{ pwStrengthMeta.label }}</span>
                    </div>
                  </div>
                  <div
                    v-else
                    class="form-hint"
                  >
                    {{ t('account.atLeast8CharsHint') }}
                  </div>
                </div>
                <div class="mb-4">
                  <label class="form-label required">{{ t('account.confirmNewPasswordLabel') }}</label>
                  <input
                    v-model="pwForm.confirm"
                    type="password"
                    class="form-control"
                    :class="{ 'is-invalid': pwErrors.confirm }"
                    placeholder="••••••••"
                    required
                  >
                  <div
                    v-if="pwErrors.confirm"
                    class="invalid-feedback"
                  >
                    {{ pwErrors.confirm }}
                  </div>
                </div>

                <div
                  v-if="pwError"
                  class="alert alert-danger mb-3"
                  role="alert"
                >
                  {{ pwError }}
                </div>
                <div
                  v-if="pwSuccess"
                  class="alert alert-success mb-3"
                  role="alert"
                >
                  {{ pwSuccess }}
                </div>

                <div class="d-flex gap-2">
                  <button
                    type="submit"
                    class="btn btn-primary"
                    :disabled="pwLoading"
                  >
                    <span
                      v-if="pwLoading"
                      class="spinner-border spinner-border-sm me-2"
                    />
                    {{ pwLoading ? t('common.saving') : t('account.updatePasswordButton') }}
                  </button>
                  <button
                    v-if="!auth.mustChangePassword"
                    type="button"
                    class="btn btn-outline-secondary"
                    @click="resetPwForm"
                  >
                    {{ t('common.cancel') }}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ── Onglet Historique ── -->
    <div
      v-show="activeTab === 'historique'"
      class="side-layout"
    >
      <!-- Table principale -->
      <div class="side-main">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title mb-0">
              <IconClock
                :size="24"
                class="icon me-2"
              />
              {{ t('account.recentActivityTitle') }}
            </h3>
            <span
              v-if="myCommands.length"
              class="badge bg-azure-lt text-azure"
            >{{ myCommands.length }}</span>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>{{ t('account.dateColumn') }}</th>
                  <th>{{ t('account.hostColumnLabel') }}</th>
                  <th>{{ t('account.typeColumn') }}</th>
                  <th>{{ t('account.commandColumn') }}</th>
                  <th>{{ t('common.status') }}</th>
                  <th>{{ t('account.durationColumn') }}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-if="cmdsLoading">
                  <td
                    colspan="7"
                    class="py-2"
                  >
                    <LoadingSkeleton
                      variant="table"
                      :lines="4"
                    />
                  </td>
                </tr>
                <tr v-else-if="!myCommands.length">
                  <td colspan="7">
                    <EmptyState :title="t('account.noRecentActivityTitle')" />
                  </td>
                </tr>
                <tr
                  v-for="cmd in myCommands"
                  :key="cmd.id"
                  :class="{ 'table-active': selectedCmd?.id === cmd.id }"
                >
                  <td class="text-secondary small">
                    {{ formatDateTime(cmd.created_at) }}
                  </td>
                  <td>
                    <router-link
                      :to="`/hosts/${cmd.host_id}`"
                      class="text-decoration-none fw-semibold"
                    >
                      {{ cmd.host_name || cmd.host_id }}
                    </router-link>
                  </td>
                  <td><span :class="moduleClass(cmd.module)">{{ moduleLabel(cmd.module) }}</span></td>
                  <td><code class="small">{{ cmdLabel(cmd) }}</code></td>
                  <td><span :class="statusClass(cmd.status)">{{ commandStatusLabel(cmd.status) }}</span></td>
                  <td class="text-secondary small">
                    {{ formatDuration(cmd.started_at, cmd.ended_at) }}
                  </td>
                  <td>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      :disabled="!cmd.output && cmd.status === 'pending'"
                      :title="t('account.viewLogsTooltip')"
                      @click="openLogViewer(cmd)"
                    >
                      <IconFileText :size="14" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="selectedCmd"
        :show="showConsole"
        wrapper-class="side-panel"
        :title="t('account.consoleTitle')"
        :empty-text="t('account.noActiveConsoleText')"
        @close="closeLogViewer"
        @open="showConsole = true"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconAlertTriangle, IconClock, IconFileText, IconKey, IconLock } from '@tabler/icons-vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import { commandStatusLabel } from '../utils/commandStatus'
import { useAccount } from '../composables/useAccount'

const { t } = useI18n()

const {
  auth,
  activeTab,
  showConsole,
  profile,
  pwForm,
  pwErrors,
  pwError,
  pwSuccess,
  pwLoading,
  pwStrength,
  pwStrengthMeta,
  cmdsLoading,
  myCommands,
  selectedCmd,
  roleBadgeClass,
  roleLabel,
  formatDate,
  formatDateTime,
  formatDuration,
  cmdLabel,
  statusClass,
  moduleLabel,
  moduleClass,
  openLogViewer,
  closeLogViewer,
  resetPwForm,
  submitChangePassword,
  switchToHistorique,
} = useAccount()
</script>

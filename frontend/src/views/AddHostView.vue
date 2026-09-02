<template>
  <div>
    <div class="page-header mb-4">
      <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              {{ t('nav.sections.control.items.dashboard') }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>{{ t('host.addHostTitle') }}</span>
          </div>
          <h2 class="page-title">
            {{ t('host.addHostTitle') }}
          </h2>
          <div class="text-secondary">
            {{ t('host.registerNewHostDesc') }}
          </div>
        </div>
        <router-link
          to="/"
          class="btn btn-outline-secondary"
        >
          {{ t('host.backToDashboard') }}
        </router-link>
      </div>
    </div>

    <div class="row justify-content-center">
      <div class="col-12 col-md-8 col-lg-6">
        <ul
          v-if="!result"
          class="nav nav-pills mb-3"
        >
          <li class="nav-item">
            <button
              type="button"
              class="nav-link"
              :class="{ active: mode === 'manual' }"
              @click="mode = 'manual'"
            >
              {{ t('host.manualAddTab') }}
            </button>
          </li>
          <li class="nav-item">
            <button
              type="button"
              class="nav-link"
              :class="{ active: mode === 'scan' }"
              @click="mode = 'scan'"
            >
              {{ t('host.scanSubnetTab') }}
            </button>
          </li>
        </ul>

        <div
          v-if="mode === 'scan' && !result"
          class="card"
        >
          <div class="card-body">
            <NetworkDiscoveryPanel @done="finishAdd" />
          </div>
        </div>

        <div
          v-else
          class="card"
        >
          <div class="card-body">
            <form
              v-if="!result"
              @submit.prevent="handleSubmit"
            >
              <div class="mb-3">
                <label
                  class="form-label"
                  for="host-name"
                >{{ t('host.nameAliasLabel') }}</label>
                <input
                  id="host-name"
                  v-model="form.name"
                  type="text"
                  :class="['form-control', touched.name && !form.name.trim() ? 'is-invalid' : '']"
                  required
                  :placeholder="t('host.namePlaceholder')"
                  @blur="touched.name = true"
                >
                <div
                  v-if="touched.name && !form.name.trim()"
                  class="invalid-feedback"
                >
                  {{ t('host.fieldRequired') }}
                </div>
              </div>
              <div class="mb-3">
                <label
                  class="form-label"
                  for="host-ip"
                >{{ t('host.ipAddressLabel') }}</label>
                <div class="input-group">
                  <input
                    id="host-ip"
                    v-model="form.ip_address"
                    type="text"
                    :class="['form-control', touched.ip_address && isValidIp === false ? 'is-invalid' : touched.ip_address && isValidIp ? 'is-valid' : '']"
                    required
                    placeholder="192.168.1.100"
                    @blur="touched.ip_address = true"
                  >
                  <button
                    type="button"
                    class="btn btn-outline-secondary"
                    :class="{ active: showGuestPicker }"
                    :title="t('host.choosePveHostTooltip')"
                    @click="toggleGuestPicker"
                  >
                    <IconServer2 :size="16" />
                  </button>
                </div>
                <div
                  v-if="ipFeedback"
                  class="invalid-feedback d-block"
                >
                  {{ ipFeedback }}
                </div>

                <div
                  v-if="showGuestPicker"
                  class="guest-picker mt-2"
                >
                  <input
                    v-model="guestSearch"
                    type="text"
                    class="form-control form-control-sm mb-2"
                    :placeholder="t('host.searchPveHostPlaceholder')"
                    autofocus
                  >
                  <div
                    v-if="guestsLoading"
                    class="text-secondary small py-2"
                  >
                    {{ t('host.loadingPveHosts') }}
                  </div>
                  <div
                    v-else-if="guestsError"
                    class="text-danger small py-2"
                  >
                    {{ guestsError }}
                  </div>
                  <div
                    v-else-if="!filteredGuestIPOptions.length"
                    class="text-secondary small py-2"
                  >
                    {{ t('host.noPveHostsAvailable') }}
                  </div>
                  <div
                    v-else
                    class="guest-picker-list"
                  >
                    <button
                      v-for="option in filteredGuestIPOptions"
                      :key="`${option.guest.guest_id}-${option.ip}`"
                      type="button"
                      class="guest-picker-item"
                      @click="pickGuestIP(option)"
                    >
                      <span class="fw-medium">{{ option.guest.name }}</span>
                      <span class="text-secondary small">
                        {{ option.guest.node }} · {{ option.guest.guest_type === 'lxc' ? 'LXC' : 'VM' }}
                      </span>
                      <span class="guest-picker-ip">{{ option.ip }}</span>
                    </button>
                  </div>
                </div>
              </div>
              <div class="mb-3">
                <label
                  class="form-label"
                  for="host-tags"
                >{{ t('host.tagsOptionalLabel') }}</label>
                <input
                  id="host-tags"
                  v-model="form.tags"
                  type="text"
                  class="form-control"
                  placeholder="prod, site-lyon"
                >
                <div class="form-hint">
                  {{ t('host.tagsHintFilter') }}
                </div>
              </div>

              <div
                v-if="error"
                class="alert alert-danger"
                role="alert"
              >
                {{ error }}
              </div>

              <div class="text-secondary small mb-3">
                {{ t('host.osHostnameAutoNote') }}
              </div>

              <button
                type="submit"
                class="btn btn-primary w-100"
                :disabled="loading"
              >
                {{ loading ? t('host.savingLabel') : t('host.registerHostButton') }}
              </button>
            </form>

            <div
              v-else
              class="host-success"
              role="alert"
            >
              <div class="host-success-header">
                <div>
                  <div class="fw-semibold">
                    {{ t('host.hostRegisteredSuccess') }}
                  </div>
                  <div class="text-secondary small">
                    {{ t('host.keyNotShownAgain') }}
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-success"
                  @click="finishAdd"
                >
                  {{ t('host.doneLabel') }}
                </button>
              </div>

              <!-- Agent connection status -->
              <div
                class="agent-connection-status mb-3"
                :class="agentConnected ? 'agent-connected' : 'agent-waiting'"
              >
                <template v-if="agentConnected">
                  <IconCircleCheck
                    :size="16"
                    :stroke-width="2.5"
                  />
                  {{ t('host.agentConnectedMsg') }}
                </template>
                <template v-else>
                  <span
                    class="spinner-border spinner-border-sm"
                    style="width:.8rem;height:.8rem;border-width:2px"
                  />
                  {{ t('host.waitingFirstReportMsg') }}
                </template>
              </div>

              <div class="host-success-card mb-3">
                <div class="d-flex align-items-center justify-content-between mb-2">
                  <div class="text-secondary small">
                    {{ t('host.oneCommandInstallLabel') }}
                  </div>
                  <button
                    type="button"
                    class="btn btn-outline-secondary btn-sm"
                    @click="copyInstallCmd"
                  >
                    {{ copiedInstall ? t('host.copiedLabel') : t('host.copyLabel') }}
                  </button>
                </div>
                <pre class="host-success-config host-success-install">{{ installCmd }}</pre>
              </div>

              <div class="host-success-grid">
                <div class="host-success-card">
                  <div class="text-secondary small mb-2">
                    {{ t('host.agentApiKeyLabel') }}
                  </div>
                  <div class="host-success-key">
                    <code>{{ result.api_key }}</code>
                    <button
                      type="button"
                      class="btn btn-outline-secondary btn-sm"
                      @click="copyApiKey"
                    >
                      {{ copiedApiKey ? t('host.copiedLabel') : t('host.copyLabel') }}
                    </button>
                  </div>
                </div>
                <div class="host-success-card">
                  <div class="d-flex align-items-center justify-content-between mb-2">
                    <div class="text-secondary small">
                      {{ t('host.agentConfigYamlLabel') }}
                    </div>
                    <button
                      type="button"
                      class="btn btn-outline-secondary btn-sm"
                      @click="copyAgentConfig"
                    >
                      {{ copiedConfig ? t('host.copiedLabel') : t('host.copyLabel') }}
                    </button>
                  </div>
                  <pre class="host-success-config">{{ agentConfig }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconCircleCheck, IconServer2 } from '@tabler/icons-vue'
import { useAddHost } from '../composables/useAddHost'
import NetworkDiscoveryPanel from '../components/host/NetworkDiscoveryPanel.vue'

const { t } = useI18n()

const mode = ref<'manual' | 'scan'>('manual')

const {
  form,
  error,
  loading,
  touched,
  isValidIp,
  ipFeedback,
  showGuestPicker,
  guestsLoading,
  guestsError,
  guestSearch,
  filteredGuestIPOptions,
  toggleGuestPicker,
  pickGuestIP,
  result,
  copiedApiKey,
  copiedConfig,
  copiedInstall,
  agentConnected,
  installCmd,
  agentConfig,
  handleSubmit,
  copyApiKey,
  copyAgentConfig,
  copyInstallCmd,
  finishAdd,
} = useAddHost()
</script>

<style scoped>
.guest-picker {
  background: var(--ss-panel-strong);
  border: 1px solid var(--ss-border-default);
  border-radius: 10px;
  padding: 10px;
}

.guest-picker-list {
  max-height: 220px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.guest-picker-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  border: 0;
  background: transparent;
  padding: 0.4rem 0.5rem;
  border-radius: 6px;
  text-align: left;
  color: var(--tblr-body-color);
}

.guest-picker-item:hover {
  background: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.1);
}

.guest-picker-ip {
  margin-left: auto;
  font-family: var(--tblr-font-monospace, monospace);
  font-size: 0.8rem;
  color: var(--tblr-secondary);
  flex-shrink: 0;
}

.host-success {
  background: var(--ss-panel-medium);
  border: 1px solid rgba(56, 189, 248, 0.35);
  border-radius: 14px;
  padding: 20px;
  color: var(--ss-text-on-dark);
}

.host-success-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.host-success-grid {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(280px, 1.4fr);
  gap: 16px;
}

.host-success-card {
  background: var(--ss-panel-strong);
  border: 1px solid var(--ss-border-default);
  border-radius: 12px;
  padding: 14px;
}

.host-success-key {
  display: flex;
  align-items: center;
  gap: 10px;
}

.host-success-key code {
  display: block;
  background: rgba(2, 6, 23, 0.6);
  color: var(--ss-text-strong);
  padding: 8px 10px;
  border-radius: 8px;
  flex: 1;
  word-break: break-all;
}

.host-success-config {
  background: rgba(2, 6, 23, 0.6);
  color: var(--ss-text-on-dark);
  border-radius: 10px;
  padding: 10px;
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.host-success-install {
  border-left: 3px solid rgba(56, 189, 248, 0.6);
  color: var(--ss-accent-blue-text);
}

.agent-connection-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 500;
}

.agent-waiting {
  background: rgba(99, 102, 241, 0.1);
  border: 1px solid rgba(99, 102, 241, 0.3);
  color: var(--ss-accent-blue-text);
}

.agent-connected {
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.35);
  color: var(--ss-success-text);
}

@media (max-width: 991px) {
  .host-success-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .host-success-grid {
    grid-template-columns: 1fr;
  }
}
</style>

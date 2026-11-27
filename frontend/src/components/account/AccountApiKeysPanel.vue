<template>
  <div>
    <div class="card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title mb-0">
          <IconKey
            :size="24"
            class="icon me-2"
          />
          {{ t('account.apiKeysTitle') }}
        </h3>
        <button
          type="button"
          class="btn btn-sm btn-primary"
          @click="openCreate"
        >
          <IconPlus
            :size="14"
            class="icon me-1"
          />
          {{ t('account.apiKeyCreateButton') }}
        </button>
      </div>
      <div class="card-body">
        <p class="text-secondary mb-2">
          {{ t('account.apiKeysExplanation') }}
        </p>
        <p class="text-secondary small mb-2">
          {{ t('account.apiKeysRevocationHint') }}
        </p>
        <p class="text-secondary small mb-3">
          {{ t('account.apiKeysUsageHint') }}
          <code>Authorization: Bearer ssk_…</code>
          ·
          <a
            href="/api/v1/openapi.json"
            target="_blank"
            rel="noopener"
            class="text-decoration-none"
          >{{ t('account.apiKeysSpecLink') }}</a>
        </p>

        <div
          v-if="error"
          class="alert alert-danger"
          role="alert"
        >
          {{ error }}
        </div>
        <div
          v-if="success"
          class="alert alert-success"
          role="status"
        >
          {{ success }}
        </div>

        <LoadingSkeleton
          v-if="loading && !keys.length"
          variant="list"
        />
        <EmptyState
          v-else-if="!keys.length"
          :title="t('account.apiKeysEmptyTitle')"
          :subtitle="t('account.apiKeysEmptyDescription')"
        />
        <div
          v-else
          class="table-responsive"
        >
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>{{ t('account.apiKeyColumnName') }}</th>
                <th>{{ t('account.apiKeyColumnPrefix') }}</th>
                <th>{{ t('account.apiKeyColumnScope') }}</th>
                <th>{{ t('account.apiKeyColumnLastUsed') }}</th>
                <th>{{ t('account.apiKeyColumnExpires') }}</th>
                <th>{{ t('account.apiKeyColumnStatus') }}</th>
                <th class="text-end">
                  <span class="visually-hidden">{{ t('account.apiKeyColumnActions') }}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="key in keys"
                :key="key.id"
                :class="{ 'text-secondary': apiKeyStatus(key) !== 'active' }"
              >
                <td>
                  <div class="fw-medium">
                    {{ key.name }}
                  </div>
                  <div class="small text-secondary text-nowrap">
                    {{ t('account.addedOnLabel', { date: formatExactDate(key.created_at) }) }}
                  </div>
                </td>
                <td><code>{{ key.prefix }}</code></td>
                <td>
                  <span
                    class="badge"
                    :class="key.scope === 'write' ? 'bg-warning-lt text-warning' : 'bg-secondary-lt text-secondary'"
                  >{{ key.scope === 'write' ? t('account.apiKeyScopeWrite') : t('account.apiKeyScopeRead') }}</span>
                </td>
                <td class="small">
                  {{ key.last_used_at ? formatRelativeTime(key.last_used_at) : t('account.apiKeyNeverUsed') }}
                </td>
                <td class="small">
                  {{ key.expires_at ? formatExactDate(key.expires_at) : t('account.apiKeyNoExpiry') }}
                </td>
                <td>
                  <span
                    class="badge"
                    :class="statusClass[apiKeyStatus(key)]"
                  >{{ t(statusLabel[apiKeyStatus(key)]) }}</span>
                </td>
                <td class="text-end">
                  <button
                    v-if="!key.revoked_at"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    :title="t('account.apiKeyRevokeButton')"
                    :aria-label="t('account.apiKeyRevokeAriaLabel', { name: key.name })"
                    @click="revoke(key)"
                  >
                    <IconTrash :size="16" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Creation dialog: form, then the one-time secret -->
    <div
      v-if="showCreate"
      ref="modalRef"
      class="modal modal-blur fade show d-block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="`${uid}-title`"
      @click.self="!secret && closeCreate()"
    >
      <div class="modal-dialog modal-dialog-centered">
        <form
          class="modal-content"
          @submit.prevent="create"
        >
          <div class="modal-header">
            <h5
              :id="`${uid}-title`"
              class="modal-title"
            >
              {{ secret ? t('account.apiKeyCreatedTitle') : t('account.apiKeyCreateTitle') }}
            </h5>
            <button
              v-if="!secret"
              type="button"
              class="btn-close"
              :aria-label="t('common.close')"
              @click="closeCreate"
            />
          </div>

          <div
            v-if="!secret"
            class="modal-body"
          >
            <div class="mb-3">
              <label
                :for="`${uid}-name`"
                class="form-label required"
              >{{ t('account.apiKeyNameLabel') }}</label>
              <input
                :id="`${uid}-name`"
                v-model="name"
                type="text"
                class="form-control"
                maxlength="100"
                required
                :placeholder="t('account.apiKeyNamePlaceholder')"
              >
            </div>
            <div class="mb-3">
              <label
                :for="`${uid}-scope`"
                class="form-label"
              >{{ t('account.apiKeyScopeLabel') }}</label>
              <select
                :id="`${uid}-scope`"
                v-model="scope"
                class="form-select"
              >
                <option value="read">
                  {{ t('account.apiKeyScopeRead') }}
                </option>
                <option value="write">
                  {{ t('account.apiKeyScopeWrite') }}
                </option>
              </select>
              <div class="form-hint">
                {{ scope === 'write' ? t('account.apiKeyScopeWriteHint') : t('account.apiKeyScopeReadHint') }}
              </div>
              <div
                v-if="adminWrite"
                class="alert alert-warning mt-2 mb-0"
                role="alert"
              >
                {{ t('account.apiKeyAdminWriteWarning') }}
              </div>
            </div>
            <div class="mb-1">
              <label
                :for="`${uid}-expiry`"
                class="form-label"
              >{{ t('account.apiKeyExpiryLabel') }}</label>
              <select
                :id="`${uid}-expiry`"
                v-model.number="expiresInDays"
                class="form-select"
              >
                <option
                  v-for="days in API_KEY_EXPIRY_OPTIONS"
                  :key="days"
                  :value="days"
                >
                  {{ days === 0 ? t('account.apiKeyExpiryNever') : t('account.apiKeyExpiryDays', { count: days }, days) }}
                </option>
              </select>
            </div>
            <div
              v-if="createError"
              class="alert alert-danger mt-3 mb-0"
              role="alert"
            >
              {{ createError }}
            </div>
          </div>

          <div
            v-else
            class="modal-body"
          >
            <div
              class="alert alert-warning"
              role="alert"
            >
              {{ t('account.apiKeySecretWarning') }}
            </div>
            <label
              :for="`${uid}-secret`"
              class="form-label"
            >{{ t('account.apiKeySecretLabel') }}</label>
            <div class="input-group">
              <input
                :id="`${uid}-secret`"
                :value="secret"
                type="text"
                class="form-control font-monospace"
                autocomplete="off"
                spellcheck="false"
                readonly
                @focus="($event.target as HTMLInputElement).select()"
              >
              <button
                type="button"
                class="btn btn-outline-secondary"
                @click="copySecret"
              >
                <IconCheck
                  v-if="copied"
                  :size="16"
                  class="icon me-1"
                />
                <IconCopy
                  v-else
                  :size="16"
                  class="icon me-1"
                />
                {{ copied ? t('account.apiKeyCopiedLabel') : t('account.apiKeyCopyButton') }}
              </button>
            </div>
            <div
              class="visually-hidden"
              aria-live="polite"
            >
              {{ copied ? t('account.apiKeyCopiedLabel') : '' }}
            </div>
          </div>

          <div class="modal-footer">
            <template v-if="!secret">
              <button
                type="button"
                class="btn btn-outline-secondary"
                @click="closeCreate"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="!name.trim() || creating"
              >
                <span
                  v-if="creating"
                  class="spinner-border spinner-border-sm me-1"
                />
                {{ t('account.apiKeyCreateSubmit') }}
              </button>
            </template>
            <button
              v-else
              type="button"
              class="btn btn-primary"
              @click="closeCreate"
            >
              {{ t('account.apiKeySecretDone') }}
            </button>
          </div>
        </form>
      </div>
    </div>
    <div
      v-if="showCreate"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconCheck, IconCopy, IconKey, IconPlus, IconTrash } from '@tabler/icons-vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { API_KEY_EXPIRY_OPTIONS, apiKeyStatus, useApiKeys } from '../../composables/useApiKeys'
import type { ApiKeyStatus } from '../../composables/useApiKeys'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { useModalChrome } from '../../composables/useModalChrome'
import { useAuthStore } from '../../stores/auth'

const { t } = useI18n()
const uid = useId()
const { formatExactDate, formatRelativeTime } = useDateFormatter()

const {
  keys,
  loading,
  error,
  success,
  showCreate,
  name,
  scope,
  expiresInDays,
  creating,
  createError,
  secret,
  copied,
  load,
  openCreate,
  closeCreate,
  create,
  copySecret,
  revoke,
} = useApiKeys()

const auth = useAuthStore()
// A write key inherits every action the owner's role allows: for an admin
// that is the whole fleet, hence the explicit warning.
const adminWrite = computed(() => auth.isAdmin && scope.value === 'write')

const statusClass: Record<ApiKeyStatus, string> = {
  active: 'bg-success-lt text-success',
  expired: 'bg-warning-lt text-warning',
  revoked: 'bg-secondary-lt text-secondary',
}
const statusLabel: Record<ApiKeyStatus, string> = {
  active: 'account.apiKeyStatusActive',
  expired: 'account.apiKeyStatusExpired',
  revoked: 'account.apiKeyStatusRevoked',
}

const modalRef = ref<HTMLElement | null>(null)
const mounted = ref(false)
// The secret step is persistent: ESC must not discard a key the user has not
// copied yet; only the explicit "done" button closes it.
useModalChrome(modalRef, () => mounted.value && showCreate.value, {
  onClose: closeCreate,
  persistent: () => secret.value !== '',
  autoFocus: 'first-input',
})

onMounted(() => {
  mounted.value = true
  void load()
})
</script>

<template>
  <div class="page page-center">
    <div class="container container-tight py-4">
      <div class="text-center mb-4">
        <span class="h1">ServerSupervisor</span>
        <div class="text-secondary">
          {{ t('auth.subtitle') }}
        </div>
      </div>

      <form
        class="card card-md"
        @submit.prevent="handleLogin"
      >
        <div class="card-body">
          <h2 class="card-title text-center mb-4">
            {{ t('auth.signIn') }}
          </h2>
          <div class="mb-3">
            <label class="form-label">{{ t('common.user') }}</label>
            <input
              ref="usernameInput"
              v-model="username"
              type="text"
              class="form-control"
              placeholder="admin"
              name="username"
              autocomplete="username webauthn"
              required
              :disabled="loading || needsMFA"
            >
          </div>
          <div class="mb-3">
            <label class="form-label">{{ t('auth.password') }}</label>
            <input
              v-model="password"
              type="password"
              class="form-control"
              placeholder="••••••••"
              name="password"
              autocomplete="current-password"
              required
              :disabled="loading || needsMFA"
            >
          </div>

          <Transition name="slide-down">
            <div
              v-if="needsMFA"
              class="mb-3"
            >
              <label
                v-if="mfaMethods?.totp"
                class="form-label"
              >{{ t('auth.totpCode') }}</label>
              <input
                v-if="mfaMethods?.totp"
                ref="totpInput"
                v-model="totpCode"
                type="text"
                class="form-control"
                placeholder="123456"
                inputmode="numeric"
                maxlength="6"
                autocomplete="one-time-code"
                required
              >
              <div
                v-if="mfaMethods?.totp"
                class="text-secondary small mt-1"
              >
                {{ t('auth.totpHint') }}
              </div>

              <div
                v-if="mfaMethods?.totp && mfaMethods?.webauthn"
                class="text-secondary small text-center my-2"
              >
                {{ t('auth.or') }}
              </div>

              <button
                v-if="mfaMethods?.webauthn && webauthnAvailable"
                type="button"
                class="btn btn-outline-primary w-100"
                :disabled="webauthnLoading"
                @click="loginWithWebAuthn"
              >
                {{ webauthnLoading ? t('auth.verifying') : t('auth.useSecurityKey') }}
              </button>
            </div>
          </Transition>

          <div
            v-if="error"
            class="alert alert-danger"
            role="alert"
          >
            {{ error }}
          </div>

          <div
            v-if="!needsMFA || mfaMethods?.totp"
            class="form-footer"
          >
            <button
              type="submit"
              class="btn btn-primary w-100"
              :disabled="loading"
            >
              {{ loading ? t('auth.signingIn') : (needsMFA ? t('auth.verifyCode') : t('auth.signIn')) }}
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLogin } from '../composables/useLogin'

const { t } = useI18n()

const {
  username,
  password,
  error,
  loading,
  needsMFA,
  totpCode,
  totpFocusRequest,
  mfaMethods,
  webauthnAvailable,
  webauthnLoading,
  handleLogin,
  loginWithWebAuthn,
} = useLogin()

const usernameInput = ref<HTMLInputElement | null>(null)
const totpInput = ref<HTMLInputElement | null>(null)

onMounted(() => {
  usernameInput.value?.focus()
})

watch(totpFocusRequest, async () => {
  await nextTick()
  totpInput.value?.focus()
})
</script>

<style scoped>
.slide-down-enter-active,
.slide-down-leave-active {
  transition: all 0.25s ease;
}
.slide-down-enter-from,
.slide-down-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>

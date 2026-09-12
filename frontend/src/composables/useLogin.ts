import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { getApiErrorMessage } from '../api/client'
import type { MFAMethods } from '../types/webauthn'
import type { OIDCStatusResponse } from '../types/generated'
import { getWebAuthnAssertion, isConditionalMediationAvailable, isWebAuthnSupported } from '../utils/webauthn'

// Deliberately does not own the username/TOTP template refs or their focus
// management: those are DOM/template concerns specific to LoginView's own
// markup, not API/business logic — the view keeps them and just reacts to
// needsMFA from here.
export function useLogin() {
  const router = useRouter()
  const route = useRoute()
  const { t } = useI18n()
  const auth = useAuthStore()

  const username = ref('')
  const password = ref('')
  const error = ref('')
  const loading = ref(false)
  const needsMFA = ref(false)
  const totpCode = ref('')
  const oidcStatus = ref<OIDCStatusResponse | null>(null)
  const oidcLoading = ref(false)
  // Increments whenever the TOTP field should regain focus (prompt just
  // appeared, or a submitted code was rejected) — the view watches this and
  // owns the actual DOM focus() call on its own template ref.
  const totpFocusRequest = ref(0)

  // Which second factors this account has registered — set once the server's
  // require_mfa response tells us, so the view can offer a passkey button
  // only when one is actually usable (and only in a WebAuthn-capable browser).
  const mfaMethods = ref<MFAMethods | null>(null)
  const webauthnAvailable = isWebAuthnSupported()
  const webauthnLoading = ref(false)

  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- matches the pre-existing loose typing of the login response (see api/auth.ts's login()).
  function completeLogin(data: any): void {
    if (data?.role) {
      auth.setAuth(data, username.value)
      if (data.must_change_password) {
        router.push('/account')
      } else {
        router.push('/')
      }
    } else {
      error.value = t('auth.invalidLoginResponse')
    }
  }

  async function handleLogin(): Promise<void> {
    // The classic form is now committed to — free up the still-pending
    // conditional get() from startConditionalWebAuthn (a browsing context
    // allows only one navigator.credentials.get() in flight at a time; left
    // unaborted, a later loginWithWebAuthn() call below would fail with
    // "a request is already pending").
    abortConditionalWebAuthn()
    loading.value = true
    error.value = ''
    try {
      const { data } = await api.login(username.value, password.value, needsMFA.value ? totpCode.value : '')

      if (data?.require_mfa) {
        needsMFA.value = true
        mfaMethods.value = data.mfa_methods || null
        totpCode.value = ''
        totpFocusRequest.value++
        return
      }

      completeLogin(data)
    } catch (e: unknown) {
      if (needsMFA.value) {
        totpCode.value = ''
        totpFocusRequest.value++
        error.value = getApiErrorMessage(e, t('auth.invalidOrExpiredCode'))
      } else {
        error.value = getApiErrorMessage(e, t('auth.loginError'))
      }
    } finally {
      loading.value = false
    }
  }

  // Alternative to submitting a TOTP code: verifies the account's registered
  // passkey/security key instead. Re-sends username+password because the
  // require_mfa step never issued a session — the server re-checks them
  // itself (see BeginWebAuthnLogin's doc comment).
  async function loginWithWebAuthn(): Promise<void> {
    // Defensive: same conflict as in handleLogin, in case this is ever
    // reached without it having run first.
    abortConditionalWebAuthn()
    webauthnLoading.value = true
    error.value = ''
    try {
      const begin = await api.beginWebAuthnLogin(username.value, password.value)
      const credential = await getWebAuthnAssertion(begin.data.options)
      const { data } = await api.finishWebAuthnLogin(username.value, begin.data.session_token, credential)
      completeLogin(data)
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('auth.securityKeyVerificationFailed'))
    } finally {
      webauthnLoading.value = false
    }
  }

  // Usernameless "conditional UI" passkey login: fires as soon as the page
  // mounts, before the user has typed anything. navigator.credentials.get()
  // with mediation "conditional" doesn't pop a modal — it just makes the
  // browser offer a matching passkey as an autofill suggestion on the
  // username field (see LoginView's autocomplete="username webauthn"); the
  // returned promise only settles once the user actually picks one (or
  // aborts it, e.g. by navigating away). One-step login: no separate
  // username/password/MFA round trip if they do.
  let conditionalAbort: AbortController | null = null

  function abortConditionalWebAuthn(): void {
    conditionalAbort?.abort()
    conditionalAbort = null
  }

  async function startConditionalWebAuthn(): Promise<void> {
    if (!webauthnAvailable || !(await isConditionalMediationAvailable())) return
    conditionalAbort = new AbortController()
    try {
      const begin = await api.beginDiscoverableWebAuthnLogin()
      const credential = await getWebAuthnAssertion(begin.data.options, {
        mediation: 'conditional',
        signal: conditionalAbort.signal,
      })
      const { data } = await api.finishDiscoverableWebAuthnLogin(begin.data.session_token, credential)
      completeLogin(data)
    } catch (e: unknown) {
      // Aborted (unmount) or dismissed/failed by the user — the classic
      // username/password form underneath is still fully usable either way.
      if (e instanceof DOMException && e.name === 'AbortError') return
    }
  }

  function loginWithOIDC(): void {
    abortConditionalWebAuthn()
    const redirect = route.query.redirect ? `?return_to=${encodeURIComponent(String(route.query.redirect))}` : ''
    window.location.href = `/api/auth/oidc/login${redirect}`
  }

  onMounted(async () => {
    if (route.query.error) {
      const errParam = String(route.query.error)
      const errDesc = route.query.error_description ? `: ${String(route.query.error_description)}` : ''
      error.value = `${errParam}${errDesc}`
    }

    try {
      const { data } = await api.getOIDCStatus()
      oidcStatus.value = data
    } catch {
      // Ignored if OIDC status endpoint is unreachable
    }

    void startConditionalWebAuthn()
  })

  onUnmounted(abortConditionalWebAuthn)

  return {
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
    oidcStatus,
    oidcLoading,
    handleLogin,
    loginWithWebAuthn,
    loginWithOIDC,
  }
}

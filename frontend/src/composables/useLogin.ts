import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { getApiErrorMessage } from '../api/client'
import type { MFAMethods } from '../types/webauthn'
import { getWebAuthnAssertion, isWebAuthnSupported } from '../utils/webauthn'

// Deliberately does not own the username/TOTP template refs or their focus
// management: those are DOM/template concerns specific to LoginView's own
// markup, not API/business logic — the view keeps them and just reacts to
// needsMFA from here.
export function useLogin() {
  const router = useRouter()
  const auth = useAuthStore()

  const username = ref('')
  const password = ref('')
  const error = ref('')
  const loading = ref(false)
  const needsMFA = ref(false)
  const totpCode = ref('')
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
      error.value = 'Réponse de connexion invalide.'
    }
  }

  async function handleLogin(): Promise<void> {
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
        error.value = getApiErrorMessage(e, 'Code invalide ou expiré — générez un nouveau code.')
      } else {
        error.value = getApiErrorMessage(e, 'Erreur de connexion')
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
    webauthnLoading.value = true
    error.value = ''
    try {
      const begin = await api.beginWebAuthnLogin(username.value, password.value)
      const credential = await getWebAuthnAssertion(begin.data.options)
      const { data } = await api.finishWebAuthnLogin(username.value, begin.data.session_token, credential)
      completeLogin(data)
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Échec de la vérification de la clé de sécurité')
    } finally {
      webauthnLoading.value = false
    }
  }

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
    handleLogin,
    loginWithWebAuthn,
  }
}

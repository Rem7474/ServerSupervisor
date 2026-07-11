import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { getApiErrorMessage } from '../api/client'

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

  async function handleLogin(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const { data } = await api.login(username.value, password.value, needsMFA.value ? totpCode.value : '')

      if (data?.require_mfa) {
        needsMFA.value = true
        totpCode.value = ''
        totpFocusRequest.value++
        return
      }

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

  return {
    username,
    password,
    error,
    loading,
    needsMFA,
    totpCode,
    totpFocusRequest,
    handleLogin,
  }
}

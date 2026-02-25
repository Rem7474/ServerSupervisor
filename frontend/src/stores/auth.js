import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const role = ref(localStorage.getItem('role') || '')
  const username = ref(localStorage.getItem('username') || '')
  const mustChangePassword = ref(localStorage.getItem('mustChangePassword') === 'true')

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => role.value === 'admin')
  const isOperator = computed(() => role.value === 'operator')
  const isViewer = computed(() => role.value === 'viewer')

  function setAuth(data, user) {
    token.value = data.token
    role.value = data.role
    username.value = user
    mustChangePassword.value = !!data.must_change_password
    localStorage.setItem('token', data.token)
    localStorage.setItem('role', data.role)
    localStorage.setItem('username', user)
    localStorage.setItem('mustChangePassword', data.must_change_password ? 'true' : 'false')
  }

  function clearMustChangePassword() {
    mustChangePassword.value = false
    localStorage.setItem('mustChangePassword', 'false')
  }

  function logout() {
    token.value = ''
    role.value = ''
    username.value = ''
    mustChangePassword.value = false
    localStorage.removeItem('token')
    localStorage.removeItem('role')
    localStorage.removeItem('username')
    localStorage.removeItem('mustChangePassword')
  }

  return { token, role, username, mustChangePassword, isAuthenticated, isAdmin, isOperator, isViewer, setAuth, clearMustChangePassword, logout }
})

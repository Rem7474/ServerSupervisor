import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const role = ref(localStorage.getItem('role') || '')
  const username = ref(localStorage.getItem('username') || '')

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => role.value === 'admin')

  function setAuth(data, user) {
    token.value = data.token
    role.value = data.role
    username.value = user
    localStorage.setItem('token', data.token)
    localStorage.setItem('role', data.role)
    localStorage.setItem('username', user)
  }

  function logout() {
    token.value = ''
    role.value = ''
    username.value = ''
    localStorage.removeItem('token')
    localStorage.removeItem('role')
    localStorage.removeItem('username')
  }

  return { token, role, username, isAuthenticated, isAdmin, setAuth, logout }
})

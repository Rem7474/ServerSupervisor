import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const ROLE_PERMISSIONS = {
  admin: ['*'],
  operator: [
    'view:audit:commands',
    'manage:hosts',
    'manage:tasks',
    'manage:docker',
    'manage:apt',
    'view:alerts',
  ],
  viewer: [
    'view:dashboard',
    'view:hosts',
    'view:alerts',
    'view:network',
  ],
}

export const useAuthStore = defineStore('auth', () => {
  let persistedUser = null
  try {
    persistedUser = JSON.parse(localStorage.getItem('user') || 'null')
  } catch {
    persistedUser = null
  }

  const token = ref(localStorage.getItem('token') || '')
  const role = ref(localStorage.getItem('role') || '')
  const username = ref(localStorage.getItem('username') || '')
  const user = ref({
    username: persistedUser?.username || localStorage.getItem('username') || '',
    role: persistedUser?.role || localStorage.getItem('role') || '',
  })
  const mustChangePassword = ref(localStorage.getItem('mustChangePassword') === 'true')

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => role.value === 'admin')
  const isOperator = computed(() => role.value === 'operator')
  const isViewer = computed(() => role.value === 'viewer')
  const canManage = computed(() => role.value === 'admin' || role.value === 'operator')

  function setAuth(data, usernameValue) {
    token.value = data.token
    role.value = data.role
    username.value = usernameValue
    const nextUser = { username: usernameValue, role: data.role }
    user.value = nextUser
    mustChangePassword.value = !!data.must_change_password
    localStorage.setItem('token', data.token)
    localStorage.setItem('role', data.role)
    localStorage.setItem('username', usernameValue)
    localStorage.setItem('user', JSON.stringify(nextUser))
    localStorage.setItem('mustChangePassword', data.must_change_password ? 'true' : 'false')
  }

  /**
   * @param {string} permission
   * @returns {boolean}
   */
  function hasPermission(permission) {
    const permissions = ROLE_PERMISSIONS[role.value] || []
    return permissions.includes('*') || permissions.includes(permission)
  }

  function clearMustChangePassword() {
    mustChangePassword.value = false
    localStorage.setItem('mustChangePassword', 'false')
  }

  function logout() {
    token.value = ''
    role.value = ''
    username.value = ''
    user.value = { username: '', role: '' }
    mustChangePassword.value = false
    localStorage.removeItem('token')
    localStorage.removeItem('role')
    localStorage.removeItem('username')
    localStorage.removeItem('user')
    localStorage.removeItem('mustChangePassword')
  }

  return {
    token,
    role,
    username,
    user,
    mustChangePassword,
    isAuthenticated,
    isAdmin,
    isOperator,
    isViewer,
    canManage,
    hasPermission,
    setAuth,
    clearMustChangePassword,
    logout,
  }
})

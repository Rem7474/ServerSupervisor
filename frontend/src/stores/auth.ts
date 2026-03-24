import { defineStore } from 'pinia'
import { ref, computed, Ref, ComputedRef } from 'vue'

interface User {
  username: string
  role: string
}

interface AuthData {
  token: string
  role: string
  must_change_password?: boolean
}

interface RolePermissions {
  [key: string]: string[]
}

const ROLE_PERMISSIONS: RolePermissions = {
  admin: ['*'],
  operator: [
    'view:audit:commands',
    'manage:hosts',
    'manage:tasks',
    'manage:docker',
    'manage:apt',
    'view:alerts',
  ],
  viewer: ['view:dashboard', 'view:hosts', 'view:alerts', 'view:network'],
}

export const useAuthStore = defineStore('auth', () => {
  let persistedUser: User | null = null
  try {
    persistedUser = JSON.parse(localStorage.getItem('user') || 'null')
  } catch {
    persistedUser = null
  }

  const token: Ref<string> = ref(localStorage.getItem('token') || '')
  const role: Ref<string> = ref(localStorage.getItem('role') || '')
  const username: Ref<string> = ref(localStorage.getItem('username') || '')
  const user: Ref<User> = ref({
    username: persistedUser?.username || localStorage.getItem('username') || '',
    role: persistedUser?.role || localStorage.getItem('role') || '',
  })
  const mustChangePassword: Ref<boolean> = ref(localStorage.getItem('mustChangePassword') === 'true')

  const isAuthenticated: ComputedRef<boolean> = computed(() => !!token.value)
  const isAdmin: ComputedRef<boolean> = computed(() => role.value === 'admin')
  const isOperator: ComputedRef<boolean> = computed(() => role.value === 'operator')
  const isViewer: ComputedRef<boolean> = computed(() => role.value === 'viewer')
  const canManage: ComputedRef<boolean> = computed(() => role.value === 'admin' || role.value === 'operator')

  function setAuth(data: AuthData, usernameValue: string): void {
    token.value = data.token
    role.value = data.role
    username.value = usernameValue
    const nextUser: User = { username: usernameValue, role: data.role }
    user.value = nextUser
    mustChangePassword.value = !!data.must_change_password
    localStorage.setItem('token', data.token)
    localStorage.setItem('role', data.role)
    localStorage.setItem('username', usernameValue)
    localStorage.setItem('user', JSON.stringify(nextUser))
    localStorage.setItem('mustChangePassword', data.must_change_password ? 'true' : 'false')
  }

  /**
   * Check if the current user has a specific permission.
   * @param {string} permission - Permission to check (or '*' for admin)
   * @returns {boolean} - True if user has permission
   */
  function hasPermission(permission: string): boolean {
    const permissions = ROLE_PERMISSIONS[role.value] || []
    return permissions.includes('*') || permissions.includes(permission)
  }

  function clearMustChangePassword(): void {
    mustChangePassword.value = false
    localStorage.setItem('mustChangePassword', 'false')
  }

  function logout(): void {
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

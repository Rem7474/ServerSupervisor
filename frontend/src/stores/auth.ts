import { defineStore } from 'pinia'
import { ref, computed, Ref, ComputedRef } from 'vue'

interface User {
  username: string
  role: string
}

interface AuthData {
  role: string
  username?: string
  must_change_password?: boolean
  csrf_token?: string
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

/**
 * Read a non-httpOnly cookie value by name. Returns '' when absent.
 * Used to pick up the CSRF token cookie ss_csrf written by the backend on
 * login/refresh so we can echo it back in X-CSRF-Token.
 */
export function readCookie(name: string): string {
  const prefix = `${name}=`
  const parts = document.cookie ? document.cookie.split('; ') : []
  for (const part of parts) {
    if (part.startsWith(prefix)) {
      try {
        return decodeURIComponent(part.slice(prefix.length))
      } catch {
        return part.slice(prefix.length)
      }
    }
  }
  return ''
}

const CSRF_COOKIE = 'ss_csrf'

export const useAuthStore = defineStore('auth', () => {
  let persistedUser: User | null
  try {
    persistedUser = JSON.parse(localStorage.getItem('user') || 'null')
  } catch {
    persistedUser = null
  }

  // Only non-sensitive fields persist across reloads. The session is held by
  // the httpOnly cookie set by the server; the CSRF token is mirrored in a
  // readable cookie picked up by the axios interceptor.
  const role: Ref<string> = ref(localStorage.getItem('role') || persistedUser?.role || '')
  const username: Ref<string> = ref(localStorage.getItem('username') || persistedUser?.username || '')
  const user: Ref<User> = ref({
    username: username.value,
    role: role.value,
  })
  const mustChangePassword: Ref<boolean> = ref(localStorage.getItem('mustChangePassword') === 'true')

  const isAuthenticated: ComputedRef<boolean> = computed(() => !!role.value)
  const isAdmin: ComputedRef<boolean> = computed(() => role.value === 'admin')
  const isOperator: ComputedRef<boolean> = computed(() => role.value === 'operator')
  const isViewer: ComputedRef<boolean> = computed(() => role.value === 'viewer')
  const canManage: ComputedRef<boolean> = computed(() => role.value === 'admin' || role.value === 'operator')
  const csrfToken: ComputedRef<string> = computed(() => readCookie(CSRF_COOKIE))

  function setAuth(data: AuthData, usernameValue: string): void {
    role.value = data.role
    username.value = data.username || usernameValue
    const nextUser: User = { username: username.value, role: data.role }
    user.value = nextUser
    mustChangePassword.value = !!data.must_change_password
    localStorage.setItem('role', data.role)
    localStorage.setItem('username', username.value)
    localStorage.setItem('user', JSON.stringify(nextUser))
    localStorage.setItem('mustChangePassword', data.must_change_password ? 'true' : 'false')
  }

  /**
   * Check if the current user has a specific permission.
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
    role.value = ''
    username.value = ''
    user.value = { username: '', role: '' }
    mustChangePassword.value = false
    localStorage.removeItem('role')
    localStorage.removeItem('username')
    localStorage.removeItem('user')
    localStorage.removeItem('mustChangePassword')
    // Best-effort cleanup of legacy keys from the localStorage-based scheme.
    localStorage.removeItem('token')
    localStorage.removeItem('refreshToken')
  }

  return {
    role,
    username,
    user,
    mustChangePassword,
    isAuthenticated,
    isAdmin,
    isOperator,
    isViewer,
    canManage,
    csrfToken,
    hasPermission,
    setAuth,
    clearMustChangePassword,
    logout,
  }
})

import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore, readCookie } from './auth'

function setCookie(name: string, value: string): void {
  document.cookie = `${name}=${value}; path=/`
}

describe('stores/auth', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  describe('readCookie', () => {
    it('returns the decoded value of a matching cookie', () => {
      setCookie('ss_csrf', 'abc123')
      expect(readCookie('ss_csrf')).toBe('abc123')
    })

    it('returns an empty string when the cookie is absent', () => {
      expect(readCookie('nonexistent_cookie')).toBe('')
    })

    it('url-decodes the value', () => {
      setCookie('ss_csrf', encodeURIComponent('a b+c'))
      expect(readCookie('ss_csrf')).toBe('a b+c')
    })

    it('falls back to the raw value when decoding fails', () => {
      // "%" alone is not a valid escape sequence — decodeURIComponent throws.
      setCookie('ss_csrf', '%')
      expect(readCookie('ss_csrf')).toBe('%')
    })
  })

  describe('hydration on store creation', () => {
    it('starts logged out when localStorage is empty', () => {
      setActivePinia(createPinia())
      const store = useAuthStore()
      expect(store.role).toBe('')
      expect(store.isAuthenticated).toBe(false)
    })

    it('restores role/username directly from their own localStorage keys', () => {
      localStorage.setItem('role', 'admin')
      localStorage.setItem('username', 'alice')
      setActivePinia(createPinia())

      const store = useAuthStore()
      expect(store.role).toBe('admin')
      expect(store.username).toBe('alice')
      expect(store.isAdmin).toBe(true)
    })

    it('falls back to the persisted "user" blob when the role/username keys are absent', () => {
      localStorage.setItem('user', JSON.stringify({ role: 'operator', username: 'bob' }))
      setActivePinia(createPinia())

      const store = useAuthStore()
      expect(store.role).toBe('operator')
      expect(store.username).toBe('bob')
    })

    it('does not throw and starts logged out when the persisted "user" blob is corrupted', () => {
      localStorage.setItem('user', '{not json')
      setActivePinia(createPinia())

      let store: ReturnType<typeof useAuthStore> | undefined
      expect(() => { store = useAuthStore() }).not.toThrow()
      expect(store!.role).toBe('')
    })

    it('restores mustChangePassword from localStorage', () => {
      localStorage.setItem('mustChangePassword', 'true')
      setActivePinia(createPinia())

      const store = useAuthStore()
      expect(store.mustChangePassword).toBe(true)
    })
  })

  describe('setAuth / logout', () => {
    beforeEach(() => {
      setActivePinia(createPinia())
    })

    it('sets role/username/mustChangePassword and persists them to localStorage', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'admin', must_change_password: true }, 'alice')

      expect(store.role).toBe('admin')
      expect(store.username).toBe('alice')
      expect(store.mustChangePassword).toBe(true)
      expect(localStorage.getItem('role')).toBe('admin')
      expect(localStorage.getItem('username')).toBe('alice')
      expect(localStorage.getItem('mustChangePassword')).toBe('true')
      expect(JSON.parse(localStorage.getItem('user') || 'null')).toEqual({ username: 'alice', role: 'admin' })
    })

    it('prefers the username returned by the server over the one passed in', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'admin', username: 'server-name' }, 'login-name')
      expect(store.username).toBe('server-name')
    })

    it('logout clears state and every persisted key, including legacy ones', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'admin' }, 'alice')
      localStorage.setItem('token', 'legacy-token')
      localStorage.setItem('refreshToken', 'legacy-refresh')

      store.logout()

      expect(store.role).toBe('')
      expect(store.username).toBe('')
      expect(store.isAuthenticated).toBe(false)
      for (const key of ['role', 'username', 'user', 'mustChangePassword', 'token', 'refreshToken']) {
        expect(localStorage.getItem(key)).toBeNull()
      }
    })

    it('clearMustChangePassword flips the flag without touching role/username', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'admin', must_change_password: true }, 'alice')

      store.clearMustChangePassword()

      expect(store.mustChangePassword).toBe(false)
      expect(localStorage.getItem('mustChangePassword')).toBe('false')
      expect(store.role).toBe('admin')
    })
  })

  describe('hasPermission', () => {
    beforeEach(() => {
      setActivePinia(createPinia())
    })

    it('grants everything to admin via the wildcard', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'admin' }, 'alice')
      expect(store.hasPermission('manage:hosts')).toBe(true)
      expect(store.hasPermission('anything:not:listed')).toBe(true)
    })

    it('grants only the listed permissions to operator', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'operator' }, 'bob')
      expect(store.hasPermission('manage:hosts')).toBe(true)
      expect(store.hasPermission('manage:users')).toBe(false)
    })

    it('denies everything to an unrecognized role', () => {
      const store = useAuthStore()
      store.setAuth({ role: 'guest' }, 'eve')
      expect(store.hasPermission('view:dashboard')).toBe(false)
    })
  })
})

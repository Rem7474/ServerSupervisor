import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import UsersView from './UsersView.vue'

const mockGetUsers = vi.fn()
const mockCreateUser = vi.fn()
const mockUpdateUserRole = vi.fn()
const mockDeleteUser = vi.fn()

vi.mock('../api', () => ({
  default: {
    getUsers: () => mockGetUsers(),
    createUser: (...args: unknown[]) => mockCreateUser(...args),
    updateUserRole: (...args: unknown[]) => mockUpdateUserRole(...args),
    deleteUser: (...args: unknown[]) => mockDeleteUser(...args),
  },
}))

describe('views/UsersView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGetUsers.mockReset()
    mockCreateUser.mockReset()
    mockUpdateUserRole.mockReset()
    mockDeleteUser.mockReset()
  })

  it('renders user list with SSO badges and email', async () => {
    mockGetUsers.mockResolvedValueOnce({
      data: [
        {
          id: 1,
          username: 'local_admin',
          role: 'admin',
          auth_provider: 'local',
          created_at: '2026-01-01T00:00:00Z',
        },
        {
          id: 2,
          username: 'sso_user',
          role: 'operator',
          auth_provider: 'oidc',
          email: 'sso_user@example.com',
          created_at: '2026-01-02T00:00:00Z',
        },
      ],
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          RouterLink: true,
          PageRefreshBar: true,
          ConfirmDialog: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('local_admin')
    expect(wrapper.text()).toContain('sso_user')
    expect(wrapper.text()).toContain('sso_user@example.com')
    expect(wrapper.text()).toContain('SSO')
  })
})

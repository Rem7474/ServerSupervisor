import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AdminConfigurationView from './AdminConfigurationView.vue'
import { setLocale } from '../i18n'

const { mockSummary } = vi.hoisted(() => ({
  mockSummary: {
    total_params: 3,
    env_count: 1,
    ui_count: 1,
    default_count: 1,
    conflict_count: 1,
    categories: ['server', 'notifications', 'auth'],
    entries: [
      {
        key: 'SERVER_PORT',
        setting_key: 'server_port',
        env_var: 'SERVER_PORT',
        label: "Port d'écoute du serveur",
        description: 'Port TCP du serveur HTTP',
        category: 'server',
        type: 'int',
        default_value: '8080',
        effective_value: '8080',
        source: 'default',
        is_secret: false,
        is_editable: true,
        requires_restart: true,
        has_env_override: false,
        has_conflict: false,
      },
      {
        key: 'SMTP_HOST',
        setting_key: 'smtp_host',
        env_var: 'SMTP_HOST',
        label: 'Hôte SMTP',
        description: 'Serveur de messagerie',
        category: 'notifications',
        type: 'string',
        default_value: '',
        effective_value: 'smtp.env.corp',
        source: 'env',
        is_secret: false,
        is_editable: true,
        requires_restart: false,
        has_env_override: true,
        env_value: 'smtp.env.corp',
        ui_value: 'smtp.ui.corp',
        has_conflict: true,
      },
      {
        key: 'JWT_SECRET',
        setting_key: 'jwt_secret',
        env_var: 'JWT_SECRET',
        label: 'Secret de signature JWT',
        description: 'Clé secrète',
        category: 'auth',
        type: 'string',
        default_value: '',
        effective_value: '••••••••',
        source: 'ui',
        is_secret: true,
        is_editable: true,
        requires_restart: false,
        has_env_override: false,
        ui_value: 'my-custom-jwt-secret-key',
        has_conflict: false,
      },
    ],
  },
}))

vi.mock('../api/config', () => ({
  configApi: {
    getConfig: vi.fn().mockResolvedValue({ data: mockSummary }),
    updateParam: vi.fn().mockResolvedValue({ data: { success: true, message: 'Updated' } }),
    resetParam: vi.fn().mockResolvedValue({ data: { success: true, message: 'Reset' } }),
    updateBulk: vi.fn().mockResolvedValue({ data: { success: true, message: 'Bulk updated' } }),
  },
}))

vi.mock('../composables/useConfirmDialog', () => ({
  useConfirmDialog: () => ({
    confirm: vi.fn().mockResolvedValue(true),
  }),
}))

vi.mock('../composables/useGlobalToast', () => ({
  addToast: vi.fn(),
  useGlobalToast: () => ({
    toasts: [],
    addToast: vi.fn(),
    removeToast: vi.fn(),
  }),
}))

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
  vi.clearAllMocks()
})

describe('AdminConfigurationView', () => {
  it('renders page title and summary metrics', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Configuration Docker & Système')
    expect(wrapper.text()).toContain("Port d'écoute du serveur")
    expect(wrapper.text()).toContain('Hôte SMTP')
    expect(wrapper.text()).toContain('Secret de signature JWT')
  })

  it('displays conflict badge for conflicting entries', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Conflit ENV vs UI')
  })

  it('filters entries by search query', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const searchInput = wrapper.find('input[type="text"]')
    await searchInput.setValue('SMTP')

    expect(wrapper.text()).toContain('Hôte SMTP')
    expect(wrapper.text()).not.toContain("Port d'écoute du serveur")
  })

  it('filters entries by source', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const selects = wrapper.findAll('select')
    const sourceSelect = selects[selects.length - 1]
    await sourceSelect.setValue('conflict')

    expect(wrapper.text()).toContain('Hôte SMTP')
    expect(wrapper.text()).not.toContain("Port d'écoute du serveur")
  })

  it('filters entries by category', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const categorySelect = wrapper.find('#config-category-filter')
    await categorySelect.setValue('server')

    expect(wrapper.text()).toContain("Port d'écoute du serveur")
    expect(wrapper.text()).not.toContain('Hôte SMTP')
  })

  it('allows saving an edited parameter', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const input = wrapper.find('#input-SERVER_PORT')
    await input.setValue('9090')
    await flushPromises()

    const row = wrapper.findAll('tr').find(r => r.text().includes("Port d'écoute du serveur"))
    const saveBtn = row?.findAll('button').find(b => b.text().includes('Enregistrer'))
    expect(saveBtn).toBeDefined()
    await saveBtn?.trigger('click')
    await flushPromises()

    const { configApi } = await import('../api/config')
    expect(configApi.updateParam).toHaveBeenCalledWith('SERVER_PORT', '9090')
  })

  it('allows resetting a parameter to default/env', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const jwtRow = wrapper.findAll('tr').find(r => r.text().includes('Secret de signature JWT'))
    const resetBtn = jwtRow?.find('button.btn-outline-danger')
    expect(resetBtn?.exists()).toBe(true)
    await resetBtn?.trigger('click')
    await flushPromises()

    const { configApi } = await import('../api/config')
    expect(configApi.resetParam).toHaveBeenCalledWith('JWT_SECRET')
  })

  it('allows saving all modified parameters in a category', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const input = wrapper.find('#input-SERVER_PORT')
    await input.setValue('9090')
    await flushPromises()

    const categoryHeaderSave = wrapper.findAll('button').find(b => b.classes().includes('btn-primary') && b.text().includes('Enregistrer'))
    expect(categoryHeaderSave?.exists()).toBe(true)
    await categoryHeaderSave?.trigger('click')
    await flushPromises()

    const { configApi } = await import('../api/config')
    expect(configApi.updateBulk).toHaveBeenCalled()
  })

  it('toggles reveal secrets', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const revealBtn = wrapper.findAll('button').find(b => b.text().includes('Afficher secrets'))
    if (revealBtn) {
      await revealBtn.trigger('click')
      await flushPromises()
      const { configApi } = await import('../api/config')
      expect(configApi.getConfig).toHaveBeenCalledWith(true)
    }
  })

  it('toggles password visibility locally', async () => {
    const wrapper = mount(AdminConfigurationView, {
      global: {
        stubs: {
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const secretInput = wrapper.find('#input-JWT_SECRET')
    expect(secretInput.attributes('type')).toBe('password')

    const eyeBtn = wrapper.find('button[aria-label="Afficher secrets"], button[aria-label="Masquer"]')
    if (eyeBtn.exists()) {
      await eyeBtn.trigger('click')
      expect(wrapper.find('#input-JWT_SECRET').attributes('type')).toBe('text')
    }
  })
})


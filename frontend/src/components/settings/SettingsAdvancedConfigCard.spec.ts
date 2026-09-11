import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsAdvancedConfigCard from './SettingsAdvancedConfigCard.vue'
import { setLocale } from '../../i18n'

const { mockSummary } = vi.hoisted(() => ({
  mockSummary: {
    total_params: 4,
    env_count: 1,
    ui_count: 1,
    default_count: 2,
    conflict_count: 1,
    categories: ['server', 'network', 'auth', 'notifications'],
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
        // A key deliberately absent from config.json's params.* catalog —
        // exercises paramLabel/paramDescription's fallback to the raw
        // Go-sent text for a param added server-side before its translation
        // lands. Category "network" (relevant) rather than "notifications"
        // so the conflict/source/save-flow assertions below stay meaningful
        // — SMTP itself lives in "notifications", covered by the dedicated
        // "entry excluded" test instead.
        key: 'SOME_FUTURE_PARAM',
        setting_key: 'some_future_param',
        env_var: 'SOME_FUTURE_PARAM',
        label: 'Origines autorisées',
        description: 'CORS WebSocket',
        category: 'network',
        type: 'csv',
        default_value: '',
        effective_value: 'https://env.corp',
        source: 'env',
        is_secret: false,
        is_editable: true,
        requires_restart: false,
        has_env_override: true,
        env_value: 'https://env.corp',
        ui_value: 'https://ui.corp',
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
      {
        // Owned exclusively by SettingsSmtpCard — must never appear here.
        key: 'SMTP_HOST',
        setting_key: 'smtp_host',
        env_var: 'SMTP_HOST',
        label: 'Hôte SMTP',
        description: 'Serveur de messagerie',
        category: 'notifications',
        type: 'string',
        default_value: '',
        effective_value: '',
        source: 'default',
        is_secret: false,
        is_editable: true,
        requires_restart: false,
        has_env_override: false,
        has_conflict: false,
      },
    ],
  },
}))

vi.mock('../../api/config', () => ({
  configApi: {
    getConfig: vi.fn().mockResolvedValue({ data: mockSummary }),
    updateParam: vi.fn().mockResolvedValue({ data: { success: true, message: 'Updated' } }),
    resetParam: vi.fn().mockResolvedValue({ data: { success: true, message: 'Reset' } }),
    updateBulk: vi.fn().mockResolvedValue({ data: { success: true, message: 'Bulk updated' } }),
  },
}))

vi.mock('../../composables/useConfirmDialog', () => ({
  useConfirmDialog: () => ({
    confirm: vi.fn().mockResolvedValue(true),
  }),
}))

vi.mock('../../composables/useGlobalToast', () => ({
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

describe('SettingsAdvancedConfigCard', () => {
  it('translates a known param label/description instead of showing the raw Go-sent French', async () => {
    setLocale('en')
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    expect(wrapper.text()).toContain('Server listen port')
    expect(wrapper.text()).toContain('TCP port the server listens for HTTP requests on.')
    expect(wrapper.text()).not.toContain("Port d'écoute du serveur")
  })

  it('falls back to the raw Go-sent label/description for a param with no translation yet', async () => {
    setLocale('en')
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    // SOME_FUTURE_PARAM has no config.json entry in either language.
    expect(wrapper.text()).toContain('Origines autorisées')
    expect(wrapper.text()).toContain('CORS WebSocket')
  })

  it('renders title and only the categories not already owned by a dedicated Settings card', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    expect(wrapper.text()).toContain('Configuration Docker & Système')
    expect(wrapper.text()).toContain("Port d'écoute du serveur")
    expect(wrapper.text()).toContain('Origines autorisées')
    expect(wrapper.text()).toContain('Secret de signature JWT')
    // SMTP_HOST is in "notifications", owned by SettingsSmtpCard — excluded.
    expect(wrapper.text()).not.toContain('Hôte SMTP')
  })

  it('excludes an entry from a non-relevant category from the metrics too', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    // 3 relevant entries (server/network/auth), not the backend's raw
    // total_params of 4 — the "Paramètres totaux" metric is the first one.
    const totalMetric = wrapper.findAll('.font-weight-medium')[0]
    expect(totalMetric.text()).toBe('3')
  })

  it('displays conflict badge for conflicting entries', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    expect(wrapper.text()).toContain('Conflit ENV vs UI')
  })

  it('filters entries by search query', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const searchInput = wrapper.find('input[type="text"]')
    await searchInput.setValue('Origines')

    expect(wrapper.text()).toContain('Origines autorisées')
    expect(wrapper.text()).not.toContain("Port d'écoute du serveur")
  })

  it('filters entries by source', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const selects = wrapper.findAll('select')
    const sourceSelect = selects[selects.length - 1]
    await sourceSelect.setValue('conflict')

    expect(wrapper.text()).toContain('Origines autorisées')
    expect(wrapper.text()).not.toContain("Port d'écoute du serveur")
  })

  it('filters entries by category, offering only the relevant ones', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const categorySelect = wrapper.find('#config-category-filter')
    expect(categorySelect.text()).not.toContain('Alertes')
    await categorySelect.setValue('server')

    expect(wrapper.text()).toContain("Port d'écoute du serveur")
    expect(wrapper.text()).not.toContain('Origines autorisées')
  })

  it('allows saving an edited parameter', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const input = wrapper.find('#input-SERVER_PORT')
    await input.setValue('9090')
    await flushPromises()

    const row = wrapper.findAll('tr').find(r => r.text().includes("Port d'écoute du serveur"))
    const saveBtn = row?.findAll('button').find(b => b.text().includes('Enregistrer'))
    expect(saveBtn).toBeDefined()
    await saveBtn?.trigger('click')
    await flushPromises()

    const { configApi } = await import('../../api/config')
    expect(configApi.updateParam).toHaveBeenCalledWith('SERVER_PORT', '9090')
  })

  it('allows resetting a parameter to default/env', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const jwtRow = wrapper.findAll('tr').find(r => r.text().includes('Secret de signature JWT'))
    const resetBtn = jwtRow?.find('button.btn-outline-danger')
    expect(resetBtn?.exists()).toBe(true)
    await resetBtn?.trigger('click')
    await flushPromises()

    const { configApi } = await import('../../api/config')
    expect(configApi.resetParam).toHaveBeenCalledWith('JWT_SECRET')
  })

  it('allows saving all modified parameters in a category', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const input = wrapper.find('#input-SERVER_PORT')
    await input.setValue('9090')
    await flushPromises()

    const categoryHeaderSave = wrapper.findAll('button').find(b => b.classes().includes('btn-primary') && b.text().includes('Enregistrer'))
    expect(categoryHeaderSave?.exists()).toBe(true)
    await categoryHeaderSave?.trigger('click')
    await flushPromises()

    const { configApi } = await import('../../api/config')
    expect(configApi.updateBulk).toHaveBeenCalled()
  })

  it('toggles reveal secrets', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

    await flushPromises()

    const revealBtn = wrapper.findAll('button').find(b => b.text().includes('Afficher secrets'))
    if (revealBtn) {
      await revealBtn.trigger('click')
      await flushPromises()
      const { configApi } = await import('../../api/config')
      expect(configApi.getConfig).toHaveBeenCalledWith(true)
    }
  })

  it('toggles password visibility locally', async () => {
    const wrapper = mount(SettingsAdvancedConfigCard)

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

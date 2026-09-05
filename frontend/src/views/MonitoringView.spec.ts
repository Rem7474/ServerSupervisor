import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import MonitoringView from './MonitoringView.vue'

const openCreateProbe = vi.fn()

describe('MonitoringView', () => {
  beforeEach(() => {
    setLocale('fr')
    setActivePinia(createPinia())
  })

  function mountView() {
    return mount(MonitoringView, {
      global: {
        stubs: {
          MonitoringOverviewPanel: {
            template: '<div />',
            methods: { openCreateProbe },
          },
        },
      },
    })
  }

  it('renders the translated page title and subtitle', () => {
    const wrapper = mountView()
    expect(wrapper.text()).toContain('Monitoring')
    expect(wrapper.text()).toContain('Sondes HTTP/TCP synthétiques et suivi des certificats SSL/TLS.')
  })

  it('shows the translated "new tracker" button for an admin and delegates to the panel', async () => {
    const auth = useAuthStore()
    auth.role = 'admin'
    const wrapper = mountView()

    const button = wrapper.findAll('button').find((b) => b.text().includes('Nouveau suivi'))
    expect(button).toBeTruthy()
  })

  it('hides the "new tracker" button for a non-admin', () => {
    const auth = useAuthStore()
    auth.role = 'viewer'
    const wrapper = mountView()

    expect(wrapper.findAll('button').find((b) => b.text().includes('Nouveau suivi'))).toBeUndefined()
  })
})

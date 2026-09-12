import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setLocale } from '../../i18n'

// Mock the API barrel so the component never hits the network on mount/watchers.
vi.mock('../../api', () => ({
  default: {
    getHostCapabilities: vi.fn(async () => ({ data: { metrics: [] } })),
    testAlertRule: vi.fn(async () => ({ data: { results: [] } })),
    downloadAlertRuleTestLogs: vi.fn(async () => ({ data: new Blob() })),
  },
}))

import AlertRuleModal from './AlertRuleModal.vue'

const capabilities = {
  metrics: [
    { metric: 'cpu', label: 'CPU', icon: '', supports_host_filter: true },
    { metric: 'memory', label: 'Mémoire', icon: '', supports_host_filter: true },
  ],
  proxmox_scope: { connections: [], nodes: [], storages: [], guests: [], disks: [] },
}

function mountModal(props: Record<string, unknown> = {}) {
  return mount(AlertRuleModal, {
    props: {
      visible: true,
      hosts: [],
      capabilities,
      ...props,
    },
    global: {
      stubs: {
        // Child component is exercised elsewhere; stub to isolate the modal.
        AlertRuleCommandTrigger: true,
      },
    },
  })
}

describe('AlertRuleModal (characterization)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('mounts when visible and renders step 1 (name field + metric cards)', async () => {
    const wrapper = mountModal()
    await nextTick()

    // Name input present (step 1).
    expect(wrapper.find('input[placeholder="Ex: CPU élevé sur serveur web"]').exists()).toBe(true)

    // Metric cards rendered from capabilities.
    const cards = wrapper.findAll('.metric-card')
    expect(cards.length).toBeGreaterThanOrEqual(2)
    expect(wrapper.text()).toContain('CPU')
    expect(wrapper.text()).toContain('Mémoire')
  })

  it('emits "close" when the close button is clicked', async () => {
    const wrapper = mountModal()
    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits "close" on Escape keydown while visible', async () => {
    mountModal()
    // The component registers a document-level keydown listener when visible.
    // We assert via a fresh wrapper that exposes the emit.
    const wrapper = mountModal()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('advances to step 2 once a metric and name are provided', async () => {
    const wrapper = mountModal()
    await nextTick()

    await wrapper.find('input[placeholder="Ex: CPU élevé sur serveur web"]').setValue('CPU rule')
    await wrapper.findAll('.metric-card')[0].trigger('click')
    await nextTick()

    // Click "Suivant" (goNextStep) — the footer next button.
    const nextBtn = wrapper.findAll('button').find((b) => b.text().includes('Suivant'))
    expect(nextBtn).toBeTruthy()
    await nextBtn!.trigger('click')
    await nextTick()

    // Step 2 shows the warning-threshold input (placeholder "70").
    expect(wrapper.find('input[placeholder="70"]').exists()).toBe(true)
  })

  it('emits "submit" with a payload object when saving an edited rule', async () => {
    const rule = {
      id: 1,
      name: 'Existing',
      source_type: 'agent',
      metric: 'cpu',
      operator: '>',
      threshold_warn: 70,
      threshold_crit: 85,
      duration: 0,
      actions: { channels: [] },
    }
    const wrapper = mountModal({ rule })
    await nextTick()

    // Navigate to the final step via the next button (twice).
    for (let i = 0; i < 2; i++) {
      const nextBtn = wrapper.findAll('button').find((b) => b.text().includes('Suivant'))
      if (nextBtn) {
        await nextBtn.trigger('click')
        await nextTick()
      }
    }

    const submitBtn = wrapper.findAll('button').find((b) => b.text().includes('Mettre à jour') || b.text().includes('Créer'))
    expect(submitBtn).toBeTruthy()
    await submitBtn!.trigger('click')
    await nextTick()

    const submits = wrapper.emitted('submit')
    expect(submits).toBeTruthy()
    expect(typeof submits![0][0]).toBe('object')
  })

  it('shows the translated modal title for create vs. edit', () => {
    const created = mountModal()
    expect(created.text()).toContain('Nouvelle alerte')

    const edited = mountModal({ rule: { id: 1, name: 'x', metric: 'cpu', operator: '>', threshold_warn: 1, threshold_crit: 2, duration: 0, actions: { channels: [] } } })
    expect(edited.text()).toContain('Modifier l\'alerte')
  })

  it('applying a preset pre-fills the (translated) rule name', async () => {
    const wrapper = mountModal()
    await nextTick()

    const presetBtn = wrapper.findAll('button').find((b) => b.text().includes('CPU élevé'))
    expect(presetBtn).toBeTruthy()
    await presetBtn!.trigger('click')
    await nextTick()

    expect((wrapper.find('input[placeholder="Ex: CPU élevé sur serveur web"]').element as HTMLInputElement).value).toBe('CPU élevé')
  })
})

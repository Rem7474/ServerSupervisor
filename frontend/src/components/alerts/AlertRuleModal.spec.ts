import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
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

import apiClient from '../../api'
import AlertRuleModal from './AlertRuleModal.vue'
import type { AlertMetricCapability, AlertRuleCapabilities } from '../../types/alert'

function metric(key: string, label: string): AlertMetricCapability {
  return {
    metric: key, label, unit: '%', icon: '', badge_class: '',
    supports_threshold: true, supports_duration: true, supports_host_filter: true,
  }
}

const agentMetrics = [metric('cpu', 'CPU'), metric('memory', 'Mémoire')]
const capabilities: AlertRuleCapabilities = {
  metrics: agentMetrics,
  agent_metrics: agentMetrics,
  proxmox_metrics: [],
  synthetic_metrics: [],
  docker_metrics: [],
  proxmox_scope: { modes: [], connections: [], nodes: [], storages: [], guests: [], disks: [] },
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

  it('"Tester" sends the rule being edited and shows the API error when the test fails', async () => {
    // Earlier tests leave mounted modals whose step-2 preview timers can still
    // fire, so only calls carrying this rule's name are counted.
    const name = 'Tester rule'
    const testAlertRule = vi.mocked(apiClient.testAlertRule)
    const callsForRule = () => testAlertRule.mock.calls.filter(([p]) => (p as { name?: string }).name === name)
    testAlertRule.mockImplementation(async (payload) => {
      if ((payload as { name?: string }).name === name) throw { response: { data: { error: 'métrique inconnue' } } }
      return { data: { results: [] } } as never
    })
    const rule = {
      id: 1, name, source_type: 'agent', metric: 'cpu', operator: '>',
      threshold_warn: 70, threshold_crit: 85, duration: 0, actions: { channels: [] },
    }
    // Entering step 2 schedules this modal's automatic preview test; fake
    // timers let it run before the manual click is counted.
    vi.useFakeTimers()
    try {
      const wrapper = mountModal({ rule })
      await nextTick()
      for (let i = 0; i < 2; i++) {
        await wrapper.findAll('button').find((b) => b.text().includes('Suivant'))!.trigger('click')
        await nextTick()
      }
      await vi.runAllTimersAsync()
      await flushPromises()
      const before = callsForRule().length

      await wrapper.findAll('button').find((b) => b.text() === 'Tester')!.trigger('click')
      await flushPromises()

      expect(callsForRule().length - before).toBe(1)
      const calls = callsForRule()
      expect(calls[calls.length - 1][0]).toMatchObject({ metric: 'cpu', threshold_crit: 85 })
      expect(wrapper.text()).toContain('métrique inconnue')
    } finally {
      vi.useRealTimers()
      testAlertRule.mockImplementation(async () => ({ data: { results: [] } }) as never)
    }
  })
})

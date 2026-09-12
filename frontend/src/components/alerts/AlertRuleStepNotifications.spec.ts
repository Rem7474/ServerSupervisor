import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import AlertRuleStepNotifications from './AlertRuleStepNotifications.vue'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

function baseForm(): AlertRuleFormData {
  return {
    name: 'CPU high',
    enabled: false,
    source_type: 'agent',
    host_id: null,
    proxmox_scope: { scope_mode: '', connection_id: '', node_id: '', storage_id: '', guest_id: '', disk_id: '' },
    docker_scope: {} as AlertRuleFormData['docker_scope'],
    metric: 'cpu',
    operator: '>',
    threshold_warn: 70,
    threshold_crit: 90,
    duration: 300,
    actions: { channels: [], smtp_to: '', ntfy_topic: '', cooldown: 0, escalate_after_minutes: 0 },
  }
}

beforeEach(() => {
  setLocale('fr')
  setActivePinia(createPinia())
})

describe('AlertRuleStepNotifications', () => {
  it('shows the escalation and notification-channel fields', () => {
    const wrapper = mount(AlertRuleStepNotifications, { props: { form: baseForm() } })
    expect(wrapper.text()).toContain('Escalade si non acquittée (minutes)')
    expect(wrapper.text()).toContain('Canaux de notification')
    expect(wrapper.text()).toContain('SMTP (Email)')
  })

  it('shows the cooldown field only when the command trigger is enabled', async () => {
    const wrapper = mount(AlertRuleStepNotifications, {
      props: { form: baseForm(), commandTriggerEnabled: false },
    })
    expect(wrapper.text()).not.toContain('Période de silence (secondes)')

    await wrapper.setProps({ commandTriggerEnabled: true })
    expect(wrapper.text()).toContain('Période de silence (secondes)')
  })

  it('shows the SMTP recipients field only when the SMTP channel is checked', async () => {
    const wrapper = mount(AlertRuleStepNotifications, {
      props: { form: baseForm(), channelSmtp: false },
    })
    expect(wrapper.text()).not.toContain('Destinataire(s) email')

    await wrapper.setProps({ channelSmtp: true })
    expect(wrapper.text()).toContain('Destinataire(s) email')
  })

  it('shows a translated message per browser-permission state', async () => {
    const wrapper = mount(AlertRuleStepNotifications, {
      props: { form: baseForm(), channelBrowser: true, browserPermission: 'denied' },
    })
    expect(wrapper.text()).toContain('Notifications bloquées par le navigateur.')

    await wrapper.setProps({ browserPermission: 'granted' })
    expect(wrapper.text()).toContain('Notifications navigateur autorisées.')

    await wrapper.setProps({ browserPermission: 'unsupported' })
    expect(wrapper.text()).toContain('Ce navigateur ne supporte pas les notifications.')

    await wrapper.setProps({ browserPermission: undefined })
    expect(wrapper.text()).toContain("La permission sera demandée à l'enregistrement.")
  })

  it('shows the translated last-test result and formatted date', () => {
    const wrapper = mount(AlertRuleStepNotifications, {
      props: { form: baseForm(), testResults: { any_fires: true, evaluated_at: '2026-01-01T12:00:00Z' } },
    })
    expect(wrapper.text()).toContain('Dernier test :')
    expect(wrapper.text()).toContain('la règle déclencherait une alerte.')
  })

  it('shows the "would not fire" message when the test found no matches', () => {
    const wrapper = mount(AlertRuleStepNotifications, {
      props: { form: baseForm(), testResults: { any_fires: false } },
    })
    expect(wrapper.text()).toContain('la règle ne déclencherait pas d\'alerte.')
  })
})

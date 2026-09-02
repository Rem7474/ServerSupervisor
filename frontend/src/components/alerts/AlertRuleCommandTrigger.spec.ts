import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import AlertRuleCommandTrigger from './AlertRuleCommandTrigger.vue'

beforeEach(() => {
  setLocale('fr')
  setActivePinia(createPinia())
})

describe('AlertRuleCommandTrigger', () => {
  it('hides the trigger config when disabled', () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: { enabled: false, modelValue: { module: 'processes', action: 'list', target: '' } },
    })
    expect(wrapper.text()).toContain("Déclencher une commande à l'alerte")
    expect(wrapper.find('select').exists()).toBe(false)
  })

  it('emits update:enabled when the checkbox is toggled', async () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: { enabled: false, modelValue: { module: 'processes', action: 'list', target: '' } },
    })
    await wrapper.find('input[type="checkbox"]').setValue(true)
    expect(wrapper.emitted('update:enabled')?.[0]).toEqual([true])
  })

  it('shows the docker action select with translated labels for a container-scoped rule', () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: {
        enabled: true,
        modelValue: { module: 'docker', action: 'logs', target: '' },
        dockerScope: { scope_mode: 'container', container_id: 'c1' },
      },
    })
    const options = wrapper.findAll('option')
    expect(options.map((o) => o.text())).toEqual(['Voir les logs', 'Redémarrer', 'Démarrer', 'Arrêter'])
    expect(wrapper.text()).toContain('Conteneur ciblé par la règle (résolu au déclenchement)')
  })

  it('shows the compose-scoped project hint and extra compose actions', () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: {
        enabled: true,
        modelValue: { module: 'docker', action: 'logs', target: '' },
        dockerScope: { scope_mode: 'compose_project', project_name: 'my-stack' },
      },
    })
    expect(wrapper.text()).toContain('Projet « my-stack » (résolu au déclenchement)')
    expect(wrapper.text()).toContain('Compose up')
    expect(wrapper.text()).toContain('Mettre à jour les images')
  })

  it('emits an update:modelValue with the docker module locked in when a docker rule is enabled', () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: {
        enabled: true,
        modelValue: { module: 'processes', action: 'list', target: '' },
        dockerScope: { scope_mode: 'container', container_id: 'c1' },
      },
    })
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual({ module: 'docker', action: 'logs', target: '' })
  })

  it('falls back to the raw action value for an action with no translated label', () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: {
        enabled: true,
        modelValue: { module: 'systemd', action: 'status', target: 'nginx' },
        dockerScope: null,
      },
    })
    // Module select is index 0; systemd's action select (status/start/stop/restart) is index 1.
    const actionSelect = wrapper.findAll('select')[1]
    expect(actionSelect.findAll('option').map((o) => o.text())).toEqual([
      'status', 'Démarrer', 'Arrêter', 'Redémarrer',
    ])
  })

  it('renders the non-docker DispatchStepEditor form with translated module labels', () => {
    const wrapper = mount(AlertRuleCommandTrigger, {
      props: {
        enabled: true,
        modelValue: { module: 'processes', action: 'list', target: '' },
        dockerScope: null,
      },
    })
    const moduleSelect = wrapper.findAll('select')[0]
    expect(moduleSelect.findAll('option').map((o) => o.text())).toEqual([
      'Processus (top)', 'Journal systemd', 'Service systemd', 'Conteneur Docker',
    ])
  })
})

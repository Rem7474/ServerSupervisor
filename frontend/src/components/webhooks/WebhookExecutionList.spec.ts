import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import WebhookExecutionList from './WebhookExecutionList.vue'

const stubs = { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } }

beforeEach(() => {
  setLocale('fr')
})

describe('WebhookExecutionList', () => {
  it('renders the translated default title and empty state when no props are overridden', () => {
    const wrapper = mount(WebhookExecutionList, { props: { executions: [] }, global: { stubs } })
    expect(wrapper.text()).toContain('Historique des exécutions')
    expect(wrapper.text()).toContain('Aucune exécution enregistrée.')
  })

  it('renders the translated tracker-kind column headers, status label, and alert tooltip', () => {
    const wrapper = mount(WebhookExecutionList, {
      props: {
        kind: 'tracker',
        executions: [{ id: '1', triggered_at: '2026-01-01T00:00:00Z', tag_name: 'v1.0', status: 'completed', command_id: 'cmd-1', alerts_after_count: 2 }],
      },
      global: { stubs },
    })
    for (const label of ['Release', 'Hôte', 'Statut', 'Logs']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('Terminé')
    expect(wrapper.find('a.badge').attributes('title')).toBe("2 alertes déclenchées sur l'hôte dans les 15 min suivant ce déploiement")
  })

  it('uses the singular for a single triggered alert', () => {
    const wrapper = mount(WebhookExecutionList, {
      props: {
        kind: 'tracker',
        executions: [{ id: '1', triggered_at: '2026-01-01T00:00:00Z', tag_name: 'v1.0', status: 'completed', command_id: 'cmd-1', alerts_after_count: 1 }],
      },
      global: { stubs },
    })
    expect(wrapper.find('a.badge').attributes('title')).toBe("1 alerte déclenchée sur l'hôte dans les 15 min suivant ce déploiement")
  })

  it('renders the translated webhook-kind column headers and payload tooltip', () => {
    const wrapper = mount(WebhookExecutionList, {
      props: {
        kind: 'webhook',
        executions: [{ id: '1', triggered_at: '2026-01-01T00:00:00Z', repo_name: 'org/repo', status: 'failed', raw_payload: '{}' }],
      },
      global: { stubs },
    })
    for (const label of ['Repo / Branche', 'Commit', 'Statut', 'Logs']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('Échoué')
    expect(wrapper.find('[title="Voir le payload reçu"]').exists()).toBe(true)
  })

  it('links to the audit log with a named icon button, and offers the payload only for webhooks', () => {
    const execution = { id: '1', triggered_at: '2026-01-01T00:00:00Z', status: 'completed', command_id: 'cmd-1', raw_payload: '{}' }
    const tracker = mount(WebhookExecutionList, { props: { kind: 'tracker', executions: [execution] }, global: { stubs } })
    const link = tracker.find('a[href="/audit?command=cmd-1"]')
    expect(link.attributes('aria-label')).toBe('Voir les logs')
    expect(tracker.find('button[aria-label="Voir le payload reçu"]').exists()).toBe(false)

    const webhook = mount(WebhookExecutionList, { props: { kind: 'webhook', executions: [execution] }, global: { stubs } })
    expect(webhook.find('a[href="/audit?command=cmd-1"]').exists()).toBe(true)
  })

  it('shows a translated refresh button when showRefresh is set', () => {
    const wrapper = mount(WebhookExecutionList, { props: { executions: [], showRefresh: true }, global: { stubs } })
    expect(wrapper.text()).toContain('Actualiser')
  })

  it('shows a translated pagination range label beyond one page', () => {
    const executions = Array.from({ length: 25 }, (_, i) => ({ id: String(i), triggered_at: '2026-01-01T00:00:00Z', status: 'completed' }))
    const wrapper = mount(WebhookExecutionList, { props: { executions }, global: { stubs } })
    expect(wrapper.text()).toContain('1–20 sur 25')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(WebhookExecutionList, { props: { executions: [] }, global: { stubs } })
    expect(wrapper.text()).toContain('Execution history')
    expect(wrapper.text()).toContain('No execution recorded.')
  })
})

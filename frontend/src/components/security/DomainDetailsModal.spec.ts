import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import DomainDetailsModal from './DomainDetailsModal.vue'

function mountModal(overrides: Record<string, unknown> = {}) {
  return mount(DomainDetailsModal, {
    props: {
      show: true,
      domain: 'app.example.com',
      loading: false,
      details: {
        hits: 10, bytes: 2048, status_4xx: 1, status_5xx: 0,
        top_paths: [], top_clients: [], requests: [], total: 0,
      },
      period: '24h',
      filters: { status: '', method: '', path: '', ip: '' },
      sortKey: 'time',
      sortDir: 'desc',
      page: 1,
      totalPages: 1,
      hasActiveFilters: false,
      ...overrides,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('DomainDetailsModal', () => {
  it('renders the translated title, close button and KPI tooltips', () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Détails domaine:')
    expect(wrapper.text()).toContain('app.example.com')
    expect(wrapper.text()).toContain('Fenêtre de logs détaillée sur 24h')
    expect(wrapper.text()).toContain('Fermer')
    expect(wrapper.find('.kpi-btn[title="Filtrer sur les statuts 4xx"]').exists()).toBe(true)
    expect(wrapper.find('.kpi-btn[title="Filtrer sur les statuts 5xx"]').exists()).toBe(true)
  })

  it('shows the translated empty states for top paths/clients and the requests table', () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Top chemins')
    expect(wrapper.text()).toContain('Aucun chemin')
    expect(wrapper.text()).toContain('Top IPs clientes')
    expect(wrapper.text()).toContain('Aucune IP')
    expect(wrapper.text()).toContain('Aucune requête disponible')
  })

  it('renders the translated table headers, toolbar options, and pagination label', () => {
    const wrapper = mountModal({ totalPages: 3, details: { hits: 10, bytes: 0, status_4xx: 0, status_5xx: 0, top_paths: [], top_clients: [], requests: [], total: 42 } })
    for (const label of ['Heure', 'IP', 'Méthode', 'Chemin']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('Tous statuts')
    expect(wrapper.text()).toContain('Toutes méthodes')
    expect(wrapper.text()).toContain('Page 1 sur 3 — 42 résultats')
  })

  it('shows a translated "reset" button and active-filter chips with translated aria-labels when filters are active', () => {
    const wrapper = mountModal({ hasActiveFilters: true, filters: { status: '', method: '', path: '/health', ip: '1.2.3.4' } })
    expect(wrapper.text()).toContain('Réinitialiser')
    expect(wrapper.text()).toContain('Filtres actifs :')
    expect(wrapper.find('[aria-label="Retirer le filtre chemin"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="Retirer le filtre IP"]').exists()).toBe(true)
  })

  it('shows translated block/copy tooltips and B/K badge titles for a client row', () => {
    const wrapper = mountModal({
      details: {
        hits: 1, bytes: 0, status_4xx: 0, status_5xx: 0, top_paths: [], total: 0,
        top_clients: [{ ip: '5.6.7.8', hits: 3, blocked: false, host_id: '' }],
        requests: [{ timestamp: '2026-01-01T00:00:00Z', ip: '5.6.7.8', method: 'GET', path: '/', status: 200, bytes: 100, blocked: true, suspicious: true, host_id: '' }],
      },
    })
    expect(wrapper.find('[title="Copier l\'IP"]').exists()).toBe(true)
    expect(wrapper.find('[title="Hôte introuvable"]').exists()).toBe(true)
    expect(wrapper.find('[title="Bloquée"]').exists()).toBe(true)
    expect(wrapper.find('[title="Suspecte"]').exists()).toBe(true)
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Domain details:')
    expect(wrapper.text()).toContain('Close')
  })
})

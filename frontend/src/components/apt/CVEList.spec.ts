import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import CVEList from './CVEList.vue'

beforeEach(() => {
  setLocale('fr')
})

describe('CVEList', () => {
  it('shows "no CVE detected" when the list is empty', () => {
    const wrapper = mount(CVEList, { props: { cveList: [] } })
    expect(wrapper.text()).toContain('Aucune CVE détectée')
  })

  it('parses a JSON string cveList the same as an array', () => {
    const wrapper = mount(CVEList, {
      props: { cveList: JSON.stringify([{ id: 'CVE-2024-1', severity: 'HIGH', package: 'openssl' }]) },
    })
    expect(wrapper.text()).toContain('CVE-2024-1')
    expect(wrapper.text()).toContain('openssl')
  })

  it('falls back to an empty list and logs when the string is not valid JSON', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = mount(CVEList, { props: { cveList: 'not json' } })
    expect(wrapper.text()).toContain('Aucune CVE détectée')
    expect(spy).toHaveBeenCalled()
    spy.mockRestore()
  })

  it('shows the max severity summary by default, with the correct CVE count pluralization', () => {
    const wrapper = mount(CVEList, {
      props: {
        cveList: [
          { id: 'CVE-1', severity: 'LOW', package: 'a' },
          { id: 'CVE-2', severity: 'CRITICAL', package: 'b' },
        ],
      },
    })
    expect(wrapper.text()).toContain('Criticité max:')
    expect(wrapper.text()).toContain('CRITICAL')
    expect(wrapper.text()).toContain('(2 CVEs)')
  })

  it('uses the singular CVE count phrasing for exactly one entry', () => {
    const wrapper = mount(CVEList, { props: { cveList: [{ id: 'CVE-1', severity: 'LOW', package: 'a' }] } })
    expect(wrapper.text()).toContain('(1 CVE)')
  })

  it('hides the max severity summary when showMaxSeverity is false', () => {
    const wrapper = mount(CVEList, {
      props: { cveList: [{ id: 'CVE-1', severity: 'LOW', package: 'a' }], showMaxSeverity: false },
    })
    expect(wrapper.text()).not.toContain('Criticité max:')
  })

  it('groups CVE entries by id and unions their impacted packages', () => {
    const wrapper = mount(CVEList, {
      props: {
        cveList: [
          { id: 'CVE-1', severity: 'HIGH', package: 'openssl' },
          { id: 'CVE-1', severity: 'HIGH', package: 'libssl' },
        ],
      },
    })
    // One group row, two impacted packages.
    expect(wrapper.findAll('.cve-group-row').length).toBe(1)
    expect(wrapper.text()).toContain('2 paquets impactés')
    expect(wrapper.text()).toContain('openssl')
    expect(wrapper.text()).toContain('libssl')
  })

  it('uses the singular impacted-package phrasing for a single package', () => {
    const wrapper = mount(CVEList, { props: { cveList: [{ id: 'CVE-1', severity: 'HIGH', package: 'openssl' }] } })
    expect(wrapper.text()).toContain('1 paquet impacté')
  })

  it('falls back to a translated placeholder when a CVE has no package name', () => {
    const wrapper = mount(CVEList, { props: { cveList: [{ id: 'CVE-1', severity: 'HIGH' }] } })
    expect(wrapper.text()).toContain('Paquet non spécifié')
  })

  it('shows the CVSS score only when present', () => {
    const withScore = mount(CVEList, { props: { cveList: [{ id: 'CVE-1', severity: 'HIGH', package: 'a', cvss_score: 7.5 }] } })
    expect(withScore.text()).toContain('CVSS 7.5')

    const withoutScore = mount(CVEList, { props: { cveList: [{ id: 'CVE-1', severity: 'HIGH', package: 'a' }] } })
    expect(withoutScore.text()).not.toContain('CVSS')
  })

  it('truncates to the limit and expands on click, showing the See-all count', async () => {
    const cves = Array.from({ length: 8 }, (_, i) => ({ id: `CVE-${i}`, severity: 'LOW', package: `pkg-${i}` }))
    const wrapper = mount(CVEList, { props: { cveList: cves, limit: 5 } })
    expect(wrapper.findAll('.cve-group-row').length).toBe(5)
    expect(wrapper.text()).toContain('Voir tout (8)')

    await wrapper.find('button').trigger('click')
    expect(wrapper.findAll('.cve-group-row').length).toBe(8)
    expect(wrapper.text()).toContain('Réduire')
  })

  it('hides the toggle button when alwaysExpanded is true, even past the limit', () => {
    const cves = Array.from({ length: 8 }, (_, i) => ({ id: `CVE-${i}`, severity: 'LOW', package: `pkg-${i}` }))
    const wrapper = mount(CVEList, { props: { cveList: cves, limit: 5, alwaysExpanded: true } })
    expect(wrapper.findAll('.cve-group-row').length).toBe(8)
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('sorts groups by severity rank, highest first', () => {
    const wrapper = mount(CVEList, {
      props: {
        cveList: [
          { id: 'CVE-LOW', severity: 'LOW', package: 'a' },
          { id: 'CVE-CRIT', severity: 'CRITICAL', package: 'b' },
          { id: 'CVE-MED', severity: 'MEDIUM', package: 'c' },
        ],
      },
    })
    const ids = wrapper.findAll('.cve-group-package .fw-semibold').map((el) => el.text())
    expect(ids).toEqual(['CVE-CRIT', 'CVE-MED', 'CVE-LOW'])
  })
})

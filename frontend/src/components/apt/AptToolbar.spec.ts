import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AptToolbar from './AptToolbar.vue'

const filterOptions = [
  { value: 'all', label: 'Tous' },
  { value: 'critical', label: 'CVE critiques' },
]

function mountToolbar(props: Partial<InstanceType<typeof AptToolbar>['$props']> = {}) {
  return mount(AptToolbar, {
    props: {
      filterOptions,
      canRunApt: true,
      selectedCount: 0,
      bulkLoading: null,
      filteredCount: 5,
      outdatedCount: 0,
      agentUpdateLoading: false,
      search: '',
      quickFilter: 'all',
      sortKey: 'name',
      sortDir: 'asc',
      allSelected: false,
      ...props,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('AptToolbar', () => {
  it('shows "select all hosts" when no search/filter narrows the list', () => {
    const wrapper = mountToolbar({ quickFilter: 'all', search: '' })
    expect(wrapper.text()).toContain('Sélectionner tous les hôtes')
  })

  it('shows the filtered-count phrasing once a quick filter is active', () => {
    const wrapper = mountToolbar({ quickFilter: 'critical', filteredCount: 3 })
    expect(wrapper.text()).toContain('Sélectionner les 3 hôtes affichés')
  })

  it('shows the singular filtered phrasing for exactly one result', () => {
    const wrapper = mountToolbar({ quickFilter: 'critical', filteredCount: 1 })
    expect(wrapper.text()).toContain("Sélectionner l'hôte affiché")
  })

  it('shows the prompt to select hosts instead of bulk buttons when nothing is selected', () => {
    const wrapper = mountToolbar({ selectedCount: 0 })
    expect(wrapper.text()).toContain('Sélectionner des hôtes pour les actions groupées')
    expect(wrapper.text()).not.toContain('apt update')
  })

  it('shows the bulk apt update/upgrade/dist-upgrade buttons once hosts are selected and the user can run apt', () => {
    const wrapper = mountToolbar({ selectedCount: 3, canRunApt: true })
    expect(wrapper.text()).toContain('apt update (3)')
    expect(wrapper.text()).toContain('apt upgrade (3)')
    expect(wrapper.text()).toContain('apt dist-upgrade (3)')
  })

  it('hides the bulk buttons for a read-only user even with hosts selected', () => {
    const wrapper = mountToolbar({ selectedCount: 3, canRunApt: false })
    expect(wrapper.text()).not.toContain('apt update (3)')
  })

  it('emits bulk-cmd with the right command from each button', async () => {
    const wrapper = mountToolbar({ selectedCount: 2, canRunApt: true })
    const buttons = wrapper.findAll('.ms-auto button')
    await buttons[0].trigger('click')
    await buttons[1].trigger('click')
    await buttons[2].trigger('click')
    expect(wrapper.emitted('bulk-cmd')).toEqual([['update'], ['upgrade'], ['dist-upgrade']])
  })

  it('shows the update-agents button only when outdatedCount > 0 and canRunApt', () => {
    expect(mountToolbar({ outdatedCount: 0 }).text()).not.toContain('Mettre à jour les agents')
    const wrapper = mountToolbar({ outdatedCount: 2, canRunApt: true })
    expect(wrapper.text()).toContain('Mettre à jour les agents (2)')
  })

  it('emits agent-update-cmd when the update-agents button is clicked', async () => {
    const wrapper = mountToolbar({ outdatedCount: 2, canRunApt: true })
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Mettre à jour les agents'))
    await btn!.trigger('click')
    expect(wrapper.emitted('agent-update-cmd')).toBeTruthy()
  })

  it('toggles the sort direction and its icon/title on click', async () => {
    const wrapper = mountToolbar({ sortDir: 'asc' })
    expect(wrapper.find('[title="Croissant"]').exists()).toBe(true)
    await wrapper.find('[title="Croissant"]').trigger('click')
    expect(wrapper.emitted('update:sortDir')?.[0]).toEqual(['desc'])
  })
})

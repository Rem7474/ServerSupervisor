import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const { getHostGroups, createHostGroup, updateHostGroup, deleteHostGroup, getHosts } = vi.hoisted(() => ({
  getHostGroups: vi.fn(),
  createHostGroup: vi.fn(),
  updateHostGroup: vi.fn(),
  deleteHostGroup: vi.fn(),
  getHosts: vi.fn(),
}))

vi.mock('../../api/index', () => ({
  default: { getHostGroups, createHostGroup, updateHostGroup, deleteHostGroup, getHosts },
}))

import SettingsHostGroupsCard from './SettingsHostGroupsCard.vue'

const hosts = [
  { id: 'h1', name: 'web-1', tags: ['prod', 'web'], os: 'Debian GNU/Linux 12' },
  { id: 'h2', name: 'db-1', tags: ['prod', 'db'], os: 'Ubuntu 24.04' },
  { id: 'h3', name: '', hostname: 'lab.local', tags: ['lab'], os: 'Debian GNU/Linux 13' },
]
const group = {
  id: 'g1', name: 'Prod', description: 'Serveurs de prod', match_mode: 'all',
  tags: ['prod'], os_contains: '', member_ids: ['h1', 'h2', 'gone'],
}

function mountCard(authIsAdmin = true) {
  return mount(SettingsHostGroupsCard, { props: { authIsAdmin } })
}

const field = (wrapper: ReturnType<typeof mountCard>, label: string) => {
  const id = wrapper.findAll('label').find(l => l.text().startsWith(label))!.attributes('for')
  return wrapper.find(`#${CSS.escape(id!)}`)
}

const saveButton = (wrapper: ReturnType<typeof mountCard>) =>
  wrapper.findAll('button').find(b => ['Créer', 'Mettre à jour'].includes(b.text()))!

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  setLocale('fr')
  getHostGroups.mockResolvedValue({ data: [] })
  getHosts.mockResolvedValue({ data: hosts })
})

describe('SettingsHostGroupsCard', () => {
  it('shows the empty state when no group exists', async () => {
    const wrapper = mountCard()
    await flushPromises()
    expect(wrapper.text()).toContain("Aucun groupe d'hôtes.")
  })

  it('lists each group with its rule and its computed member count', async () => {
    getHostGroups.mockResolvedValue({ data: [group, { ...group, id: 'g2', name: 'Debian', description: '', match_mode: 'any', tags: ['db', 'lab'], os_contains: 'debian', member_ids: ['h3'] }] })
    const wrapper = mountCard()
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('Serveurs de prod')
    expect(rows[0].text()).toContain('tous :')
    expect(rows[0].text()).toContain('3 hôtes')
    // Member names come from the hosts store; unknown ids stay visible as ids.
    expect(rows[0].find('.badge.bg-blue-lt').attributes('title')).toBe('web-1, db-1, gone')
    expect(rows[1].text()).toContain('au moins un :')
    expect(rows[1].text()).toContain('OS contenant « debian »')
    expect(rows[1].text()).toContain('1 hôte')
    expect(rows[1].find('.badge.bg-blue-lt').attributes('title')).toBe('lab.local')
  })

  it('hides the edit controls from non-admins', async () => {
    getHostGroups.mockResolvedValue({ data: [group] })
    const wrapper = mountCard(false)
    await flushPromises()
    expect(wrapper.find('button[title="Modifier"]').exists()).toBe(false)
    expect(wrapper.find('.btn-primary.btn-sm').exists()).toBe(false)
  })

  it('previews the members live while the rule is edited', async () => {
    const wrapper = mountCard()
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')

    expect(wrapper.text()).toContain('0 hôte correspond actuellement')
    await field(wrapper, 'Tags').setValue('prod')
    expect(wrapper.text()).toContain('2 hôtes correspondent actuellement — web-1, db-1')
    await field(wrapper, "Filtre d'OS").setValue('debian')
    expect(wrapper.text()).toContain('1 hôte correspond actuellement — web-1')
  })

  it('toggles a known tag into and out of the tag field', async () => {
    const wrapper = mountCard()
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')

    const tagButtons = wrapper.findAll('button[aria-pressed]')
    expect(tagButtons.map(b => b.text())).toEqual(['db', 'lab', 'prod', 'web'])
    await tagButtons[2].trigger('click')
    await tagButtons[3].trigger('click')
    expect((field(wrapper, 'Tags').element as HTMLInputElement).value).toBe('prod, web')
    expect(wrapper.findAll('button[aria-pressed="true"]').map(b => b.text())).toEqual(['prod', 'web'])
    await tagButtons[2].trigger('click')
    expect((field(wrapper, 'Tags').element as HTMLInputElement).value).toBe('web')
  })

  it('requires a name, then at least one criterion', async () => {
    const wrapper = mountCard()
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')

    await saveButton(wrapper).trigger('click')
    expect(wrapper.text()).toContain('Le nom du groupe est obligatoire.')

    await field(wrapper, 'Nom').setValue('Prod')
    await saveButton(wrapper).trigger('click')
    expect(wrapper.text()).toContain("Indiquez au moins un tag ou un filtre d'OS.")
    expect(createHostGroup).not.toHaveBeenCalled()
  })

  it('creates a group with the tags parsed from the input', async () => {
    createHostGroup.mockResolvedValue({ data: {} })
    const wrapper = mountCard()
    await flushPromises()
    await wrapper.find('.btn-primary.btn-sm').trigger('click')
    await field(wrapper, 'Nom').setValue('Web')
    await field(wrapper, 'Correspondance').setValue('any')
    await field(wrapper, 'Tags').setValue(' web, , edge ')
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(createHostGroup).toHaveBeenCalledWith({
      name: 'Web', description: '', match_mode: 'any', tags: ['web', 'edge'], os_contains: '',
    })
    expect(getHostGroups).toHaveBeenCalledTimes(2)
  })

  it('pre-fills the form and updates when editing', async () => {
    getHostGroups.mockResolvedValue({ data: [{ ...group, match_mode: '' }] })
    updateHostGroup.mockResolvedValue({ data: {} })
    const wrapper = mountCard()
    await flushPromises()

    await wrapper.find('button[title="Modifier"]').trigger('click')
    expect((field(wrapper, 'Tags').element as HTMLInputElement).value).toBe('prod')
    expect((field(wrapper, 'Correspondance').element as HTMLSelectElement).value).toBe('all')
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(updateHostGroup).toHaveBeenCalledWith('g1', expect.objectContaining({ name: 'Prod', tags: ['prod'], match_mode: 'all' }))
  })

  it('shows the server refusal when the group is still targeted by rules', async () => {
    getHostGroups.mockResolvedValue({ data: [group] })
    deleteHostGroup.mockRejectedValue({
      response: { data: { error: 'host group is targeted by alert rules', i18nKey: 'HOST_GROUP_IN_USE', params: { count: '2' } } },
    })
    const wrapper = mountCard()
    await flushPromises()

    const click = wrapper.find('button[title="Supprimer"]').trigger('click')
    const dialog = useConfirmDialog()
    expect(dialog.message.value).toContain('Prod')
    dialog.onConfirm()
    await click
    await flushPromises()

    expect(deleteHostGroup).toHaveBeenCalledWith('g1')
    expect(wrapper.find('.card-footer').text()).toContain("ciblé par des règles d'alerte (2)")
  })

  it('translates in English', async () => {
    setLocale('en')
    const wrapper = mountCard()
    await flushPromises()
    expect(wrapper.text()).toContain('Host groups')
    expect(wrapper.text()).toContain('No host group.')
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'

const { getHostCustomTasks, getBackupProfiles, getBackupGroups } = vi.hoisted(() => ({
  getHostCustomTasks: vi.fn(),
  getBackupProfiles: vi.fn(),
  getBackupGroups: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getHostCustomTasks, getBackupProfiles, getBackupGroups },
}))

import DispatchStepEditor from './DispatchStepEditor.vue'

function mountEditor() {
  return mount(DispatchStepEditor, {
    props: {
      hostId: 'h1',
      'onUpdate:hostId': () => {},
      module: 'docker',
      'onUpdate:module': () => {},
      action: '',
      'onUpdate:action': () => {},
      target: '',
      'onUpdate:target': () => {},
      actionsForModule: () => [],
      targetConfig: (m: string) => (m === 'docker' ? { label: 'Conteneur', placeholder: 'nginx' } : { label: 'Cible' }),
      showHost: false,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
  setActivePinia(createPinia())
  getHostCustomTasks.mockResolvedValue({ data: [] })
  getBackupProfiles.mockResolvedValue({ data: { profiles: [] } })
  getBackupGroups.mockResolvedValue({ data: { groups: [] } })
})

describe('DispatchStepEditor target field', () => {
  it('falls back to a free-text input for a module with no discovered options (docker)', async () => {
    const wrapper = mountEditor()
    await flushPromises()
    // Only the Module <select> is present — Action falls back to input
    // (empty actionsForModule) and Target falls back to input (docker has
    // no discovered-options source at all).
    expect(wrapper.findAll('select').length).toBe(1)
    expect(wrapper.find('input[placeholder="nginx"]').exists()).toBe(true)
  })

  it('offers a select of discovered custom tasks by value/label', async () => {
    getHostCustomTasks.mockResolvedValue({ data: [{ id: 'deploy', name: 'Deploy' }] })
    const wrapper = mount(DispatchStepEditor, {
      props: {
        hostId: 'h1', 'onUpdate:hostId': () => {},
        module: 'custom', 'onUpdate:module': () => {},
        action: '', 'onUpdate:action': () => {},
        target: '', 'onUpdate:target': () => {},
        actionsForModule: () => [],
        targetConfig: () => ({ label: 'Tâche' }),
        showHost: false,
      },
    })
    await flushPromises()
    // Module select is first, Target select (populated) is last.
    const selects = wrapper.findAll('select')
    expect(selects.length).toBe(2)
    expect(selects[1].text()).toContain('Deploy (deploy)')
  })

  it('offers profiles and groups grouped by <optgroup> for module=restic', async () => {
    getBackupProfiles.mockResolvedValue({ data: { profiles: ['files'] } })
    getBackupGroups.mockResolvedValue({ data: { groups: ['full-backup'] } })
    const wrapper = mount(DispatchStepEditor, {
      props: {
        hostId: 'h1', 'onUpdate:hostId': () => {},
        module: 'restic', 'onUpdate:module': () => {},
        action: '', 'onUpdate:action': () => {},
        target: '', 'onUpdate:target': () => {},
        actionsForModule: () => [],
        targetConfig: () => ({ label: 'Profil' }),
        showHost: false,
      },
    })
    await flushPromises()
    const selects = wrapper.findAll('select')
    expect(selects.length).toBe(2)
    const optgroups = selects[1].findAll('optgroup')
    expect(optgroups.map((g) => g.attributes('label'))).toEqual(['Profils', 'Groupes'])
    expect(wrapper.text()).toContain('plusieurs profils')
  })

  it('renders the translated host label/placeholder when showHost is enabled', async () => {
    const wrapper = mount(DispatchStepEditor, {
      props: {
        hostId: '', 'onUpdate:hostId': () => {},
        module: 'docker', 'onUpdate:module': () => {},
        action: '', 'onUpdate:action': () => {},
        target: '', 'onUpdate:target': () => {},
        actionsForModule: () => [],
        targetConfig: () => null,
        showHost: true,
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('Hôte')
    expect(wrapper.text()).toContain('Sélectionner un hôte...')
  })

  it('translates the profiles/groups optgroup labels to English when the locale is switched', async () => {
    setLocale('en')
    getBackupProfiles.mockResolvedValue({ data: { profiles: ['files'] } })
    getBackupGroups.mockResolvedValue({ data: { groups: ['full-backup'] } })
    const wrapper = mount(DispatchStepEditor, {
      props: {
        hostId: 'h1', 'onUpdate:hostId': () => {},
        module: 'restic', 'onUpdate:module': () => {},
        action: '', 'onUpdate:action': () => {},
        target: '', 'onUpdate:target': () => {},
        actionsForModule: () => [],
        targetConfig: () => ({ label: 'Profile' }),
        showHost: false,
      },
    })
    await flushPromises()
    const optgroups = wrapper.findAll('select')[1].findAll('optgroup')
    expect(optgroups.map((g) => g.attributes('label'))).toEqual(['Profiles', 'Groups'])
  })
})

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ResticBackupSummaryCard from './ResticBackupSummaryCard.vue'
import type { ResticBackupSummary } from './ResticBackupSummaryCard.vue'

describe('ResticBackupSummaryCard', () => {
  it('renders a successful summary with files/volume/snapshot/repo size', () => {
    const summary: ResticBackupSummary = {
      status: 'ok',
      profile: 'files',
      duration_sec: 125,
      files_new: 3,
      files_changed: 1,
      bytes_added: 2048,
      snapshot_id: 'abc123',
      repo_size_bytes: 1024 * 1024,
    }
    const wrapper = mount(ResticBackupSummaryCard, { props: { summary } })
    expect(wrapper.find('.badge').text()).toBe('ok')
    expect(wrapper.text()).toContain('files')
    expect(wrapper.text()).toContain('2min')
    expect(wrapper.text()).toContain('3 nouveau')
    expect(wrapper.text()).toContain('1 modifié')
    expect(wrapper.text()).toContain('abc123')
    expect(wrapper.text()).toContain('Mo')
    expect(wrapper.find('.alert-danger').exists()).toBe(false)
  })

  it('renders an error message for a failed run', () => {
    const summary: ResticBackupSummary = {
      status: 'error',
      duration_sec: 5,
      error_message: 'resticconf not readable',
    }
    const wrapper = mount(ResticBackupSummaryCard, { props: { summary } })
    expect(wrapper.find('.badge').text()).toBe('error')
    expect(wrapper.find('.alert-danger').text()).toContain('resticconf not readable')
  })

  it('falls back to "défaut" when no profile is set', () => {
    const summary: ResticBackupSummary = { status: 'ok', duration_sec: 10 }
    const wrapper = mount(ResticBackupSummaryCard, { props: { summary } })
    expect(wrapper.text()).toContain('défaut')
  })

  it('omits optional rows entirely when the corresponding field is absent', () => {
    const summary: ResticBackupSummary = { status: 'ok', duration_sec: 10 }
    const wrapper = mount(ResticBackupSummaryCard, { props: { summary } })
    expect(wrapper.text()).not.toContain('Fichiers')
    expect(wrapper.text()).not.toContain('Snapshot')
    expect(wrapper.text()).not.toContain('Volume ajouté')
    expect(wrapper.text()).not.toContain('Taille du dépôt')
  })
})

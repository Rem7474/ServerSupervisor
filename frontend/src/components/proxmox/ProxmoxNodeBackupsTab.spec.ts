import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ProxmoxNodeBackupsTab from './ProxmoxNodeBackupsTab.vue'
import type { ProxmoxBackupJob, ProxmoxBackupRun } from '../../types/proxmox'

beforeEach(() => {
  setLocale('fr')
})

const JOB: ProxmoxBackupJob = {
  id: 'j1', connection_id: 'c1', job_id: 'backup-nightly', enabled: true,
  schedule: '0 3 * * *', storage: 'local', mode: 'snapshot', compress: 'zstd',
  vmids: 'all', mail_to: '', last_seen_at: new Date().toISOString(),
}

const RUN_OK: ProxmoxBackupRun = {
  id: 'r1', connection_id: 'c1', node_name: 'pve1', vmid: 100, task_upid: 'UPID:pve1:...',
  status: 'OK', end_time: new Date().toISOString(), exit_status: 'OK',
  last_seen_at: new Date().toISOString(), guest_name: 'web-01',
}

const RUN_ERROR: ProxmoxBackupRun = {
  id: 'r2', connection_id: 'c1', node_name: 'pve1', vmid: 101, task_upid: 'UPID:pve1:...',
  status: 'failed', end_time: new Date().toISOString(),
  exit_status: "could not activate storage 'pbs-immich': error fetching datastores - 500",
  last_seen_at: new Date().toISOString(), guest_name: 'db-01',
}

function mountTab(props: Partial<InstanceType<typeof ProxmoxNodeBackupsTab>['$props']> = {}) {
  return mount(ProxmoxNodeBackupsTab, {
    props: { jobs: [], runs: [], loading: false, error: '', ...props },
  })
}

describe('ProxmoxNodeBackupsTab', () => {
  it('shows an empty state for both sections when there is no data', () => {
    const wrapper = mountTab()
    expect(wrapper.text()).toContain('Aucun job de sauvegarde configuré')
    expect(wrapper.text()).toContain('Aucun résultat de sauvegarde')
  })

  it('renders a configured job with its schedule and target', () => {
    const wrapper = mountTab({ jobs: [JOB] })
    expect(wrapper.text()).toContain('backup-nightly')
    expect(wrapper.text()).toContain('0 3 * * *')
    expect(wrapper.text()).toContain('Tous')
    expect(wrapper.text()).toContain('Activé')
  })

  it('renders a disabled job', () => {
    const wrapper = mountTab({ jobs: [{ ...JOB, enabled: false }] })
    expect(wrapper.text()).toContain('Désactivé')
  })

  it('renders backup runs with the unified execution-state badge (OK -> success)', () => {
    const wrapper = mountTab({ runs: [RUN_OK, RUN_ERROR] })
    expect(wrapper.text()).toContain('web-01')
    expect(wrapper.text()).toContain('db-01')
    expect(wrapper.text()).toContain('OK')
    expect(wrapper.text()).toContain('Échoué')
    const badges = wrapper.findAll('.badge')
    expect(badges.some((b) => b.classes().includes('bg-success-lt'))).toBe(true)
    expect(badges.some((b) => b.classes().includes('bg-danger-lt'))).toBe(true)
  })

  it('falls back to "VM <vmid>" when the guest name is missing', () => {
    const wrapper = mountTab({ runs: [{ ...RUN_OK, guest_name: undefined }] })
    expect(wrapper.text()).toContain('VM 100')
  })

  it('emits view-logs with the task UPID when the logs button is clicked', async () => {
    const wrapper = mountTab({ runs: [RUN_OK] })
    await wrapper.find('button[title="Voir les logs"]').trigger('click')
    expect(wrapper.emitted('view-logs')?.[0][0]).toMatchObject({ upid: RUN_OK.task_upid, label: 'web-01' })
  })

  it('shows the error alert when loading failed', () => {
    const wrapper = mountTab({ error: 'Erreur lors du chargement des sauvegardes.' })
    expect(wrapper.text()).toContain('Erreur lors du chargement des sauvegardes.')
  })
})

describe('ProxmoxNodeBackupsTab — status presentation', () => {
  // The badge used to be the raw PVE outcome, so a storage failure rendered as
  // an untranslated, unbounded grey badge while the live task panel showed a
  // red "failed" for the very same run.
  it('badges a failure with the shared state vocabulary and keeps PVE wording alongside', () => {
    const wrapper = mountTab({ runs: [RUN_ERROR] })

    const badge = wrapper.find('tbody .badge')
    expect(badge.text()).toBe('Échoué')
    expect(badge.classes()).toContain('bg-danger-lt')
    expect(wrapper.find('.run-detail').text()).toContain("could not activate storage 'pbs-immich'")
  })

  it('exposes the full PVE outcome as a tooltip, since the line is clipped', () => {
    const wrapper = mountTab({ runs: [RUN_ERROR] })
    expect(wrapper.find('.run-detail').attributes('title')).toBe(RUN_ERROR.exit_status)
  })

  it('shows no detail line for a successful run', () => {
    const wrapper = mountTab({ runs: [{ ...RUN_OK, status: 'OK', exit_status: 'OK' }] })

    expect(wrapper.find('tbody .badge').text()).toBe('OK')
    expect(wrapper.find('.run-detail').exists()).toBe(false)
  })

  it('shows no detail line for a storage-derived run, which has no PVE outcome', () => {
    // Those rows come from a retained backup file, not a task.
    const wrapper = mountTab({ runs: [{ ...RUN_OK, status: 'OK', exit_status: '', task_upid: '' }] })

    expect(wrapper.find('tbody .badge').text()).toBe('OK')
    expect(wrapper.find('.run-detail').exists()).toBe(false)
  })

  it('badges a run that ended with warnings as a warning', () => {
    const wrapper = mountTab({ runs: [{ ...RUN_OK, status: 'warnings', exit_status: 'WARNINGS: 1' }] })

    const badge = wrapper.find('tbody .badge')
    expect(badge.text()).toBe('Avertissements')
    expect(badge.classes()).toContain('bg-warning-lt')
  })
})

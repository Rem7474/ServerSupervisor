import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Host } from '../types/host'
import { useHostsStore } from './hosts'
import { useDashboardStore } from './dashboard'

function makeHost(overrides: Partial<Host> = {}): Host {
  return {
    id: 'h1',
    name: 'host-1',
    hostname: 'host-1',
    ip_address: '10.0.0.1',
    status: 'online',
    ...overrides,
  } as Host
}

describe('stores/dashboard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reads hosts through the shared hosts store instead of holding its own copy', () => {
    const hostsStore = useHostsStore()
    const dashboard = useDashboardStore()

    hostsStore.setHosts([makeHost({ id: 'h1' })])
    expect(dashboard.hosts).toEqual(hostsStore.hosts)

    // A later push into the hosts store must be reflected without any
    // dashboard-side action — this is the single-source-of-truth invariant
    // (navbar badge and dashboard KPIs must never disagree).
    hostsStore.setHosts([makeHost({ id: 'h1' }), makeHost({ id: 'h2', status: 'offline' })])
    expect(dashboard.hosts).toHaveLength(2)
  })

  it('counts online/offline hosts from the shared store', () => {
    const hostsStore = useHostsStore()
    const dashboard = useDashboardStore()
    hostsStore.setHosts([
      makeHost({ id: 'h1', status: 'online' }),
      makeHost({ id: 'h2', status: 'online' }),
      makeHost({ id: 'h3', status: 'offline' }),
    ])

    expect(dashboard.onlineCount).toBe(2)
    expect(dashboard.offlineCount).toBe(1)
  })

  it('hasProxmox is false until a summary with at least one connection is set', () => {
    const dashboard = useDashboardStore()
    expect(dashboard.hasProxmox).toBe(false)

    dashboard.setProxmoxSummary({ connection_count: 0 })
    expect(dashboard.hasProxmox).toBe(false)

    dashboard.setProxmoxSummary({ connection_count: 1 })
    expect(dashboard.hasProxmox).toBe(true)
  })

  it('proxmoxStoragePct guards against a missing/zero total instead of dividing by zero', () => {
    const dashboard = useDashboardStore()
    expect(dashboard.proxmoxStoragePct).toBe(0)

    dashboard.setProxmoxSummary({ storage_used: 50, storage_total: 0 })
    expect(dashboard.proxmoxStoragePct).toBe(0)

    dashboard.setProxmoxSummary({ storage_used: 25, storage_total: 100 })
    expect(dashboard.proxmoxStoragePct).toBe(25)
  })

  it('counts an outdated Docker image only once it has actually run or been confirmed', () => {
    const dashboard = useDashboardStore()

    // Not up to date, but never observed running nor confirmed — e.g. a
    // freshly-added tracker with no container yet. Must not count as outdated.
    dashboard.setVersionComparisons([{ is_up_to_date: false }])
    expect(dashboard.outdatedDockerImages).toBe(0)

    dashboard.setVersionComparisons([{ is_up_to_date: false, running_version: 'v1.0' }])
    expect(dashboard.outdatedDockerImages).toBe(1)

    dashboard.setVersionComparisons([{ is_up_to_date: false, update_confirmed: true }])
    expect(dashboard.outdatedDockerImages).toBe(1)

    dashboard.setVersionComparisons([{ is_up_to_date: true, running_version: 'v1.0' }])
    expect(dashboard.outdatedDockerImages).toBe(0)
  })

  it('outdatedVersions sums outdated Docker images and pending APT packages', () => {
    const dashboard = useDashboardStore()
    dashboard.setVersionComparisons([{ is_up_to_date: false, running_version: 'v1.0' }])
    dashboard.setAptPending(3)

    expect(dashboard.outdatedVersions).toBe(4)
  })
})

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { runReleaseTracker } = vi.hoisted(() => ({ runReleaseTracker: vi.fn() }))
vi.mock('../api', () => ({ default: { runReleaseTracker } }))

import { useDockerContainerVersions } from './useDockerContainerVersions'
import type { Container, VersionComparison } from '../types/dockerContainersTab'

const container: Container = { id: 'c1', name: 'web', host_id: 'h1', image: 'nginx', image_tag: '1.25', state: 'running' }

function setup(vcs: VersionComparison[], canRun = true) {
  let result!: ReturnType<typeof useDockerContainerVersions>
  mount(defineComponent({
    setup() {
      result = useDockerContainerVersions(ref(vcs), () => canRun)
      return () => null
    },
  }))
  return result
}

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

describe('useDockerContainerVersions', () => {
  it('prefers the exact-tag ambient row over the tracker row', () => {
    const ambient: VersionComparison = { host_id: 'h1', docker_image: 'nginx', image_tag: '1.25', status: 'up_to_date' }
    const tracker: VersionComparison = { host_id: 'h1', docker_image: 'nginx', tracker_id: 't1' }
    expect(setup([tracker, ambient]).containerVersion(container)).toEqual(ambient)
    expect(setup([tracker]).containerVersion(container)).toEqual(tracker)
    expect(setup([]).containerVersion(container)).toBeNull()
  })

  it('only enables a tracker run for permitted users once a latest version is known', () => {
    const vc: VersionComparison = { host_id: 'h1', docker_image: 'nginx', tracker_id: 't1', latest_version: '1.26' }
    expect(setup([vc]).isTrackerRunDisabled(vc)).toBe(false)
    expect(setup([vc], false).isTrackerRunDisabled(vc)).toBe(true)
    expect(setup([vc]).isTrackerRunDisabled({ ...vc, latest_version: ' ' })).toBe(true)
    expect(setup([vc]).isTrackerRunDisabled(null)).toBe(true)
  })

  it('does not call the API when the run is disabled and clears the loading flag afterwards', async () => {
    const vc: VersionComparison = { host_id: 'h1', docker_image: 'nginx', tracker_id: 't1', latest_version: '1.26' }
    const v = setup([vc])
    runReleaseTracker.mockResolvedValueOnce({})
    await v.runTracker(vc, container)
    expect(runReleaseTracker).toHaveBeenCalledWith('t1')
    expect(v.trackerRunLoading.value).toEqual({})
    expect(v.trackerFeedbackIsError.value).toBe(false)

    await setup([vc], false).runTracker(vc, container)
    expect(runReleaseTracker).toHaveBeenCalledTimes(1)
  })
})

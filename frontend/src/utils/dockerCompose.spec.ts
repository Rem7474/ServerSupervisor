import { describe, it, expect } from 'vitest'
import { getComposeInfo, isComposeServiceRedundant, isComposeContainer } from './dockerCompose'

describe('dockerCompose', () => {
  describe('getComposeInfo', () => {
    it('extracts project/service/workingDir/configFiles from compose labels', () => {
      const info = getComposeInfo({
        'com.docker.compose.project': 'nextcloud',
        'com.docker.compose.service': 'app',
        'com.docker.compose.project.working_dir': '/srv/nextcloud',
        'com.docker.compose.project.config_files': '/srv/nextcloud/docker-compose.yml',
      })
      expect(info).toEqual({
        project: 'nextcloud',
        service: 'app',
        workingDir: '/srv/nextcloud',
        configFiles: '/srv/nextcloud/docker-compose.yml',
      })
    })

    it('returns empty strings for a standalone (non-compose) container', () => {
      expect(getComposeInfo(undefined)).toEqual({ project: '', service: '', workingDir: '', configFiles: '' })
      expect(getComposeInfo({})).toEqual({ project: '', service: '', workingDir: '', configFiles: '' })
    })
  })

  describe('isComposeServiceRedundant', () => {
    it('is true when the service name is just a normalized copy of the project name', () => {
      expect(isComposeServiceRedundant({
        'com.docker.compose.project': 'nextcloud',
        'com.docker.compose.service': 'Nextcloud',
      })).toBe(true)
    })

    it('is false when the service name is genuinely distinct', () => {
      expect(isComposeServiceRedundant({
        'com.docker.compose.project': 'nextcloud',
        'com.docker.compose.service': 'db',
      })).toBe(false)
    })

    it('is true (nothing to show twice) for a standalone container', () => {
      expect(isComposeServiceRedundant(undefined)).toBe(true)
    })
  })

  describe('isComposeContainer', () => {
    it('is true when the compose project label is present', () => {
      expect(isComposeContainer({ 'com.docker.compose.project': 'nextcloud' })).toBe(true)
    })

    it('is false for a standalone container', () => {
      expect(isComposeContainer({})).toBe(false)
      expect(isComposeContainer(undefined)).toBe(false)
    })
  })
})

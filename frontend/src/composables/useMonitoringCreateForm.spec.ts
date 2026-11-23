import { describe, it, expect } from 'vitest'
import { parseCertTarget } from './useMonitoringCreateForm'

describe('parseCertTarget', () => {
  it('takes hostname and explicit port from an http(s) URL', () => {
    expect(parseCertTarget('https://api.example.com:8443/health')).toEqual({ host: 'api.example.com', port: 8443 })
  })

  it('falls back to the scheme default port for a URL without one', () => {
    expect(parseCertTarget('https://api.example.com/x')).toEqual({ host: 'api.example.com', port: 443 })
    expect(parseCertTarget('http://api.example.com')).toEqual({ host: 'api.example.com', port: 80 })
  })

  it('splits host:port for tcp targets and ignores a non-numeric port', () => {
    expect(parseCertTarget('db.example.com:5432')).toEqual({ host: 'db.example.com', port: 5432 })
    expect(parseCertTarget('db.example.com:abc')).toEqual({ host: 'db.example.com', port: 443 })
  })

  it('keeps a bare host (icmp) and drops any path', () => {
    expect(parseCertTarget('example.com')).toEqual({ host: 'example.com', port: 443 })
    expect(parseCertTarget('example.com/path')).toEqual({ host: 'example.com', port: 443 })
  })
})

import { describe, it, expect, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { parseCertTarget, useMonitoringCreateForm } from './useMonitoringCreateForm'
import type { ProbeForm } from './useUptimeProbes'
import type { CertForm } from './useSslCertificates'

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

describe('useMonitoringCreateForm', () => {
  function probe(): ProbeForm {
    return {
      id: '', name: '', type: 'http', target: '', interval_sec: 60, timeout_sec: 10, expected_status: 200,
      expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true,
    }
  }
  function cert(): CertForm {
    return { id: '', name: '', host: '', port: 443, server_name: '', enabled: true }
  }

  function setup(opts: { probeOpen?: boolean; certOpen?: boolean; saveProbe?: () => Promise<void>; saveCert?: () => Promise<void> } = {}) {
    const probeForm = ref(probe())
    const certForm = ref(cert())
    const probeOpen = ref(opts.probeOpen ?? true)
    const certOpen = ref(opts.certOpen ?? false)
    const saveProbe = vi.fn(opts.saveProbe ?? (async () => { probeOpen.value = false }))
    const saveCert = vi.fn(opts.saveCert ?? (async () => { certOpen.value = false }))
    const api = useMonitoringCreateForm({
      probeForm, certForm,
      isProbeModalOpen: () => probeOpen.value,
      isCertModalOpen: () => certOpen.value,
      saveProbe, saveCert,
    })
    return { probeForm, certForm, probeOpen, certOpen, saveProbe, saveCert, ...api }
  }

  it('starts with the toggle matching the modal the entry point opened', () => {
    const probeOnly = setup({ probeOpen: true, certOpen: false })
    expect([probeOnly.createIncludeProbe.value, probeOnly.createIncludeCert.value]).toEqual([true, false])
    const certOnly = setup({ probeOpen: false, certOpen: true })
    expect([certOnly.createIncludeProbe.value, certOnly.createIncludeCert.value]).toEqual([false, true])
  })

  it('never lets the last selected resource be toggled off', () => {
    const s = setup({ probeOpen: true, certOpen: false })
    s.toggleIncludeProbe()
    expect(s.createIncludeProbe.value).toBe(true)
    s.toggleIncludeCert()
    expect(s.createIncludeCert.value).toBe(true)
    s.toggleIncludeProbe()
    expect(s.createIncludeProbe.value).toBe(false)
    s.toggleIncludeCert() // cert is now the only one left
    expect(s.createIncludeCert.value).toBe(true)
  })

  it('sharedName reads and writes the probe name, or the cert name when probe is off', () => {
    const s = setup({ probeOpen: true, certOpen: true })
    s.sharedName.value = 'both'
    expect(s.probeForm.value.name).toBe('both')
    expect(s.certForm.value.name).toBe('both')

    s.toggleIncludeProbe()
    s.sharedName.value = 'cert only'
    expect(s.probeForm.value.name).toBe('both')
    expect(s.certForm.value.name).toBe('cert only')
    expect(s.sharedName.value).toBe('cert only')
  })

  it('derives cert host/port from the probe target while both are selected', async () => {
    const s = setup({ probeOpen: true, certOpen: true })
    s.probeForm.value.target = 'https://api.example.com:8443/x'
    await nextTick()
    expect(s.certForm.value.host).toBe('api.example.com')
    expect(s.certForm.value.port).toBe(8443)

    s.probeForm.value.target = '  '
    await nextTick()
    expect(s.certForm.value.host).toBe('')
  })

  it('does not touch the cert form when only the probe is selected', async () => {
    const s = setup({ probeOpen: true, certOpen: false })
    s.probeForm.value.target = 'example.com'
    await nextTick()
    expect(s.certForm.value.host).toBe('')
  })

  it('saves the probe then the cert when both are selected', async () => {
    const order: string[] = []
    const s = setup({
      probeOpen: true, certOpen: true,
      saveProbe: async () => { order.push('probe') },
      saveCert: async () => { order.push('cert') },
    })
    // saveProbe above leaves the probe modal closed from the composable's view
    s.probeOpen.value = false
    await s.submitCreateModal()
    expect(order).toEqual(['probe', 'cert'])
  })

  it('skips the cert when the probe save failed (modal still open)', async () => {
    const s = setup({ probeOpen: true, certOpen: true, saveProbe: async () => { /* error shown, stays open */ } })
    await s.submitCreateModal()
    expect(s.saveProbe).toHaveBeenCalledOnce()
    expect(s.saveCert).not.toHaveBeenCalled()
  })

  it('saves only the cert when the probe is not selected', async () => {
    const s = setup({ probeOpen: false, certOpen: true })
    await s.submitCreateModal()
    expect(s.saveProbe).not.toHaveBeenCalled()
    expect(s.saveCert).toHaveBeenCalledOnce()
  })

  it('saves only the probe when the cert is not selected', async () => {
    const s = setup({ probeOpen: true, certOpen: false })
    await s.submitCreateModal()
    expect(s.saveProbe).toHaveBeenCalledOnce()
    expect(s.saveCert).not.toHaveBeenCalled()
  })
})

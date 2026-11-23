import { describe, it, expect, beforeEach } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useProbeTargetHints } from './useProbeTargetHints'
import type { ProbeForm } from './useUptimeProbes'

function probe(type: string): ProbeForm {
  return {
    id: '', name: '', type, target: '', interval_sec: 60, timeout_sec: 10, expected_status: 200,
    expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true,
  }
}

function setup(type: string) {
  const form = ref(probe(type))
  let hints!: ReturnType<typeof useProbeTargetHints>
  mount(defineComponent({
    setup() {
      hints = useProbeTargetHints(form)
      return () => null
    },
  }))
  return { form, hints }
}

describe('useProbeTargetHints', () => {
  beforeEach(() => setLocale('fr'))

  it('asks for a URL for http probes', () => {
    const { hints } = setup('http')
    expect(hints.probeTargetLabel.value).toBe('URL')
    expect(hints.probeTargetPlaceholder.value).toBe('https://example.com/health')
  })

  it('asks for a host or IP for icmp probes', () => {
    const { hints } = setup('icmp')
    expect(hints.probeTargetLabel.value).toBe('Hôte ou IP')
    expect(hints.probeTargetPlaceholder.value).toBe('192.168.1.1 ou switch.local')
  })

  it('asks for host:port for tcp probes', () => {
    const { hints } = setup('tcp')
    expect(hints.probeTargetLabel.value).toBe('host:port')
    expect(hints.probeTargetPlaceholder.value).toBe('example.com:443')
  })

  it('follows the probe type reactively', () => {
    const { form, hints } = setup('http')
    form.value.type = 'icmp'
    expect(hints.probeTargetLabel.value).toBe('Hôte ou IP')
    form.value.type = 'tcp'
    expect(hints.probeTargetPlaceholder.value).toBe('example.com:443')
  })
})

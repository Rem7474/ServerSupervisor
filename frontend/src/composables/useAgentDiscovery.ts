import { computed, reactive, ref } from 'vue'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import { i18n } from '../i18n'
import { useCommandStream } from './useCommandStream'
import type { Host } from '../types/host'
import type { AgentDiscoveryKind, DiscoveryJob, DiscoveryLimits, SNMPProbeRequest } from '../types/discovery'

export const JOB_POLL_INTERVAL_MS = 1500

/** Port-scan progress as streamed by the agent (one JSON object per chunk). */
export interface ScanProgress {
  done: number
  total: number
  open: number
}

/** SNMP form state. Credential fields are write-only: cleared once sent. */
export interface SnmpForm {
  target: string
  port: number
  version: '2c' | '3'
  includeInterfaces: boolean
  community: string
  username: string
  securityLevel: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv'
  authProtocol: 'SHA' | 'SHA256'
  authPassphrase: string
  privPassphrase: string
}

function isTerminal(status: string): boolean {
  return status === 'completed' || status === 'failed'
}

/**
 * Number of addresses a port scan probes for an IPv4 CIDR: network and
 * broadcast are skipped up to /30, as the agent does. null when the value
 * is not a CIDR.
 */
export function cidrHostCount(cidr: string): number | null {
  const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\/(\d{1,2})$/.exec(cidr.trim())
  if (!m) return null
  if (m.slice(1, 5).some((o) => Number(o) > 255)) return null
  const bits = Number(m[5])
  if (bits > 32) return null
  const size = 2 ** (32 - bits)
  return bits <= 30 ? size - 2 : size
}

/**
 * Number of distinct ports in a "22,80,8000-8100" spec; 0 for an empty spec
 * (server defaults), null when the spec does not parse.
 */
export function countPorts(spec: string): number | null {
  const seen = new Set<number>()
  for (const raw of spec.split(',')) {
    const part = raw.trim()
    if (!part) continue
    const m = /^(\d{1,5})(?:\s*-\s*(\d{1,5}))?$/.exec(part)
    if (!m) return null
    const start = Number(m[1])
    const end = m[2] ? Number(m[2]) : start
    if (start < 1 || end > 65535 || start > end) return null
    for (let p = start; p <= end; p++) seen.add(p)
  }
  return seen.size
}

/**
 * Agent-run discovery: pick an agent on the target LAN, start an ARP read,
 * a bounded port scan or an SNMP probe, follow it until it ends (polling the
 * job, plus the live progress stream for the port scan) and cancel it.
 * One job per tool, so switching tabs does not lose a running scan.
 */
export function useAgentDiscovery() {
  const { t } = i18n.global
  const { openCommandStream, closeStream } = useCommandStream()

  const hosts = ref<Host[]>([])
  const hostsLoaded = ref(false)
  const loadError = ref('')
  const limits = ref<DiscoveryLimits | null>(null)
  const hostId = ref('')

  const agents = computed(() => hosts.value.filter((h) => h.collectors?.network_discovery))
  const selectedHost = computed(() => agents.value.find((h) => h.id === hostId.value) ?? null)
  const scanAllowed = computed(() => !!selectedHost.value?.collectors?.network_scan)

  const jobs = reactive<Record<AgentDiscoveryKind, DiscoveryJob | null>>({ arp: null, port_scan: null, snmp: null })
  const errors = reactive<Record<AgentDiscoveryKind, string>>({ arp: '', port_scan: '', snmp: '' })
  const starting = reactive<Record<AgentDiscoveryKind, boolean>>({ arp: false, port_scan: false, snmp: false })
  const cancelling = reactive<Record<AgentDiscoveryKind, boolean>>({ arp: false, port_scan: false, snmp: false })
  const progress = ref<ScanProgress | null>(null)
  const timers: Partial<Record<AgentDiscoveryKind, ReturnType<typeof setTimeout>>> = {}

  const cidr = ref('')
  const ports = ref('')
  const snmp = reactive<SnmpForm>({
    target: '',
    port: 161,
    version: '2c',
    includeInterfaces: true,
    community: '',
    username: '',
    securityLevel: 'authPriv',
    authProtocol: 'SHA256',
    authPassphrase: '',
    privPassphrase: '',
  })

  async function load(): Promise<void> {
    loadError.value = ''
    try {
      const [hostsRes, limitsRes] = await Promise.all([apiClient.getHosts(), apiClient.getDiscoveryLimits()])
      hosts.value = hostsRes.data ?? []
      limits.value = limitsRes.data
      if (!hostId.value && agents.value.length) hostId.value = agents.value[0].id
    } catch (e: unknown) {
      loadError.value = getApiErrorMessage(e, t('discovery.loadError'))
    } finally {
      hostsLoaded.value = true
    }
  }

  const portCount = computed(() => countPorts(ports.value))
  const effectivePortCount = computed(() =>
    portCount.value === 0 ? (limits.value?.default_ports.length ?? 0) : (portCount.value ?? 0),
  )
  const addressCount = computed(() => cidrHostCount(cidr.value))

  /** Client-side mirror of the server checks, for an immediate hint. */
  const portScanProblem = computed(() => {
    const min = limits.value?.min_prefix_len ?? 24
    if (cidr.value.trim() && addressCount.value === null) return t('discovery.cidrInvalid')
    const bits = Number(cidr.value.split('/')[1])
    if (cidr.value.trim() && bits < min) return t('discovery.cidrTooWide', { min })
    if (portCount.value === null) return t('discovery.portsInvalid')
    const max = limits.value?.max_ports ?? 1024
    if (portCount.value > max) return t('discovery.portsTooMany', { max })
    return ''
  })

  function isActive(kind: AgentDiscoveryKind): boolean {
    const job = jobs[kind]
    return !!job && !isTerminal(job.status)
  }

  function stopFollowing(kind: AgentDiscoveryKind): void {
    const timer = timers[kind]
    if (timer) clearTimeout(timer)
    delete timers[kind]
    if (kind === 'port_scan') closeStream()
  }

  async function poll(kind: AgentDiscoveryKind, id: string): Promise<void> {
    try {
      const res = await apiClient.getDiscoveryJob(id)
      if (jobs[kind]?.id !== id) return // replaced by a newer job
      jobs[kind] = res.data
    } catch (e: unknown) {
      errors[kind] = getApiErrorMessage(e, t('discovery.pollError'))
      stopFollowing(kind)
      return
    }
    if (isTerminal(jobs[kind]!.status)) {
      stopFollowing(kind)
      return
    }
    timers[kind] = setTimeout(() => void poll(kind, id), JOB_POLL_INTERVAL_MS)
  }

  function follow(kind: AgentDiscoveryKind, id: string): void {
    stopFollowing(kind)
    jobs[kind] = { id, host_id: hostId.value, kind, status: 'pending', created_at: new Date().toISOString() }
    if (kind === 'port_scan') {
      progress.value = null
      openCommandStream(id, {
        onChunk: (msg) => {
          try {
            const p = JSON.parse(msg.chunk) as ScanProgress
            if (typeof p.done === 'number' && typeof p.total === 'number') progress.value = p
          } catch {
            // not a progress chunk
          }
        },
      })
    }
    void poll(kind, id)
  }

  /** Starts a job; resolves true once the server accepted it. */
  async function start(kind: AgentDiscoveryKind, call: () => Promise<{ data: { job_id: string } }>): Promise<boolean> {
    if (!hostId.value || isActive(kind)) return false
    errors[kind] = ''
    starting[kind] = true
    try {
      const res = await call()
      follow(kind, res.data.job_id)
      return true
    } catch (e: unknown) {
      errors[kind] = getApiErrorMessage(e, t('discovery.startError'))
      return false
    } finally {
      starting[kind] = false
    }
  }

  function startArp(): Promise<boolean> {
    return start('arp', () => apiClient.startArpDiscovery(hostId.value))
  }

  function startPortScan(): Promise<boolean> {
    return start('port_scan', () =>
      apiClient.startPortScan({ host_id: hostId.value, cidr: cidr.value.trim(), ports: ports.value.trim() }),
    )
  }

  function snmpRequest(): SNMPProbeRequest {
    const v3 = snmp.version === '3'
    const auth = v3 && snmp.securityLevel !== 'noAuthNoPriv'
    const priv = v3 && snmp.securityLevel === 'authPriv'
    return {
      host_id: hostId.value,
      target: snmp.target.trim(),
      port: snmp.port || 161,
      version: snmp.version,
      include_interfaces: snmp.includeInterfaces,
      community: v3 ? '' : snmp.community,
      username: v3 ? snmp.username.trim() : '',
      security_level: v3 ? snmp.securityLevel : '',
      auth_protocol: auth ? snmp.authProtocol : '',
      auth_passphrase: auth ? snmp.authPassphrase : '',
      priv_protocol: priv ? 'AES' : '',
      priv_passphrase: priv ? snmp.privPassphrase : '',
    }
  }

  async function startSnmp(): Promise<boolean> {
    const req = snmpRequest()
    const accepted = await start('snmp', () => apiClient.startSnmpProbe(req))
    // Credentials are write-only: dropped from the page once the server
    // accepted them, kept after a refusal so the user can fix and resend.
    if (accepted) {
      snmp.community = ''
      snmp.authPassphrase = ''
      snmp.privPassphrase = ''
    }
    return accepted
  }

  async function cancel(kind: AgentDiscoveryKind): Promise<void> {
    const job = jobs[kind]
    if (!job || isTerminal(job.status)) return
    cancelling[kind] = true
    try {
      const res = await apiClient.cancelDiscoveryJob(job.id)
      jobs[kind] = res.data
      if (isTerminal(res.data.status)) stopFollowing(kind)
    } catch (e: unknown) {
      errors[kind] = getApiErrorMessage(e, t('discovery.cancelError'))
    } finally {
      cancelling[kind] = false
    }
  }

  function dispose(): void {
    for (const kind of Object.keys(timers) as AgentDiscoveryKind[]) stopFollowing(kind)
    closeStream()
  }

  return {
    hosts,
    hostsLoaded,
    loadError,
    limits,
    hostId,
    agents,
    selectedHost,
    scanAllowed,
    jobs,
    errors,
    starting,
    cancelling,
    progress,
    cidr,
    ports,
    snmp,
    addressCount,
    effectivePortCount,
    portScanProblem,
    load,
    isActive,
    startArp,
    startPortScan,
    startSnmp,
    cancel,
    dispose,
  }
}

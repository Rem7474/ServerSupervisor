import { api } from './client'
import type {
  DiscoveryJob,
  DiscoveryJobStarted,
  DiscoveryLimits,
  PortScanRequest,
  SNMPProbeRequest,
} from '../types/discovery'

// Agent-run network discovery (ARP table, TCP port scan, SNMP probe). Each
// start call queues a command on the chosen agent and returns a job id to
// poll; the ping sweep stays in hosts.ts (discoverHosts).
export const discoveryApi = {
  getDiscoveryLimits: () => api.get<DiscoveryLimits>('/v1/discovery/limits'),
  startArpDiscovery: (hostId: string) =>
    api.post<DiscoveryJobStarted>('/v1/discovery/arp', { host_id: hostId }),
  startPortScan: (req: PortScanRequest) =>
    api.post<DiscoveryJobStarted>('/v1/discovery/port-scan', req),
  startSnmpProbe: (req: SNMPProbeRequest) =>
    api.post<DiscoveryJobStarted>('/v1/discovery/snmp', req),
  getDiscoveryJob: (id: string) => api.get<DiscoveryJob>(`/v1/discovery/jobs/${id}`),
  cancelDiscoveryJob: (id: string) => api.post<DiscoveryJob>(`/v1/discovery/jobs/${id}/cancel`),
}

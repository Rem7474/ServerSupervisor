// Network discovery types, generated from the Go model: the server-side
// subnet ping sweep and the agent-run probes (ARP, port scan, SNMP).
export type {
  NetworkScanRequest,
  DiscoveredHost,
  DiscoveryLimits,
  DiscoveryJob,
  DiscoveryJobStarted,
  PortScanRequest,
  PortScanResult,
  PortScanHost,
  SNMPProbeRequest,
  SNMPResult,
  SNMPInterface,
  ARPResult,
  ARPNeighbor,
} from './generated'

/** Agent-run discovery tool, as the server names the job kind. */
export type AgentDiscoveryKind = 'arp' | 'port_scan' | 'snmp'

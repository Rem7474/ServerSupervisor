/**
 * Well-known-port → service-name heuristic for the host "Trafic réseau" tab.
 *
 * This is a **guess**, not a classification. conntrack gives us layer 4 only
 * (tcp/udp), so the only thing available client-side is "port 443 is usually
 * HTTPS" — a host is perfectly free to run anything on any port, and this map
 * will confidently mislabel it when it does. Every call site must present the
 * result as a heuristic (see `PORT_GUESS_HINT`), never as a fact.
 *
 * The real signal, when it exists, is the agent's TLS SNI capture
 * (`server_name` on a talker — optional, off by default, needs CAP_NET_RAW).
 * That always wins over this file; this is the fallback.
 *
 * Intentionally not merged with `utils/dockerPorts.ts`: that module parses
 * Docker's port-mapping *syntax* (`0.0.0.0:8080->80/tcp`) and has no notion of
 * what a port means. Different concern, no overlap to reuse.
 */

/** Ports that mean the same service on both TCP and UDP (or where it doesn't matter). */
const COMMON_PORTS: Record<number, string> = {
  20: 'FTP-DATA',
  21: 'FTP',
  22: 'SSH',
  23: 'Telnet',
  25: 'SMTP',
  53: 'DNS',
  67: 'DHCP',
  68: 'DHCP',
  69: 'TFTP',
  80: 'HTTP',
  110: 'POP3',
  111: 'RPC',
  119: 'NNTP',
  123: 'NTP',
  135: 'MS-RPC',
  137: 'NetBIOS',
  138: 'NetBIOS',
  139: 'NetBIOS',
  143: 'IMAP',
  161: 'SNMP',
  162: 'SNMP-Trap',
  179: 'BGP',
  389: 'LDAP',
  443: 'HTTPS',
  445: 'SMB',
  465: 'SMTPS',
  514: 'Syslog',
  515: 'LPD',
  546: 'DHCPv6',
  547: 'DHCPv6',
  587: 'SMTP',
  623: 'IPMI',
  636: 'LDAPS',
  873: 'rsync',
  993: 'IMAPS',
  995: 'POP3S',
  1080: 'SOCKS',
  1194: 'OpenVPN',
  1433: 'MSSQL',
  1521: 'Oracle',
  1701: 'L2TP',
  1723: 'PPTP',
  1883: 'MQTT',
  2049: 'NFS',
  2375: 'Docker',
  2376: 'Docker',
  2377: 'Swarm',
  3000: 'HTTP-alt',
  3128: 'Squid',
  3306: 'MySQL',
  3389: 'RDP',
  4444: 'HTTP-alt',
  4646: 'Nomad',
  5000: 'HTTP-alt',
  5432: 'PostgreSQL',
  5433: 'PostgreSQL',
  5060: 'SIP',
  5061: 'SIPS',
  5222: 'XMPP',
  5353: 'mDNS',
  5672: 'AMQP',
  5900: 'VNC',
  5984: 'CouchDB',
  6379: 'Redis',
  6443: 'Kubernetes',
  8006: 'Proxmox',
  8080: 'HTTP-alt',
  8081: 'HTTP-alt',
  8086: 'InfluxDB',
  8123: 'Home Assistant',
  8443: 'HTTPS-alt',
  8883: 'MQTTS',
  9000: 'HTTP-alt',
  9090: 'Prometheus',
  9091: 'Transmission',
  9100: 'node_exporter',
  9200: 'Elasticsearch',
  9300: 'Elasticsearch',
  9418: 'Git',
  11211: 'Memcached',
  15672: 'RabbitMQ',
  19999: 'Netdata',
  25565: 'Minecraft',
  27017: 'MongoDB',
  51820: 'WireGuard',
}

/** Ports whose meaning differs by transport protocol. */
const UDP_ONLY_PORTS: Record<number, string> = {
  500: 'IPsec',
  4500: 'IPsec-NAT',
  3478: 'STUN',
  27015: 'Steam',
}

/** Shown wherever a guessed label appears, so the uncertainty is never implicit. */
export const PORT_GUESS_HINT = 'Détection par port, non garantie — le port ne prouve pas le protocole applicatif'

/**
 * Best-effort application-protocol name for a remote port, or `''` when the
 * port carries no useful convention (ephemeral/high ports, unknown services).
 *
 * Returning `''` rather than `'inconnu'` is deliberate: the caller decides how
 * to render "no guess", and an empty string keeps it out of filter option lists.
 */
export function guessServiceForPort(port: number | undefined | null, protocol?: string): string {
  if (typeof port !== 'number' || !Number.isFinite(port) || port <= 0 || port > 65535) {
    return ''
  }
  if (protocol === 'udp') {
    const udp = UDP_ONLY_PORTS[port]
    if (udp) return udp
  }
  return COMMON_PORTS[port] ?? ''
}

/**
 * The label to show for a talker: the agent's real TLS SNI hostname when the
 * optional capture provided one, otherwise the port guess.
 *
 * `source` is what tells the UI whether to mark the value as uncertain —
 * `'sni'` is observed on the wire, `'port'` is a guess, `'none'` means show
 * nothing.
 */
export interface ProtocolLabel {
  text: string
  source: 'sni' | 'port' | 'none'
}

export function protocolLabelFor(
  serverName: string | undefined | null,
  port: number | undefined | null,
  protocol?: string,
): ProtocolLabel {
  const sni = (serverName ?? '').trim()
  if (sni) {
    return { text: sni, source: 'sni' }
  }
  const guess = guessServiceForPort(port, protocol)
  return guess ? { text: guess, source: 'port' } : { text: '', source: 'none' }
}

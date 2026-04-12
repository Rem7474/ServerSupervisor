export interface DockerPortMapping {
  internalPort: string
  hostPort: string
  proto: string
  ip: string
}

export interface DockerPortBadgeGroup {
  key: string
  mapping: DockerPortMapping
  hasGlobalIPv6: boolean
}

function toStringValue(value: unknown): string {
  if (value === undefined || value === null) {
    return ''
  }
  return String(value).trim()
}

function isInternalToken(token: string): boolean {
  return /\/(tcp|udp|sctp)$/i.test(token.trim())
}

function splitTokens(rawPorts: unknown): string[] {
  return String(rawPorts)
    .split(/,|\n/)
    .map((token) => token.trim())
    .filter(Boolean)
}

function parseHostToken(token: string): { hostPort: string; ip: string } {
  const cleaned = token.replace(/\/[a-zA-Z0-9]+\s*$/, '').trim()

  const bracketIpMatch = cleaned.match(/^\[([^\]]+)\]:(\d+)$/)
  if (bracketIpMatch) {
    return { ip: bracketIpMatch[1], hostPort: bracketIpMatch[2] }
  }

  const endPortMatch = cleaned.match(/:(\d+)$/)
  if (endPortMatch) {
    return { ip: cleaned.replace(/:(\d+)$/, ''), hostPort: endPortMatch[1] }
  }

  const directPortMatch = cleaned.match(/^(\d+)$/)
  if (directPortMatch) {
    return { ip: '', hostPort: directPortMatch[1] }
  }

  return { ip: '', hostPort: '' }
}

function parseInternalToken(token: string): { internalPort: string; proto: string } {
  const trimmed = token.trim()
  const protoMatch = trimmed.match(/\/([a-zA-Z0-9]+)\s*$/)
  const proto = protoMatch ? protoMatch[1].toLowerCase() : ''
  const withoutProto = trimmed.replace(/\/[a-zA-Z0-9]+\s*$/, '').trim()

  const portMatch = withoutProto.match(/(\d+)/)
  return {
    internalPort: portMatch ? portMatch[1] : '',
    proto,
  }
}

function parseToken(token: string): DockerPortMapping {
  if (token.includes('->')) {
    const [leftRaw, rightRaw] = token.split('->')
    const left = toStringValue(leftRaw)
    const right = toStringValue(rightRaw)

    const leftInternal = isInternalToken(left)
    const rightInternal = isInternalToken(right)

    const internalSource = leftInternal && !rightInternal ? left : right
    const hostSource = leftInternal && !rightInternal ? right : left

    const internal = parseInternalToken(internalSource)
    const host = parseHostToken(hostSource)

    return {
      internalPort: internal.internalPort,
      hostPort: host.hostPort,
      proto: internal.proto,
      ip: host.ip,
    }
  }

  if (isInternalToken(token)) {
    const internal = parseInternalToken(token)
    return {
      internalPort: internal.internalPort,
      hostPort: '',
      proto: internal.proto,
      ip: '',
    }
  }

  const host = parseHostToken(token)
  return {
    internalPort: '',
    hostPort: host.hostPort,
    proto: '',
    ip: host.ip,
  }
}

function normalizeIp(value: string): string {
  return value.trim().toLowerCase()
}

function isGlobalIp(value: string): boolean {
  const normalized = normalizeIp(value)
  return normalized === '' || normalized === '0.0.0.0' || normalized === '::' || normalized === ':::'
}

function isGlobalIPv6(value: string): boolean {
  const normalized = normalizeIp(value)
  return normalized === '::' || normalized === ':::'
}

function portDisplayGroupKey(mapping: DockerPortMapping): string {
  return `${mapping.internalPort || 'none'}|${mapping.hostPort || 'none'}|${mapping.proto || 'na'}`
}

export function normalizeDockerPorts(raw: unknown): DockerPortMapping[] {
  if (!raw) {
    return []
  }

  const normalized: DockerPortMapping[] = []

  const pushPort = (mapping: DockerPortMapping) => {
    if (!mapping.internalPort && !mapping.hostPort) {
      return
    }
    normalized.push(mapping)
  }

  if (Array.isArray(raw)) {
    for (const entry of raw) {
      if (typeof entry === 'string') {
        pushPort(parseToken(entry))
        continue
      }

      if (!entry || typeof entry !== 'object') {
        continue
      }

      const privatePort = toStringValue((entry as Record<string, unknown>).PrivatePort ?? (entry as Record<string, unknown>).private_port ?? (entry as Record<string, unknown>).privatePort ?? (entry as Record<string, unknown>).container_port ?? (entry as Record<string, unknown>).internalPort)
      const publicPort = toStringValue((entry as Record<string, unknown>).PublicPort ?? (entry as Record<string, unknown>).public_port ?? (entry as Record<string, unknown>).publicPort ?? (entry as Record<string, unknown>).host_port ?? (entry as Record<string, unknown>).hostPort)
      const proto = toStringValue((entry as Record<string, unknown>).Type ?? (entry as Record<string, unknown>).type ?? (entry as Record<string, unknown>).protocol ?? (entry as Record<string, unknown>).proto).toLowerCase()
      const ip = toStringValue((entry as Record<string, unknown>).IP ?? (entry as Record<string, unknown>).ip)

      pushPort({
        internalPort: privatePort,
        hostPort: publicPort,
        proto,
        ip,
      })
    }
  } else {
    for (const token of splitTokens(raw)) {
      pushPort(parseToken(token))
    }
  }

  const seen = new Set<string>()
  return normalized.filter((mapping) => {
    const key = `${mapping.internalPort}|${mapping.hostPort}|${mapping.proto}|${mapping.ip}`
    if (seen.has(key)) {
      return false
    }
    seen.add(key)
    return true
  })
}

export function formatInternalPort(mapping: DockerPortMapping): string {
  return mapping.proto ? `${mapping.internalPort}/${mapping.proto}` : mapping.internalPort
}

export function groupGlobalDockerPorts(ports: DockerPortMapping[]): DockerPortBadgeGroup[] {
  const groupedPorts: DockerPortBadgeGroup[] = []
  const groupedIndices = new Map<string, number>()

  for (const mapping of ports) {
    if (!mapping.hostPort) {
      const key = portDisplayGroupKey(mapping)
      if (groupedIndices.has(key)) {
        continue
      }

      groupedIndices.set(key, groupedPorts.length)
      groupedPorts.push({
        key,
        mapping,
        hasGlobalIPv6: false,
      })
      continue
    }

    if (!isGlobalIp(mapping.ip)) {
      groupedPorts.push({
        key: portMappingKey(mapping),
        mapping,
        hasGlobalIPv6: false,
      })
      continue
    }

    const key = portDisplayGroupKey(mapping)
    const existingIndex = groupedIndices.get(key)

    if (existingIndex === undefined) {
      groupedIndices.set(key, groupedPorts.length)
      groupedPorts.push({
        key,
        mapping,
        hasGlobalIPv6: isGlobalIPv6(mapping.ip),
      })
      continue
    }

    if (isGlobalIPv6(mapping.ip)) {
      groupedPorts[existingIndex].hasGlobalIPv6 = true
    }
  }

  return groupedPorts
}

export function formatExposedPort(mapping: DockerPortMapping): string {
  if (!mapping.hostPort) {
    return '—'
  }

  const ip = mapping.ip.trim()
  const visibleIp = ip && ip !== '0.0.0.0' && ip !== '::' && ip !== ':::' ? `${ip}:` : ''
  const proto = mapping.proto ? `/${mapping.proto}` : ''

  return `${visibleIp}${mapping.hostPort}${proto}`
}

export function portMappingKey(mapping: DockerPortMapping): string {
  return `${mapping.internalPort || 'none'}-${mapping.hostPort || 'none'}-${mapping.proto || 'na'}-${mapping.ip || 'all'}`
}

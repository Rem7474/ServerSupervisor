// Client-side heuristic only, used to route a free-text search box input to
// the right endpoint (domain-details vs IP-timeline). Doesn't need to be
// RFC-perfect — a wrong guess just returns an empty result, the backend
// endpoints are the actual source of truth.
const IPV4_RE = /^(\d{1,3}\.){3}\d{1,3}$/
const IPV6_RE = /^[0-9a-fA-F:]+$/

export function looksLikeIP(value: string): boolean {
  const v = value.trim()
  if (!v) return false
  if (IPV4_RE.test(v)) {
    return v.split('.').every((octet) => Number(octet) <= 255)
  }
  return v.includes(':') && IPV6_RE.test(v)
}

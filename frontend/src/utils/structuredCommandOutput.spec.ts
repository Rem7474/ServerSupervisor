import { describe, it, expect } from 'vitest'
import { resolveStructuredOutput } from './structuredCommandOutput'

describe('resolveStructuredOutput', () => {
  it('returns a processes kind for module=processes action=list with a valid JSON array', () => {
    const raw = JSON.stringify([{ pid: 1, name: 'systemd', user: 'root', cpu_pct: 0.1, mem_pct: 0.2, mem_rss_kb: 1024, state: 'S' }])
    const result = resolveStructuredOutput('processes', 'list', raw)
    expect(result?.kind).toBe('processes')
    expect(result?.data).toHaveLength(1)
  })

  it('returns null for module=processes action=list while streaming (invalid JSON)', () => {
    expect(resolveStructuredOutput('processes', 'list', '[{"pid":1,"name":"sys')).toBeNull()
  })

  it('returns null for module=processes action=list when the JSON is not an array', () => {
    expect(resolveStructuredOutput('processes', 'list', '{"pid":1}')).toBeNull()
  })

  it('returns null for an unrelated module/action', () => {
    expect(resolveStructuredOutput('apt', 'upgrade', 'Reading package lists...')).toBeNull()
  })

  it('returns null when output is empty/undefined', () => {
    expect(resolveStructuredOutput('processes', 'list', undefined)).toBeNull()
    expect(resolveStructuredOutput('processes', 'list', '')).toBeNull()
  })
})

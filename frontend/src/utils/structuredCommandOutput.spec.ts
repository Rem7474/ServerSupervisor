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

  it('returns a systemd kind for module=systemd action=list with a valid JSON array', () => {
    const raw = JSON.stringify([{ name: 'nginx.service', active_state: 'active' }])
    const result = resolveStructuredOutput('systemd', 'list', raw)
    expect(result?.kind).toBe('systemd')
    expect(result?.data).toHaveLength(1)
  })

  it('returns a restic_backup_summary kind for module=restic action=run_backup with a valid JSON object', () => {
    const raw = JSON.stringify({ status: 'ok', profile: 'files', duration_sec: 42 })
    const result = resolveStructuredOutput('restic', 'run_backup', raw)
    expect(result?.kind).toBe('restic_backup_summary')
    expect(result?.data).toEqual({ status: 'ok', profile: 'files', duration_sec: 42 })
  })

  it('returns null for module=restic action=run_backup when the JSON is an array, not an object', () => {
    expect(resolveStructuredOutput('restic', 'run_backup', '[1,2,3]')).toBeNull()
  })

  it('returns null for module=restic action=status (not a run_backup summary)', () => {
    const raw = JSON.stringify({ installed: true, source: 'restic_commands' })
    expect(resolveStructuredOutput('restic', 'status', raw)).toBeNull()
  })

  it('returns null for an unrelated module/action', () => {
    expect(resolveStructuredOutput('apt', 'upgrade', 'Reading package lists...')).toBeNull()
  })

  it('returns null when output is empty/undefined', () => {
    expect(resolveStructuredOutput('processes', 'list', undefined)).toBeNull()
    expect(resolveStructuredOutput('processes', 'list', '')).toBeNull()
  })
})

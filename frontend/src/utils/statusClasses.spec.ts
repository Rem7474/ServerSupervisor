import { describe, it, expect } from 'vitest'
import { getEntityStateClass, getEntityStateLabel } from './statusClasses'

describe('getEntityStateLabel', () => {
  it('translates every known entity state to French', () => {
    expect(getEntityStateLabel('running')).toBe('En cours')
    expect(getEntityStateLabel('stopped')).toBe('Arrêté')
    expect(getEntityStateLabel('exited')).toBe('Arrêté')
    expect(getEntityStateLabel('paused')).toBe('En pause')
    expect(getEntityStateLabel('online')).toBe('En ligne')
    expect(getEntityStateLabel('offline')).toBe('Hors ligne')
  })

  it('is case-insensitive, matching getEntityStateClass', () => {
    expect(getEntityStateLabel('RUNNING')).toBe('En cours')
  })

  it('falls back to the raw state for an unknown value', () => {
    expect(getEntityStateLabel('something-new')).toBe('something-new')
  })

  it('never leaks the raw English "running"/"stopped" for a known state', () => {
    // The bug this locks in: Proxmox used to render the raw English
    // "running" while Docker showed "En cours" for the exact same state —
    // both entity types now go through this same shared source.
    expect(getEntityStateLabel('running')).not.toBe('running')
    expect(getEntityStateLabel('stopped')).not.toBe('stopped')
    expect(getEntityStateClass('running')).toBe('badge bg-success-lt text-success')
  })
})

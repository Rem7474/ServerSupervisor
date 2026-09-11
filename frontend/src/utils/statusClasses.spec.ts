import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import {
  getEntityStateClass, getEntityStateLabel, getExecutionStateClass, getExecutionStateLabel,
  knownEntityStates, knownExecutionStates,
} from './statusClasses'

beforeEach(() => {
  setLocale('fr')
})

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

describe('getExecutionStateLabel / getExecutionStateClass — "ok" alias', () => {
  it('treats Proxmox vzdump\'s "OK" status the same as a successful run', () => {
    // Proxmox backup runs report status "OK", not "completed"/"success" like
    // every other execution-state consumer in the app.
    expect(getExecutionStateLabel('OK')).toBe('OK')
    expect(getExecutionStateClass('OK')).toBe('badge bg-success-lt text-success')
    expect(getExecutionStateClass('OK')).toBe(getExecutionStateClass('completed'))
  })

  it('is case-insensitive and falls back to the raw status for an unknown value', () => {
    expect(getExecutionStateLabel('ok')).toBe('OK')
    expect(getExecutionStateLabel('weird-status')).toBe('weird-status')
  })
})

describe('label/class parity — a color with no matching translated text', () => {
  // ENTITY_STATE_MAP/EXECUTION_STATE_MAP (color) and entityStateLabels()/
  // executionStateLabels() (text) are two independently-written object
  // literals that currently share the same keys. Nothing but this test
  // enforces that: a state added to one map and not the other would render a
  // correctly-colored badge with the raw backend string as its text — the
  // same class of bug this module exists to prevent at every call site,
  // just one level deeper, inside the module itself.
  it('has a translated label for every entity state that has a color', () => {
    for (const state of knownEntityStates()) {
      expect(getEntityStateLabel(state), `entity state "${state}" has a color but no label`).not.toBe(state)
    }
  })

  it('has a translated label for every execution state that has a color', () => {
    for (const status of knownExecutionStates()) {
      expect(getExecutionStateLabel(status), `execution state "${status}" has a color but no label`).not.toBe(status)
    }
  })
})

describe('locale reactivity', () => {
  it('re-translates labels when the active locale changes, instead of freezing at import time', () => {
    expect(getEntityStateLabel('running')).toBe('En cours')
    expect(getExecutionStateLabel('completed')).toBe('Terminé')

    setLocale('en')
    expect(getEntityStateLabel('running')).toBe('Running')
    expect(getExecutionStateLabel('completed')).toBe('Completed')
  })
})

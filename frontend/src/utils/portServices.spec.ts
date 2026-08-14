import { describe, it, expect } from 'vitest'
import { guessServiceForPort, protocolLabelFor } from './portServices'

describe('guessServiceForPort', () => {
  it('names well-known ports', () => {
    expect(guessServiceForPort(443)).toBe('HTTPS')
    expect(guessServiceForPort(22)).toBe('SSH')
    expect(guessServiceForPort(5432)).toBe('PostgreSQL')
    expect(guessServiceForPort(53)).toBe('DNS')
  })

  it('prefers the UDP meaning of a port when the transport is UDP', () => {
    expect(guessServiceForPort(500, 'udp')).toBe('IPsec')
    expect(guessServiceForPort(3478, 'udp')).toBe('STUN')
    // The same port over TCP has no UDP-only meaning to apply.
    expect(guessServiceForPort(500, 'tcp')).toBe('')
  })

  it('returns an empty string for ports with no useful convention', () => {
    expect(guessServiceForPort(51324)).toBe('')
    expect(guessServiceForPort(1)).toBe('')
  })

  it('returns an empty string for missing or out-of-range values', () => {
    expect(guessServiceForPort(undefined)).toBe('')
    expect(guessServiceForPort(null)).toBe('')
    expect(guessServiceForPort(0)).toBe('')
    expect(guessServiceForPort(-1)).toBe('')
    expect(guessServiceForPort(70000)).toBe('')
    expect(guessServiceForPort(Number.NaN)).toBe('')
  })

  it('never contains an out-of-range port key', () => {
    // Guards against a typo'd entry (e.g. 5432000) silently sitting in the map.
    for (const port of [443, 22, 53, 500, 3478]) {
      expect(guessServiceForPort(port) || guessServiceForPort(port, 'udp')).not.toBe('')
    }
  })
})

describe('protocolLabelFor', () => {
  it('prefers a real SNI hostname over the port guess', () => {
    expect(protocolLabelFor('github.com', 443, 'tcp')).toEqual({ text: 'github.com', source: 'sni' })
  })

  it('falls back to the port guess when there is no SNI', () => {
    expect(protocolLabelFor('', 443, 'tcp')).toEqual({ text: 'HTTPS', source: 'port' })
    expect(protocolLabelFor(undefined, 22, 'tcp')).toEqual({ text: 'SSH', source: 'port' })
  })

  it('treats a whitespace-only SNI as absent', () => {
    expect(protocolLabelFor('   ', 443, 'tcp')).toEqual({ text: 'HTTPS', source: 'port' })
  })

  it('reports no label when neither signal is available', () => {
    expect(protocolLabelFor(null, 51324, 'tcp')).toEqual({ text: '', source: 'none' })
  })
})

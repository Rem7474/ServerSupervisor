import { describe, it, expect } from 'vitest'
import { highlightParts } from './highlightMatch'

describe('highlightParts', () => {
  it('returns a single unmatched part for an empty query', () => {
    expect(highlightParts('web-01', '')).toEqual([{ text: 'web-01', matched: false }])
  })

  it('returns an empty array for empty text', () => {
    expect(highlightParts('', 'web')).toEqual([])
  })

  it('is case-insensitive and preserves original casing in the output', () => {
    expect(highlightParts('Web-01', 'WEB')).toEqual([
      { text: 'Web', matched: true },
      { text: '-01', matched: false },
    ])
  })

  it('splits around a match in the middle', () => {
    expect(highlightParts('proxmox-host-01', 'host')).toEqual([
      { text: 'proxmox-', matched: false },
      { text: 'host', matched: true },
      { text: '-01', matched: false },
    ])
  })

  it('highlights every occurrence', () => {
    expect(highlightParts('web-web-01', 'web')).toEqual([
      { text: 'web', matched: true },
      { text: '-', matched: false },
      { text: 'web', matched: true },
      { text: '-01', matched: false },
    ])
  })

  it('returns a single unmatched part when there is no match', () => {
    expect(highlightParts('db-01', 'web')).toEqual([{ text: 'db-01', matched: false }])
  })
})

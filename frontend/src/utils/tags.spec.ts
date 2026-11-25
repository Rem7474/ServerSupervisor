import { describe, it, expect } from 'vitest'
import { collectHostTags, formatTagsInput, hostHasTag, hostMatchesGroup, parseTagsInput } from './tags'

const web = { tags: ['prod', 'Web'], os: 'Debian GNU/Linux 12' }
const db = { tags: ['prod', 'db'], os: 'Ubuntu 24.04' }
const bare = { tags: null, os: null }

describe('tags utils', () => {
  it('parses and formats the comma-separated input', () => {
    expect(parseTagsInput(' a, ,b ')).toEqual(['a', 'b'])
    expect(formatTagsInput(['a', 'b'])).toBe('a, b')
    expect(formatTagsInput(null)).toBe('')
  })

  it('compares tags case-insensitively', () => {
    expect(hostHasTag(web, ' web ')).toBe(true)
    expect(hostHasTag(bare, 'web')).toBe(false)
  })

  it('matches every tag in "all" mode and one tag in "any" mode', () => {
    expect(hostMatchesGroup(web, { match_mode: 'all', tags: ['prod', 'web'], os_contains: '' })).toBe(true)
    expect(hostMatchesGroup(db, { match_mode: 'all', tags: ['prod', 'web'], os_contains: '' })).toBe(false)
    expect(hostMatchesGroup(db, { match_mode: 'any', tags: ['web', 'db'], os_contains: '' })).toBe(true)
    expect(hostMatchesGroup(bare, { match_mode: 'any', tags: ['web'], os_contains: '' })).toBe(false)
  })

  it('applies the OS filter alone or on top of the tags', () => {
    expect(hostMatchesGroup(web, { match_mode: 'all', tags: [], os_contains: 'debian' })).toBe(true)
    expect(hostMatchesGroup(db, { match_mode: 'all', tags: ['prod'], os_contains: 'Debian' })).toBe(false)
    expect(hostMatchesGroup(bare, { match_mode: 'all', tags: [], os_contains: 'debian' })).toBe(false)
  })

  it('matches nothing without any criterion', () => {
    expect(hostMatchesGroup(web, { match_mode: 'all', tags: [' '], os_contains: ' ' })).toBe(false)
  })

  it('collects a sorted, de-duplicated tag union', () => {
    expect(collectHostTags([web, db, bare, { tags: ['WEB', ' '] }])).toEqual(['db', 'prod', 'Web'])
  })
})

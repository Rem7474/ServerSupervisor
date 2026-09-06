import { describe, it, expect } from 'vitest'
import { fr, loadLocaleMessages } from './index'

type Messages = Record<string, unknown>

/** Flattens a nested message tree into `namespace.path.to.key` → value. */
function flatten(node: Messages, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(node)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object') {
      Object.assign(out, flatten(value as Messages, path))
    } else {
      out[path] = String(value)
    }
  }
  return out
}

// The fallback is bundled eagerly; every other locale is a lazy chunk, so it
// has to be pulled in explicitly here.
const en = (await loadLocaleMessages('en')) as Messages

const FR = flatten(fr as Messages)
const EN = flatten(en)

/**
 * Distinct `{name}` interpolation placeholders used by a message. Deduplicated
 * because a pluralized message repeats the same placeholder in each form.
 */
function placeholders(message: string): string[] {
  return [...new Set([...message.matchAll(/\{\s*([a-zA-Z0-9_]+)\s*\}/g)].map((m) => m[1]))].sort()
}

describe('locales', () => {
  it('defines every key in both languages', () => {
    // Guards the failure mode the rollout is most exposed to: a key added to
    // one language only, which renders the raw key path (or silently falls
    // back to French) for users of the other.
    expect(Object.keys(FR).filter((k) => !(k in EN))).toEqual([])
    expect(Object.keys(EN).filter((k) => !(k in FR))).toEqual([])
  })

  it('has a non-empty message for every key', () => {
    expect(Object.entries(FR).filter(([, v]) => !v.trim()).map(([k]) => k)).toEqual([])
    expect(Object.entries(EN).filter(([, v]) => !v.trim()).map(([k]) => k)).toEqual([])
  })

  it('uses the same interpolation variables in both languages', () => {
    // A translator renaming {count} to {n} on one side produces a literal
    // "{n}" on screen rather than a number.
    const mismatched = Object.keys(FR)
      .filter((k) => k in EN)
      .filter((k) => placeholders(FR[k]).join() !== placeholders(EN[k]).join())
    expect(mismatched).toEqual([])
  })

  it('keeps plural forms consistent for messages that take a count', () => {
    // A pluralized French message whose English counterpart has a single form
    // (or vice versa) silently drops one of the two branches.
    const mismatched = Object.keys(FR)
      .filter((k) => k in EN)
      .filter((k) => {
        const frForms = FR[k].split('|').length
        const enForms = EN[k].split('|').length
        // English legitimately needs one form where French inflects an
        // adjective, so only flag a *French* message that lost its plural.
        return frForms > 1 && enForms > frForms
      })
    expect(mismatched).toEqual([])
  })

  it('exposes the same namespaces in both languages', () => {
    expect(Object.keys(fr as Messages).sort()).toEqual(Object.keys(en).sort())
  })
})

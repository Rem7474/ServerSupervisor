import { describe, it, expect } from 'vitest'
import { visibleNavSections } from './navigation'

const passthroughT = (key: string) => key

function authFor(role: 'admin' | 'operator' | 'viewer') {
  return {
    isAdmin: role === 'admin',
    hasPermission: (perm: string) => {
      if (role === 'admin') return true
      if (role === 'operator') return perm === 'view:audit:commands'
      return false
    },
  }
}

describe('visibleNavSections', () => {
  it('disambiguates control.commands and settings.audit URLs for admins', () => {
    const sections = visibleNavSections(authFor('admin'), passthroughT)
    const control = sections.find((s) => s.key === 'control')
    const settings = sections.find((s) => s.key === 'settings')

    expect(control?.items.map((i) => i.to)).toContain('/audit?tab=commandes')
    expect(settings?.items.map((i) => i.to)).toContain('/audit?tab=journal')
    expect(settings?.items.map((i) => i.to)).toContain('/settings')
  })

  it('shows only /audit?tab=commandes to operators and hides admin settings sections', () => {
    const sections = visibleNavSections(authFor('operator'), passthroughT)
    const allDestinations = sections.flatMap((s) => s.items.map((i) => i.to))

    expect(allDestinations).toContain('/audit?tab=commandes')
    expect(allDestinations).not.toContain('/audit?tab=journal')
    expect(allDestinations).not.toContain('/settings')
    expect(sections.find((s) => s.key === 'settings')).toBeUndefined()
  })

  it('hides both audit destinations and /settings from viewers', () => {
    const sections = visibleNavSections(authFor('viewer'), passthroughT)
    const allDestinations = sections.flatMap((s) => s.items.map((i) => i.to))

    expect(allDestinations).not.toContain('/audit?tab=commandes')
    expect(allDestinations).not.toContain('/audit?tab=journal')
    expect(allDestinations).not.toContain('/settings')
  })
})

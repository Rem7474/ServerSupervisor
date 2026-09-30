import { describe, it, expect } from 'vitest'
import { gitProviderBadgeClass, notificationChannelBadgeClass } from './categoryBadges'

describe('categoryBadges', () => {
  it('gives every notification channel its own color', () => {
    const classes = ['smtp', 'ntfy', 'browser', 'notify'].map(notificationChannelBadgeClass)
    expect(new Set(classes).size).toBe(4)
    expect(classes).not.toContain('bg-secondary-lt text-secondary')
  })

  it('never uses a state color for a channel', () => {
    for (const ch of ['smtp', 'ntfy', 'browser', 'notify']) {
      expect(notificationChannelBadgeClass(ch)).not.toMatch(/success|danger|warning|green|red|yellow/)
    }
  })

  it('falls back to neutral for unknown or missing values', () => {
    expect(notificationChannelBadgeClass('slack')).toBe('bg-secondary-lt text-secondary')
    expect(gitProviderBadgeClass(undefined)).toBe('bg-secondary-lt text-secondary')
    expect(gitProviderBadgeClass('bitbucket')).toBe('bg-secondary-lt text-secondary')
  })

  it('maps known git providers', () => {
    expect(gitProviderBadgeClass('github')).toBe('bg-blue-lt text-blue')
    expect(gitProviderBadgeClass('gitlab')).toBe('bg-orange-lt text-orange')
  })
})

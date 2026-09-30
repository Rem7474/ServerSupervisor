/**
 * Categorical (not state) badge colors — the palette names frontend/CLAUDE.md
 * reserves for neutral tagging. One map per category so a given git provider
 * or notification channel has the same color on every page.
 */

const NEUTRAL = 'bg-secondary-lt text-secondary'

const GIT_PROVIDER_BADGES: Record<string, string> = {
  github: 'bg-blue-lt text-blue',
  gitlab: 'bg-orange-lt text-orange',
  gitea: 'bg-teal-lt text-teal',
  forgejo: 'bg-purple-lt text-purple',
  custom: NEUTRAL,
}

export function gitProviderBadgeClass(provider: string | null | undefined): string {
  return (provider && GIT_PROVIDER_BADGES[provider]) || NEUTRAL
}

const NOTIFICATION_CHANNEL_BADGES: Record<string, string> = {
  smtp: 'bg-blue-lt text-blue',
  ntfy: 'bg-orange-lt text-orange',
  browser: 'bg-purple-lt text-purple',
  notify: 'bg-cyan-lt text-cyan',
}

export function notificationChannelBadgeClass(channel: string | null | undefined): string {
  return (channel && NOTIFICATION_CHANNEL_BADGES[channel]) || NEUTRAL
}

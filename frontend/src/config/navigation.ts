import type { Component } from 'vue'
import {
  IconApps, IconLayoutDashboard, IconBell, IconTerminal2,
  IconServer, IconBrandDocker, IconRefresh, IconActivity,
  IconRobot, IconClock, IconPlayerPlay, IconGitBranch,
  IconShieldLock, IconShieldCheck, IconChartLine,
  IconStack2, IconServer2, IconTopologyStar3, IconBox,
  IconAdjustments, IconSettings, IconUsers, IconClipboardList,
} from '@tabler/icons-vue'

export interface NavItem {
  label: string
  to: string
  icon: Component
  adminOnly?: boolean
  // Permission string checked via auth.hasPermission (see stores/auth.ts's
  // ROLE_PERMISSIONS) — for items visible to operator+admin but not viewer,
  // where adminOnly's admin-vs-everyone-else split is too coarse.
  requiresPermission?: string
}

export interface NavSection {
  key: string
  label: string
  icon: Component
  items: NavItem[]
}

interface NavItemDef {
  key: string
  to: string
  icon: Component
  adminOnly?: boolean
  requiresPermission?: string
}

interface NavSectionDef {
  key: string
  icon: Component
  items: NavItemDef[]
}

// Grouped by intent (what the user is trying to do) rather than by role or
// alphabetically — replaces the previous flat navbar + "Plus"/"Admin"
// dropdown split. Every route below already existed; this only changes how
// they're grouped for navigation. adminOnly mirrors each item's exact
// pre-existing visibility (either the route's own requiresAdmin, or the
// v-if the old navbar had on it) — not a new permission decision.
//
// Labels are i18n keys resolved by visibleNavSections (locales/{fr,en}/nav.json,
// under `sections.<section.key>.label` / `.items.<item.key>`), not literal text —
// this list is otherwise the same shape it was before i18n.
const navigationSections: NavSectionDef[] = [
  {
    key: 'control',
    icon: IconApps,
    items: [
      { key: 'dashboard', to: '/', icon: IconLayoutDashboard },
      { key: 'alerts', to: '/alerts', icon: IconBell },
      { key: 'commands', to: '/audit', icon: IconTerminal2, requiresPermission: 'view:audit:commands' },
    ],
  },
  {
    key: 'hosts',
    icon: IconServer,
    items: [
      { key: 'docker', to: '/docker', icon: IconBrandDocker },
      { key: 'updates', to: '/apt', icon: IconRefresh },
      { key: 'monitoring', to: '/monitoring', icon: IconActivity },
    ],
  },
  {
    key: 'automation',
    icon: IconRobot,
    items: [
      { key: 'scheduledTasks', to: '/scheduled-tasks', icon: IconClock },
      { key: 'runbooks', to: '/runbooks', icon: IconPlayerPlay, adminOnly: true },
      { key: 'gitAutomation', to: '/git-webhooks', icon: IconGitBranch, adminOnly: true },
    ],
  },
  {
    key: 'security',
    icon: IconShieldLock,
    items: [
      { key: 'webThreats', to: '/threats', icon: IconShieldCheck, adminOnly: true },
      { key: 'webStats', to: '/traffic', icon: IconChartLine, adminOnly: true },
    ],
  },
  {
    key: 'infrastructure',
    icon: IconStack2,
    items: [
      { key: 'proxmox', to: '/proxmox', icon: IconServer2 },
      { key: 'network', to: '/network', icon: IconTopologyStar3 },
      { key: 'npm', to: '/npm', icon: IconBox },
    ],
  },
  {
    key: 'settings',
    icon: IconAdjustments,
    items: [
      { key: 'settings', to: '/settings', icon: IconSettings, adminOnly: true },
      { key: 'users', to: '/users', icon: IconUsers, adminOnly: true },
      { key: 'audit', to: '/audit', icon: IconClipboardList, adminOnly: true },
    ],
  },
]

// Shared by the navbar and the command palette so the two never disagree
// about which destinations a given role can see. Returns only sections that
// still have at least one visible item (Security/Settings are 100%
// adminOnly today, so a viewer/operator gets 4 sections, not 6 with two
// dead-ends). `t` is vue-i18n's translate function — passed in rather than
// imported so this stays a plain function callers can use inside their own
// reactive computed (its locale-reactivity comes from that computed reading
// t(), not from anything here).
export function visibleNavSections(
  auth: { isAdmin: boolean; hasPermission: (permission: string) => boolean },
  t: (key: string) => string
): NavSection[] {
  return navigationSections
    .map((section) => ({
      key: section.key,
      label: t(`nav.sections.${section.key}.label`),
      icon: section.icon,
      items: section.items
        .filter((item) =>
          (!item.adminOnly || auth.isAdmin) &&
          (!item.requiresPermission || auth.hasPermission(item.requiresPermission))
        )
        .map((item) => ({
          label: t(`nav.sections.${section.key}.items.${item.key}`),
          to: item.to,
          icon: item.icon,
          adminOnly: item.adminOnly,
          requiresPermission: item.requiresPermission,
        })),
    }))
    .filter((section) => section.items.length > 0)
}

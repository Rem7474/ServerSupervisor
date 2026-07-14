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
}

export interface NavSection {
  key: string
  label: string
  icon: Component
  items: NavItem[]
}

// Grouped by intent (what the user is trying to do) rather than by role or
// alphabetically — replaces the previous flat navbar + "Plus"/"Admin"
// dropdown split. Every route below already existed; this only changes how
// they're grouped for navigation. adminOnly mirrors each item's exact
// pre-existing visibility (either the route's own requiresAdmin, or the
// v-if the old navbar had on it) — not a new permission decision.
export const navigationSections: NavSection[] = [
  {
    key: 'control',
    label: 'Centre de contrôle',
    icon: IconApps,
    items: [
      { label: 'Dashboard', to: '/', icon: IconLayoutDashboard },
      { label: 'Alertes', to: '/alerts', icon: IconBell },
      { label: 'Commandes en cours', to: '/commands', icon: IconTerminal2 },
    ],
  },
  {
    key: 'hosts',
    label: 'Hôtes',
    icon: IconServer,
    items: [
      { label: 'Docker', to: '/docker', icon: IconBrandDocker },
      { label: 'Mises à jour', to: '/apt', icon: IconRefresh },
      { label: 'Monitoring', to: '/monitoring', icon: IconActivity },
    ],
  },
  {
    key: 'automation',
    label: 'Automatisation',
    icon: IconRobot,
    items: [
      { label: 'Tâches planifiées', to: '/scheduled-tasks', icon: IconClock },
      { label: 'Runbooks', to: '/runbooks', icon: IconPlayerPlay, adminOnly: true },
      { label: 'Git / Automatisation', to: '/git-webhooks', icon: IconGitBranch, adminOnly: true },
    ],
  },
  {
    key: 'security',
    label: 'Sécurité',
    icon: IconShieldLock,
    items: [
      { label: 'Menaces web', to: '/threats', icon: IconShieldCheck, adminOnly: true },
      { label: 'Stats web', to: '/traffic', icon: IconChartLine, adminOnly: true },
    ],
  },
  {
    key: 'infrastructure',
    label: 'Infrastructure',
    icon: IconStack2,
    items: [
      { label: 'Proxmox', to: '/proxmox', icon: IconServer2 },
      { label: 'Réseau', to: '/network', icon: IconTopologyStar3 },
      { label: 'NPM', to: '/npm', icon: IconBox },
    ],
  },
  {
    key: 'settings',
    label: 'Réglages',
    icon: IconAdjustments,
    items: [
      { label: 'Paramètres', to: '/settings', icon: IconSettings, adminOnly: true },
      { label: 'Utilisateurs', to: '/users', icon: IconUsers, adminOnly: true },
      { label: 'Audit', to: '/audit', icon: IconClipboardList, adminOnly: true },
    ],
  },
]

// Shared by the navbar and the command palette so the two never disagree
// about which destinations a given role can see. Returns only sections that
// still have at least one visible item (Sécurité/Réglages are 100%
// adminOnly today, so a viewer/operator gets 4 sections, not 6 with two
// dead-ends).
export function visibleNavSections(isAdmin: boolean): NavSection[] {
  return navigationSections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => !item.adminOnly || isAdmin),
    }))
    .filter((section) => section.items.length > 0)
}

import { computed, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EntityTab } from '../components/EntityTabShell.vue'
import type { HostRecord } from '../types/hostDetail'

interface Options {
  isAdmin: () => boolean
  canRunApt: Ref<boolean>
  aptStatus: Ref<HostRecord | null>
  containers: Ref<HostRecord[]>
  hostActiveIncidents: Ref<HostRecord[]>
  exposureDomainCount: Ref<number>
  customTasksCount: Ref<number>
  tasksCount: Ref<number>
  hasLxcConsole: Ref<boolean>
}

export function useHostTabs(o: Options) {
  const { t } = useI18n()
  // `lazy` is set on every tab that fetches its own data from a `host-id` prop
  // and feeds nothing back to the view — mounting those eagerly meant one page
  // load issued their requests too, whether or not the user ever opened them
  // (Timeline alone is a ~1.4 MB response). The eager ones are eager for a
  // reason: `overview` is the default tab, `metrics`/`docker`/`apt` render data
  // the view already holds from the WS snapshot (no request to save), and
  // `exposition`/`taches-personnalisees`/`planifiees` each emit a count the
  // view renders as a badge or a KPI, so deferring them would blank that count
  // until first visit.
  return computed<EntityTab[]>(() => {
    const securityUpdates = o.aptStatus.value?.security_updates || 0
    const pendingPackages = o.aptStatus.value?.pending_packages || 0

    const tabs: EntityTab[] = [
      {
        key: 'overview',
        label: t('host.overviewTabLabel'),
        badges: o.hostActiveIncidents.value.length
          ? [{ value: o.hostActiveIncidents.value.length, badgeClass: 'badge bg-danger-lt text-danger ms-1' }]
          : [],
      },
      { key: 'metrics', label: t('host.metricsTabLabel') },
      {
        key: 'docker',
        label: 'Docker',
        badges: o.containers.value.length ? [{ value: o.containers.value.length, badgeClass: 'badge bg-blue-lt text-blue ms-1' }] : [],
      },
      {
        key: 'apt',
        label: 'APT',
        badges: securityUpdates > 0
          ? [{ value: securityUpdates, badgeClass: 'badge bg-danger-lt text-danger ms-1' }]
          : pendingPackages > 0
            ? [{ value: pendingPackages, badgeClass: 'badge bg-warning-lt text-warning ms-1' }]
            : [],
      },
      { key: 'backup', label: t('host.backupTabLabel'), lazy: true },
      { key: 'reseau-flux', label: t('host.networkTrafficTabLabel'), lazy: true },
      {
        key: 'exposition',
        label: t('host.exposureLabel'),
        badges: o.exposureDomainCount.value > 0
          ? [{ value: o.exposureDomainCount.value, badgeClass: 'badge bg-azure-lt text-azure ms-1' }]
          : [],
      },
    ]

    if (o.canRunApt.value) {
      tabs.push(
        { key: 'systeme', label: t('host.systemTabLabel'), lazy: true },
        { key: 'processus', label: t('host.processesTabLabel'), lazy: true }
      )
    }

    // Labeled "Permissions" (not "Security") to avoid colliding with
    // ProxmoxNodeView's "Security logs" tab — same word, unrelated content
    // (per-host RBAC here vs. PVE syslog auth-failure search there).
    if (o.isAdmin()) {
      tabs.push({ key: 'securite', label: t('host.permissionsTabLabel') })
    }
    tabs.push({
      key: 'taches-personnalisees',
      label: t('host.customTasksTabLabel'),
      badges: o.customTasksCount.value ? [{ value: o.customTasksCount.value, badgeClass: 'badge bg-secondary-lt text-secondary ms-1' }] : [],
    })
    tabs.push({
      key: 'planifiees',
      label: t('host.scheduledTasksTabLabel'),
      badges: o.tasksCount.value ? [{ value: o.tasksCount.value, badgeClass: 'badge bg-secondary-lt text-secondary ms-1' }] : [],
    })
    tabs.push({ key: 'timeline', label: 'Timeline', lazy: true })
    if (o.hasLxcConsole.value) {
      tabs.push({ key: 'console', label: t('host.consoleTabLabel') })
    }

    return tabs
  })
}

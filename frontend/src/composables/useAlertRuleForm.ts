import { ref, Ref } from 'vue'
import { getAlertMetricMeta } from '../utils/alertMetrics'

function isProxmoxMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'proxmox'
}

function isSyntheticMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'synthetic'
}

function isDockerMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'docker'
}

function isProxmoxGuestMetric(metric: string): boolean {
  return metric === 'proxmox_guest_cpu_percent' || metric === 'proxmox_guest_memory_percent'
}

function isProxmoxDiskMetric(metric: string): boolean {
  return metric === 'proxmox_disk_failed_count' || metric === 'proxmox_disk_min_wearout_percent'
}

function isProxmoxCountMetric(metric: string): boolean {
  const meta = getAlertMetricMeta(metric)
  return meta.category === 'proxmox' && meta.unit === '' && metric !== 'proxmox_auth_failures_recent'
}

interface CommandTrigger {
  module: string
  action: string
  target: string
}

interface AlertRuleFormActions {
  channels: string[]
  smtp_to: string
  ntfy_topic: string
  cooldown: number
  command_trigger?: CommandTrigger
}

interface ProxmoxScope {
  scope_mode: string
  connection_id: string
  node_id: string
  storage_id: string
  guest_id: string
  disk_id: string
}

export interface DockerScope {
  scope_mode: string   // 'host' | 'container' | 'compose_project'
  host_id: string
  container_id: string
  project_name: string
}

export interface AlertRuleFormData {
  name: string
  enabled: boolean
  source_type: 'agent' | 'proxmox' | 'synthetic' | 'docker'
  host_id: string | null
  proxmox_scope: ProxmoxScope
  docker_scope: DockerScope
  metric: string
  operator: string
  threshold_warn: number
  threshold_crit: number
  threshold_clear_warn?: number
  threshold_clear_crit?: number
  duration: number
  actions: AlertRuleFormActions
}

interface AlertRuleInput {
  name?: string
  enabled?: boolean
  source_type?: 'agent' | 'proxmox' | 'synthetic' | 'docker'
  host_id?: string | null
  metric?: string
  proxmox_scope?: Partial<ProxmoxScope>
  docker_scope?: Partial<DockerScope>
  operator?: string
  threshold_warn?: number
  threshold_crit?: number
  threshold_clear_warn?: number
  threshold_clear_crit?: number
  duration_seconds?: number
  actions?: {
    channels?: string[]
    smtp_to?: string
    ntfy_topic?: string
    cooldown?: number
    command_trigger?: Partial<CommandTrigger>
  }
}

interface AlertRulePayload extends Omit<AlertRuleFormData, 'proxmox_scope' | 'docker_scope' | 'threshold_clear_warn' | 'threshold_clear_crit'> {
  proxmox_scope: ProxmoxScope | null
  docker_scope: DockerScope | null
  threshold_clear_warn: number | null
  threshold_clear_crit: number | null
}

interface AlertRuleFormApi {
  form: Ref<AlertRuleFormData>
  channelSmtp: Ref<boolean>
  channelNtfy: Ref<boolean>
  channelBrowser: Ref<boolean>
  commandTriggerEnabled: Ref<boolean>
  defaultCommandTrigger: () => CommandTrigger
  defaultForm: () => AlertRuleFormData
  hydrateFormFromRule: (rule: AlertRuleInput | null) => void
  onMetricChange: () => void
  buildPayload: () => AlertRulePayload
}

function normalizeOptionalNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const n = Number(value)
  return Number.isFinite(n) ? n : undefined
}

export function useAlertRuleForm(): AlertRuleFormApi {
  const channelSmtp: Ref<boolean> = ref(false)
  const channelNtfy: Ref<boolean> = ref(false)
  const channelBrowser: Ref<boolean> = ref(false)
  const commandTriggerEnabled: Ref<boolean> = ref(false)

  const defaultCommandTrigger = (): CommandTrigger => ({ module: 'processes', action: 'list', target: '' })
  const defaultForm = (): AlertRuleFormData => ({
    name: '',
    enabled: true,
    source_type: 'agent',
    host_id: null,
    metric: 'cpu',
    proxmox_scope: {
      scope_mode: 'global',
      connection_id: '',
      node_id: '',
      storage_id: '',
      guest_id: '',
      disk_id: '',
    },
    docker_scope: {
      scope_mode: 'host',
      host_id: '',
      container_id: '',
      project_name: '',
    },
    operator: '>',
    threshold_warn: 70,
    threshold_crit: 85,
    threshold_clear_warn: undefined,
    threshold_clear_crit: undefined,
    duration: 300,
    actions: {
      channels: [],
      smtp_to: '',
      ntfy_topic: '',
      cooldown: 3600,
      command_trigger: defaultCommandTrigger(),
    },
  })

  const form: Ref<AlertRuleFormData> = ref(defaultForm())

  function hydrateFormFromRule(rule: AlertRuleInput | null): void {
    if (!rule) {
      form.value = defaultForm()
      channelSmtp.value = false
      channelNtfy.value = false
      channelBrowser.value = false
      commandTriggerEnabled.value = false
      return
    }

    const actions = rule.actions || {}
    const scope = rule.proxmox_scope || {}
    const dscope = rule.docker_scope || {}
    const commandTrigger = actions.command_trigger
    const metric = rule.metric ?? 'cpu'

    form.value = {
      name: rule.name || '',
      enabled: rule.enabled ?? true,
      source_type:
        rule.source_type ||
        (isDockerMetric(metric) ? 'docker' : isProxmoxMetric(metric) ? 'proxmox' : isSyntheticMetric(metric) ? 'synthetic' : 'agent'),
      host_id: rule.host_id ?? null,
      metric,
      proxmox_scope: {
        scope_mode: scope.scope_mode || 'global',
        connection_id: scope.connection_id || '',
        node_id: scope.node_id || '',
        storage_id: scope.storage_id || '',
        guest_id: scope.guest_id || '',
        disk_id: scope.disk_id || '',
      },
      docker_scope: {
        scope_mode: dscope.scope_mode || 'host',
        host_id: dscope.host_id || '',
        container_id: dscope.container_id || '',
        project_name: dscope.project_name || '',
      },
      operator: rule.operator ?? '>',
      threshold_warn: rule.threshold_warn ?? 70,
      threshold_crit: rule.threshold_crit ?? 85,
      threshold_clear_warn: rule.threshold_clear_warn,
      threshold_clear_crit: rule.threshold_clear_crit,
      duration: rule.duration_seconds ?? 300,
      actions: {
        channels: actions.channels || [],
        smtp_to: actions.smtp_to || '',
        ntfy_topic: actions.ntfy_topic || '',
        cooldown: actions.cooldown ?? 3600,
        command_trigger: commandTrigger
          ? {
              module: commandTrigger.module ?? 'processes',
              action: commandTrigger.action ?? 'list',
              target: commandTrigger.target || '',
            }
          : defaultCommandTrigger(),
      },
    }

    channelSmtp.value = actions.channels?.includes('smtp') || false
    channelNtfy.value = actions.channels?.includes('ntfy') || false
    channelBrowser.value = actions.channels?.includes('browser') || false
    commandTriggerEnabled.value = !!commandTrigger
  }

  function onMetricChange(): void {
    if (form.value.metric === 'heartbeat_timeout') {
      form.value.source_type = 'agent'
      form.value.operator = '>'
      if (!form.value.threshold_crit || form.value.threshold_crit === 85) {
        form.value.threshold_warn = 240
        form.value.threshold_crit = 300
      }
      form.value.duration = 0
      return
    }

    if (isDockerMetric(form.value.metric)) {
      form.value.source_type = 'docker'
      form.value.host_id = null
      if (form.value.metric === 'docker_container_running_count') {
        form.value.docker_scope.scope_mode = 'host'
        form.value.docker_scope.container_id = ''
      }
    } else if (isProxmoxMetric(form.value.metric)) {
      form.value.source_type = 'proxmox'
      form.value.host_id = null

      if (isProxmoxGuestMetric(form.value.metric)) {
        form.value.proxmox_scope.scope_mode = 'guest'
      } else if (isProxmoxDiskMetric(form.value.metric)) {
        form.value.proxmox_scope.scope_mode = 'disk'
      } else if (form.value.proxmox_scope.scope_mode === 'guest' || form.value.proxmox_scope.scope_mode === 'disk') {
        form.value.proxmox_scope.scope_mode = 'global'
        form.value.proxmox_scope.guest_id = ''
        form.value.proxmox_scope.disk_id = ''
      }
    } else if (isSyntheticMetric(form.value.metric)) {
      form.value.source_type = 'synthetic'
      form.value.host_id = null
    } else {
      form.value.source_type = 'agent'
    }

    if (form.value.metric === 'proxmox_disk_min_wearout_percent') {
      form.value.operator = '<'
      if (!form.value.threshold_crit || form.value.threshold_crit === 85) {
        form.value.threshold_warn = 25
        form.value.threshold_crit = 20
      }
      return
    }

    if (isProxmoxCountMetric(form.value.metric)) {
      form.value.operator = '>'
      if (!form.value.threshold_crit || form.value.threshold_crit === 85) {
        form.value.threshold_warn = 0.3
        form.value.threshold_crit = 0.5
      }
      form.value.duration = 0
      return
    }

    if (form.value.metric === 'status_offline' || form.value.metric === 'disk_smart_status') {
      form.value.operator = '>'
      form.value.threshold_warn = 0.5
      form.value.threshold_crit = 0.5
      form.value.duration = 0
    }
  }

  function buildPayload(): AlertRulePayload {
    const channels: string[] = []
    if (channelSmtp.value) channels.push('smtp')
    if (channelNtfy.value) channels.push('ntfy')
    if (channelBrowser.value) channels.push('browser')

    const actions: AlertRuleFormActions = {
      ...form.value.actions,
      channels,
    }

    if (!commandTriggerEnabled.value) {
      delete actions.command_trigger
    }

    const thresholdClearWarn = normalizeOptionalNumber(form.value.threshold_clear_warn) ?? null
    const thresholdClearCrit = normalizeOptionalNumber(form.value.threshold_clear_crit) ?? null

    return {
      ...form.value,
      threshold_clear_warn: thresholdClearWarn,
      threshold_clear_crit: thresholdClearCrit,
      source_type: form.value.source_type,
      proxmox_scope: isProxmoxMetric(form.value.metric) ? { ...form.value.proxmox_scope } : null,
      docker_scope: isDockerMetric(form.value.metric) ? { ...form.value.docker_scope } : null,
      actions,
    }
  }

  return {
    form,
    channelSmtp,
    channelNtfy,
    channelBrowser,
    commandTriggerEnabled,
    defaultCommandTrigger,
    defaultForm,
    hydrateFormFromRule,
    onMetricChange,
    buildPayload,
  }
}


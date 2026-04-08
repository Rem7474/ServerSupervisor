import { ref, Ref } from 'vue'
import { getAlertMetricMeta } from '../utils/alertMetrics'

function isProxmoxMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'proxmox'
}

function isProxmoxCountMetric(metric: string): boolean {
  const meta = getAlertMetricMeta(metric)
  return meta.category === 'proxmox' && meta.unit === ''
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
}

interface AlertRuleFormData {
  name: string
  enabled: boolean
  source_type: 'agent' | 'proxmox'
  host_id: string | null
  proxmox_scope: ProxmoxScope
  metric: string
  operator: string
  threshold: number
  duration: number
  actions: AlertRuleFormActions
}

interface AlertRuleFormApi {
  form: Ref<AlertRuleFormData>
  channelSmtp: Ref<boolean>
  channelNtfy: Ref<boolean>
  channelBrowser: Ref<boolean>
  commandTriggerEnabled: Ref<boolean>
  defaultCommandTrigger: () => CommandTrigger
  defaultForm: () => AlertRuleFormData
  hydrateFormFromRule: (rule: any) => void
  onMetricChange: () => void
  buildPayload: () => any
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
    },
    operator: '>',
    threshold: 80,
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

  function hydrateFormFromRule(rule: any): void {
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
    const commandTrigger = actions.command_trigger

    form.value = {
      name: rule.name || '',
      enabled: rule.enabled,
        source_type: rule.source_type || (isProxmoxMetric(rule.metric) ? 'proxmox' : 'agent'),
      host_id: rule.host_id,
      metric: rule.metric,
        proxmox_scope: {
          scope_mode: scope.scope_mode || 'global',
          connection_id: scope.connection_id || '',
          node_id: scope.node_id || '',
          storage_id: scope.storage_id || '',
        },
      operator: rule.operator,
      threshold: rule.threshold,
      duration: rule.duration_seconds,
      actions: {
        channels: actions.channels || [],
        smtp_to: actions.smtp_to || '',
        ntfy_topic: actions.ntfy_topic || '',
        cooldown: actions.cooldown || 3600,
        command_trigger: commandTrigger
          ? { module: commandTrigger.module, action: commandTrigger.action, target: commandTrigger.target || '' }
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
      if (!form.value.threshold || form.value.threshold === 80) {
        form.value.threshold = 300
      }
      form.value.duration = 0
      return
    }

    if (isProxmoxMetric(form.value.metric)) {
      form.value.source_type = 'proxmox'
      form.value.host_id = null
    } else {
      form.value.source_type = 'agent'
    }

    if (form.value.metric === 'proxmox_disk_min_wearout_percent') {
      form.value.operator = '<'
      if (!form.value.threshold || form.value.threshold === 80) {
        form.value.threshold = 20
      }
      return
    }

    if (isProxmoxCountMetric(form.value.metric)) {
      form.value.operator = '>'
      if (!form.value.threshold || form.value.threshold === 80) {
        form.value.threshold = 0.5
      }
      form.value.duration = 0
      return
    }

    if (form.value.metric === 'status_offline' || form.value.metric === 'disk_smart_status') {
      form.value.operator = '>'
      form.value.threshold = 0.5
      form.value.duration = 0
    }
  }

  function buildPayload(): any {
    const channels: string[] = []
    if (channelSmtp.value) channels.push('smtp')
    if (channelNtfy.value) channels.push('ntfy')
    if (channelBrowser.value) channels.push('browser')

    const actions: any = {
      ...form.value.actions,
      channels,
    }

    if (!commandTriggerEnabled.value) {
      delete actions.command_trigger
    }

    return {
      ...form.value,
      source_type: form.value.source_type,
      proxmox_scope: isProxmoxMetric(form.value.metric) ? { ...form.value.proxmox_scope } : null,
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


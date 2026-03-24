import { ref, Ref } from 'vue'

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

interface AlertRuleFormData {
  name: string
  enabled: boolean
  host_id: string | null
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
    host_id: null,
    metric: 'cpu',
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
    const commandTrigger = actions.command_trigger

    form.value = {
      name: rule.name || '',
      enabled: rule.enabled,
      host_id: rule.host_id,
      metric: rule.metric,
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
    if (form.value.metric !== 'heartbeat_timeout') return
    form.value.operator = '>'
    if (!form.value.threshold || form.value.threshold === 80) {
      form.value.threshold = 300
    }
    form.value.duration = 0
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

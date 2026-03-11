import { ref } from 'vue'

/**
 * @returns {{
 *  form: import('vue').Ref<any>,
 *  channelSmtp: import('vue').Ref<boolean>,
 *  channelNtfy: import('vue').Ref<boolean>,
 *  channelBrowser: import('vue').Ref<boolean>,
 *  commandTriggerEnabled: import('vue').Ref<boolean>,
 *  defaultCommandTrigger: () => { module: string, action: string, target: string },
 *  defaultForm: () => any,
 *  hydrateFormFromRule: (rule: any) => void,
 *  onMetricChange: () => void,
 *  buildPayload: () => any,
 * }}
 */
export function useAlertRuleForm() {
  const channelSmtp = ref(false)
  const channelNtfy = ref(false)
  const channelBrowser = ref(false)
  const commandTriggerEnabled = ref(false)

  const defaultCommandTrigger = () => ({ module: 'processes', action: 'list', target: '' })
  const defaultForm = () => ({
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

  const form = ref(defaultForm())

  /** @param {any} rule */
  function hydrateFormFromRule(rule) {
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

  function onMetricChange() {
    if (form.value.metric !== 'heartbeat_timeout') return
    form.value.operator = '>'
    if (!form.value.threshold || form.value.threshold === 80) {
      form.value.threshold = 300
    }
    form.value.duration = 0
  }

  function buildPayload() {
    const channels = []
    if (channelSmtp.value) channels.push('smtp')
    if (channelNtfy.value) channels.push('ntfy')
    if (channelBrowser.value) channels.push('browser')

    const actions = {
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

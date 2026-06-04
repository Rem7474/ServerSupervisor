import { describe, it, expect, vi } from 'vitest'
import { reactive } from 'vue'

// The composable loads custom tasks / registry credentials on open via the API
// barrel; stub it so the immediate watcher never hits the network.
vi.mock('../api', () => ({
  default: {
    getHostCustomTasks: vi.fn(async () => ({ data: [] })),
    getRegistryCredentials: vi.fn(async () => ({ data: { credentials: [] } })),
  },
}))

import { useWebhookForm, type WebhookFormProps, type WebhookFormData } from './useWebhookForm'

function makeProps(overrides: Partial<WebhookFormProps> = {}): WebhookFormProps {
  return reactive({
    visible: true,
    mode: 'webhook',
    item: null,
    error: '',
    prefillDockerImage: '',
    prefillDockerTag: '',
    prefillComposeProject: '',
    ...overrides,
  })
}

// setup builds a composable instance and captures the last submitted payload.
function setup(props: WebhookFormProps) {
  const submitted: { payload: WebhookFormData | null } = { payload: null }
  const api = useWebhookForm(props, (p) => { submitted.payload = p })
  return { ...api, submitted }
}

describe('useWebhookForm — defaults', () => {
  it('defaults to a webhook form in webhook mode', () => {
    const { form } = setup(makeProps({ mode: 'webhook' }))
    expect(form.value.provider).toBe('github')
    expect(form.value.event_filter).toBe('push')
    expect(form.value.notify_on_failure).toBe(true)
    expect(form.value.enabled).toBe(true)
    expect(form.value.name).toBe('')
  })

  it('defaults to a git tracker form in tracker mode', () => {
    const { form } = setup(makeProps({ mode: 'tracker' }))
    expect(form.value.tracker_type).toBe('git')
    expect(form.value.dispatch_task).toBe(true)
    expect(form.value.notify_on_release).toBe(true)
  })

  it('prefills a docker tracker (monitor-only) from the Docker page', () => {
    const { form } = setup(makeProps({ mode: 'tracker', prefillDockerImage: 'nginx', prefillDockerTag: '1.27' }))
    expect(form.value.tracker_type).toBe('docker')
    expect(form.value.docker_image).toBe('nginx')
    expect(form.value.docker_tag).toBe('1.27')
    expect(form.value.dispatch_task).toBe(false)
  })

  it('prefills a compose tracker with dispatch enabled', () => {
    const { form } = setup(makeProps({ mode: 'tracker', prefillComposeProject: 'my-app' }))
    expect(form.value.dispatch_task).toBe(true)
    expect(form.value.update_action).toBe('compose')
    expect(form.value.compose_project).toBe('my-app')
  })
})

describe('useWebhookForm — hydration from an edited item', () => {
  it('marks dispatch_task when the tracker item has a host + task', () => {
    const { form } = setup(makeProps({
      mode: 'tracker',
      item: { name: 'HA', tracker_type: 'docker', host_id: 'h1', custom_task_id: 't1', docker_image: 'img', enabled: false },
    }))
    expect(form.value.name).toBe('HA')
    expect(form.value.tracker_type).toBe('docker')
    expect(form.value.dispatch_task).toBe(true)
    expect(form.value.enabled).toBe(false)
  })

  it('treats a host-less tracker item as monitor-only', () => {
    const { form } = setup(makeProps({
      mode: 'tracker',
      item: { name: 'mon', tracker_type: 'git', repo_owner: 'a', repo_name: 'b' },
    }))
    expect(form.value.dispatch_task).toBe(false)
  })
})

describe('useWebhookForm — validation (submit guard)', () => {
  it('rejects an incomplete webhook (no host/task)', () => {
    const { form, submit, validationError, submitted } = setup(makeProps({ mode: 'webhook' }))
    form.value.name = 'x'
    submit()
    expect(validationError.value).not.toBe('')
    expect(submitted.payload).toBeNull()
  })

  it('accepts a complete webhook', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'webhook' }))
    form.value.name = 'deploy'
    form.value.host_id = 'h1'
    form.value.custom_task_id = 't1'
    submit()
    expect(submitted.payload).not.toBeNull()
  })

  it('rejects a git tracker missing repo_owner', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    form.value.name = 'x'
    form.value.repo_owner = ''
    form.value.repo_name = 'b'
    submit()
    expect(submitted.payload).toBeNull()
  })

  it('rejects an out-of-range cooldown', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    form.value.name = 'x'
    form.value.repo_owner = 'a'
    form.value.repo_name = 'b'
    form.value.cooldown_hours = 200
    submit()
    expect(submitted.payload).toBeNull()
  })
})

describe('useWebhookForm — submit payload normalisation', () => {
  it('clears dispatch targets for a monitor-only tracker and drops dispatch_task', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    Object.assign(form.value, {
      name: 'mon', tracker_type: 'git', repo_owner: 'a', repo_name: 'b',
      dispatch_task: false, host_id: 'h1', custom_task_id: 't1',
      compose_project: 'p', compose_service: 's',
    })
    submit()
    const p = submitted.payload!
    expect(p.host_id).toBe('')
    expect(p.custom_task_id).toBe('')
    expect(p.update_action).toBe('custom')
    expect(p.compose_project).toBe('')
    expect('dispatch_task' in p).toBe(false)
  })

  it('clears custom_task_id in docker compose mode', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    Object.assign(form.value, {
      name: 'app', tracker_type: 'docker', docker_image: 'nginx',
      dispatch_task: true, update_action: 'compose',
      host_id: 'h1', compose_project: 'proj', custom_task_id: 'leftover',
      notify_channels: ['ntfy'],
    })
    submit()
    expect(submitted.payload!.custom_task_id).toBe('')
  })

  it('clears compose fields in docker custom mode', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    Object.assign(form.value, {
      name: 'app', tracker_type: 'docker', docker_image: 'nginx',
      dispatch_task: true, update_action: 'custom',
      host_id: 'h1', custom_task_id: 't1',
      compose_project: 'proj', compose_service: 'svc',
      pre_update_task_id: 'pre', post_update_task_id: 'post',
      notify_channels: ['ntfy'],
    })
    submit()
    const p = submitted.payload!
    expect(p.compose_project).toBe('')
    expect(p.compose_service).toBe('')
    expect(p.pre_update_task_id).toBe('')
    expect(p.post_update_task_id).toBe('')
  })

  it('clears registry credentials and forces custom action for a git tracker', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    Object.assign(form.value, {
      name: 'g', tracker_type: 'git', repo_owner: 'a', repo_name: 'b',
      dispatch_task: false, registry_credentials_id: 'cred-1', update_action: 'compose',
    })
    submit()
    const p = submitted.payload!
    expect(p.registry_credentials_id).toBe('')
    expect(p.update_action).toBe('custom')
  })

  it('disables notify_on_release when no channel is selected (tracker)', () => {
    const { form, submit, submitted } = setup(makeProps({ mode: 'tracker' }))
    Object.assign(form.value, {
      name: 'g', tracker_type: 'git', repo_owner: 'a', repo_name: 'b',
      dispatch_task: false, notify_channels: [], notify_on_release: true,
    })
    submit()
    expect(submitted.payload!.notify_on_release).toBe(false)
  })
})

describe('useWebhookForm — computeds', () => {
  it('exposes docker env vars and the edit title for a docker tracker item', () => {
    const { currentEnvVars, title, submitLabel } = setup(makeProps({
      mode: 'tracker',
      item: { name: 'x', tracker_type: 'docker', docker_image: 'nginx' },
    }))
    expect(currentEnvVars.value.some(v => v.name === 'SS_IMAGE_NAME')).toBe(true)
    expect(title.value).toBe('Modifier le tracker')
    expect(submitLabel.value).toBe('Mettre a jour')
  })

  it('flags compose mode only when docker + dispatch + compose action align', () => {
    const { form, isComposeMode } = setup(makeProps({ mode: 'tracker' }))
    Object.assign(form.value, { tracker_type: 'docker', dispatch_task: true, update_action: 'compose' })
    expect(isComposeMode.value).toBe(true)
    form.value.update_action = 'custom'
    expect(isComposeMode.value).toBe(false)
  })
})

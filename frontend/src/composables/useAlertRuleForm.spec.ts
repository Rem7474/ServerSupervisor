import { describe, it, expect } from 'vitest'
import { useAlertRuleForm } from './useAlertRuleForm'

describe('useAlertRuleForm — docker container scope migration', () => {
  it('starts with an empty container_ids array by default', () => {
    const { form } = useAlertRuleForm()
    expect(form.value.docker_scope.container_ids).toEqual([])
  })

  it('transparently migrates a legacy single container_id into a one-element container_ids array', () => {
    const { form, hydrateFormFromRule } = useAlertRuleForm()

    hydrateFormFromRule({
      source_type: 'docker',
      metric: 'docker_container_state',
      docker_scope: { scope_mode: 'container', host_id: 'h1', container_id: 'c1' },
    })

    expect(form.value.docker_scope.container_id).toBe('c1')
    expect(form.value.docker_scope.container_ids).toEqual(['c1'])
  })

  it('prefers an already-populated container_ids array over the legacy field', () => {
    const { form, hydrateFormFromRule } = useAlertRuleForm()

    hydrateFormFromRule({
      source_type: 'docker',
      metric: 'docker_container_state',
      docker_scope: { scope_mode: 'container', host_id: 'h1', container_id: 'c1', container_ids: ['c1', 'c2'] },
    })

    expect(form.value.docker_scope.container_ids).toEqual(['c1', 'c2'])
  })

  it('yields an empty container_ids array when the rule has neither field set', () => {
    const { form, hydrateFormFromRule } = useAlertRuleForm()

    hydrateFormFromRule({
      source_type: 'docker',
      metric: 'docker_container_state',
      docker_scope: { scope_mode: 'host', host_id: 'h1' },
    })

    expect(form.value.docker_scope.container_ids).toEqual([])
  })
})

describe('useAlertRuleForm — chat channels', () => {
  it('hydrates slack/teams/discord from the rule and writes them back after the built-in channels', () => {
    const { channelChat, channelSmtp, hydrateFormFromRule, buildPayload } = useAlertRuleForm()

    hydrateFormFromRule({
      metric: 'cpu',
      actions: { channels: ['smtp', 'discord', 'slack', 'unknown'] },
    })

    expect(channelSmtp.value).toBe(true)
    expect(channelChat.value).toEqual(['slack', 'discord'])
    expect(buildPayload().actions.channels).toEqual(['smtp', 'slack', 'discord'])
  })

  it('clears the chat selection when the form is hydrated for a new rule', () => {
    const { channelChat, hydrateFormFromRule } = useAlertRuleForm()
    channelChat.value = ['teams']

    hydrateFormFromRule(null)

    expect(channelChat.value).toEqual([])
  })
})

describe('useAlertRuleForm — agent target', () => {
  it('defaults new rules to every host', () => {
    const { form, buildPayload } = useAlertRuleForm()
    expect(form.value.target_type).toBe('all')
    expect(buildPayload()).toMatchObject({ target_type: 'all', host_id: null, host_group_id: null, target_tags: [] })
  })

  it('derives the target of rules saved before targeting existed', () => {
    const { form, hydrateFormFromRule } = useAlertRuleForm()
    hydrateFormFromRule({ metric: 'cpu', host_id: 'h1' })
    expect(form.value.target_type).toBe('host')
    hydrateFormFromRule({ metric: 'cpu', host_id: null })
    expect(form.value.target_type).toBe('all')
    hydrateFormFromRule({ metric: 'cpu', target_type: 'bogus', host_id: 'h1' })
    expect(form.value.target_type).toBe('host')
  })

  it('hydrates an explicit tag target and sends only its tags', () => {
    const { buildPayload, hydrateFormFromRule } = useAlertRuleForm()
    hydrateFormFromRule({ metric: 'cpu', target_type: 'tag', target_tags: ['db'], host_id: 'h1', host_group_id: 'g1' })
    expect(buildPayload()).toMatchObject({ target_type: 'tag', target_tags: ['db'], host_id: null, host_group_id: null })
  })

  it('omits the agent target for non-agent rules', () => {
    const { buildPayload, hydrateFormFromRule } = useAlertRuleForm()
    hydrateFormFromRule({ metric: 'docker_container_state', source_type: 'docker', target_type: 'tag', target_tags: ['db'] })
    const payload = buildPayload()
    expect(payload.target_type).toBeUndefined()
    expect(payload.target_tags).toBeUndefined()
  })
})

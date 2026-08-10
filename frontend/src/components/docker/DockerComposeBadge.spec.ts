import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DockerComposeBadge from './DockerComposeBadge.vue'

describe('DockerComposeBadge', () => {
  it('shows project and service for a compose-managed container', () => {
    const wrapper = mount(DockerComposeBadge, {
      props: { labels: { 'com.docker.compose.project': 'nextcloud', 'com.docker.compose.service': 'db' } },
    })
    expect(wrapper.text()).toContain('nextcloud')
    expect(wrapper.text()).toContain('db')
  })

  it('hides the redundant service line when it just repeats the project name', () => {
    const wrapper = mount(DockerComposeBadge, {
      props: { labels: { 'com.docker.compose.project': 'nextcloud', 'com.docker.compose.service': 'Nextcloud' } },
    })
    expect(wrapper.text()).toContain('nextcloud')
    expect(wrapper.findAll('div.text-secondary')).toHaveLength(0)
  })

  it('shows a plain dash for a standalone container', () => {
    const wrapper = mount(DockerComposeBadge, { props: {} })
    expect(wrapper.text()).toBe('-')
  })
})

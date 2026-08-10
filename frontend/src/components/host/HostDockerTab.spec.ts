import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import HostDockerTab from './HostDockerTab.vue'

// Regression test: the host-detail Docker tab used to have no way to tell a
// compose-managed container from a standalone one, unlike the /docker global
// page (DockerContainersTab.vue) which already showed the compose project/
// service. Both now share DockerComposeBadge.vue.
describe('HostDockerTab — compose column parity with the /docker page', () => {
  it('shows the compose project/service for a compose-managed container', () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        containers: [
          {
            id: 'c1', name: 'nextcloud-app', image: 'nextcloud', image_tag: 'latest', state: 'running', status: 'Up 2 days',
            labels: { 'com.docker.compose.project': 'nextcloud', 'com.docker.compose.service': 'app' },
          },
        ],
      },
    })
    expect(wrapper.text()).toContain('nextcloud')
    expect(wrapper.text()).toContain('app')
  })

  it('shows a dash for a standalone container', () => {
    const wrapper = mount(HostDockerTab, {
      props: {
        hostId: 'h1',
        containers: [
          { id: 'c2', name: 'redis', image: 'redis', image_tag: 'alpine', state: 'running', status: 'Up 2 days' },
        ],
      },
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('-')
  })
})

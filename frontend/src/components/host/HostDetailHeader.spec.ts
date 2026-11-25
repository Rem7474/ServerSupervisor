import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import HostDetailHeader from './HostDetailHeader.vue'

beforeEach(() => {
  setLocale('fr')
})

function mountHeader(host: Record<string, unknown>) {
  return mount(HostDetailHeader, {
    props: { host, isAdmin: false, canUpdateAgent: false, agentUpdateLoading: false, isAgentUpToDate: () => true },
    global: { stubs: { RouterLink: RouterLinkStub, RelativeTime: true } },
  })
}

describe('HostDetailHeader — tags', () => {
  it('lists the host tags', () => {
    const wrapper = mountHeader({ id: 'h1', name: 'web-1', status: 'online', tags: ['prod', 'web'] })
    const list = wrapper.find('ul[aria-label="Tags de l\'hôte"]')
    expect(list.findAll('li').map(li => li.text())).toEqual(['prod', 'web'])
  })

  it('renders no tag list for an untagged host', () => {
    const wrapper = mountHeader({ id: 'h1', name: 'web-1', status: 'online', tags: [] })
    expect(wrapper.find('ul').exists()).toBe(false)
  })
})

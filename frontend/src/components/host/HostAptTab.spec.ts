import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import HostAptTab from './HostAptTab.vue'

// Regression test: the host-detail APT tab used to only show the pending
// package *count*, unlike the /apt page's AptHostCard which lists the actual
// packages — the two surfaces read the same apt_status row, so there was no
// reason for one to hide data the other shows. Both now share
// AptPendingPackagesList.vue.
describe('HostAptTab — pending packages list parity with the /apt page', () => {
  it('lists the pending packages by name, not just the count', () => {
    const wrapper = mount(HostAptTab, {
      props: {
        aptStatus: {
          pending_packages: 2,
          security_updates: 0,
          package_list: JSON.stringify(['curl', 'openssl']),
        },
      },
    })
    expect(wrapper.text()).toContain('Paquets en attente')
    expect(wrapper.text()).toContain('curl')
    expect(wrapper.text()).toContain('openssl')
  })

  it('renders no package grid when package_list is absent (only the KPI count tile)', () => {
    const wrapper = mount(HostAptTab, {
      props: { aptStatus: { pending_packages: 0, security_updates: 0 } },
    })
    expect(wrapper.find('.apt-package-item').exists()).toBe(false)
  })
})

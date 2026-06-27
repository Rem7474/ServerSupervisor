import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import EmptyState from './EmptyState.vue'

// Regression: EmptyState's `icon` prop defaulted to a Tabler icon (a function).
// Vue resolves a function-typed default by CALLING it as a factory
// (resolvePropValue), invoking the icon with no setup context → it throws
// `Cannot destructure property 'attrs' of 'undefined'`, which then cascades into
// an unmount crash (`Cannot destructure property 'bum' of 'e'`) for the whole
// page. This guards the default-icon path that the Docker page hits whenever its
// container list is empty (e.g. before the first WS snapshot arrives).
describe('EmptyState', () => {
  it('mounts with the default icon (no icon prop, no slot) without throwing', () => {
    const wrapper = mount(EmptyState, { props: { title: 'Aucun élément' } })
    expect(wrapper.text()).toContain('Aucun élément')
    expect(wrapper.find('svg').exists()).toBe(true)
  })

  it('mounts with an overriding icon slot but no icon prop', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 'Vide' },
      slots: { icon: '<i class="custom-icon" />' },
    })
    expect(wrapper.find('.custom-icon').exists()).toBe(true)
  })
})

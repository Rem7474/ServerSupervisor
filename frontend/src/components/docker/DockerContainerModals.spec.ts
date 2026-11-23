import { describe, it, expect, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { Component } from 'vue'
import { setLocale } from '../../i18n'
import DockerContainerComposeModal from './DockerContainerComposeModal.vue'
import DockerContainerInspectModal from './DockerContainerInspectModal.vue'

const container = {
  id: 'c1',
  name: 'web',
  hostname: 'host-1',
  host_id: 'h1',
  image: 'nginx',
  image_tag: 'latest',
  state: 'running',
  labels: {},
  env_vars: {},
  volumes: [],
  networks: [],
  ports: '80/tcp',
}

afterEach(() => {
  document.body.innerHTML = ''
})

// The modals are created already open. Their focus handling is armed from
// onMounted: armed during setup(), the modal element would not exist yet and
// neither autofocus nor the Tab trap would attach.
const cases: Array<[string, Component, Record<string, unknown>]> = [
  ['compose details', DockerContainerComposeModal, { container }],
  ['inspect', DockerContainerInspectModal, { container, version: null }],
]

describe.each(cases)('Docker %s modal focus handling', (_name, component, props) => {
  it('moves focus into the dialog on open', async () => {
    setLocale('fr')
    const wrapper = mount(component as Component, { props, attachTo: document.body })
    await flushPromises()

    expect(wrapper.get('.modal').element.contains(document.activeElement)).toBe(true)
    wrapper.unmount()
  })

  it('traps Tab inside the dialog, wrapping from the last focusable to the first', async () => {
    setLocale('fr')
    const wrapper = mount(component as Component, { props, attachTo: document.body })
    await flushPromises()

    const dialog = wrapper.get('.modal').element
    const focusables = Array.from(
      dialog.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'),
    )
    const last = focusables[focusables.length - 1]
    last.focus()
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))

    expect(document.activeElement).toBe(focusables[0])
    wrapper.unmount()
  })

  it('emits close on Escape', async () => {
    setLocale('fr')
    const wrapper = mount(component as Component, { props, attachTo: document.body })
    await flushPromises()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
})

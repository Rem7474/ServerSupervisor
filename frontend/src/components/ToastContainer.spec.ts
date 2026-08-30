import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ToastContainer from './ToastContainer.vue'
import { addToast, removeToast, useGlobalToast } from '../composables/useGlobalToast'

describe('ToastContainer', () => {
  afterEach(() => {
    // useGlobalToast holds module-level (singleton) state shared across the
    // whole app — clear it between tests so one test's toasts don't leak
    // into the next.
    const { toasts } = useGlobalToast()
    for (const t of [...toasts]) removeToast(t.id)
  })

  it('renders one toast per type with its own accent class and icon, and dismisses it on close', async () => {
    const wrapper = mount(ToastContainer, { attachTo: document.body })
    addToast('Saved successfully', 'success', 0)
    await nextTick()

    const toastEl = document.querySelector('.ss-toast')
    expect(toastEl).toBeTruthy()
    expect(toastEl?.classList.contains('toast')).toBe(true)
    expect(toastEl?.classList.contains('show')).toBe(true)
    expect(toastEl?.classList.contains('ss-toast--success')).toBe(true)
    expect(toastEl?.getAttribute('aria-atomic')).toBe('true')
    expect(document.body.textContent).toContain('Saved successfully')

    const closeButton = document.querySelector('.ss-toast-close') as HTMLButtonElement
    closeButton.click()
    await nextTick()

    expect(document.querySelector('.ss-toast')).toBeNull()
    wrapper.unmount()
  })

  it('picks the correct icon/accent class per toast type', async () => {
    const wrapper = mount(ToastContainer, { attachTo: document.body })
    addToast('warn message', 'warning', 0)
    addToast('err message', 'error', 0)
    addToast('info message', 'info', 0)
    await nextTick()

    const toastEls = document.querySelectorAll('.ss-toast')
    expect(toastEls).toHaveLength(3)
    expect(toastEls[0].classList.contains('ss-toast--warning')).toBe(true)
    expect(toastEls[1].classList.contains('ss-toast--error')).toBe(true)
    expect(toastEls[2].classList.contains('ss-toast--info')).toBe(true)
    wrapper.unmount()
  })
})

import { describe, it, expect, vi, beforeAll } from 'vitest'

// Bootstrap module — importing it runs createApp(...).mount('#app') plus a
// handful of window-level error/rejection listeners as a side effect. The
// app shell (router + App.vue) is stubbed out to a no-op so this test
// exercises only main.ts's own logic (the benign-error/rejection filters +
// the fatal-fallback renderer), not the full app boot sequence.
vi.mock('./router', () => ({
  default: { install: () => {} },
}))
vi.mock('./App.vue', () => ({
  default: { name: 'AppStub', render: () => null },
}))

describe('main.ts — window error/rejection handling', () => {
  beforeAll(async () => {
    document.body.innerHTML = '<div id="app"></div>'
    await import('./main')
  })

  it('treats a ResizeObserver loop error as benign: logs it, does not render the fatal fallback', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    window.dispatchEvent(new ErrorEvent('error', {
      message: 'ResizeObserver loop completed with undelivered notifications.',
    }))

    expect(warnSpy).toHaveBeenCalledWith(
      '[app] Erreur ignorée (bénigne, ResizeObserver):',
      expect.anything(),
    )
    expect(document.querySelector('.alert-danger')).toBeNull()
    warnSpy.mockRestore()
  })

  it('renders the fatal fallback UI for a genuine unhandled window error', () => {
    window.dispatchEvent(new ErrorEvent('error', {
      message: 'TypeError: something is not a function',
    }))

    const alert = document.querySelector('.alert-danger')
    expect(alert).not.toBeNull()
    expect(document.body.textContent).toContain('Erreur JavaScript non gérée')
  })
})

// Global test setup. Stubs browser APIs that components touch on mount but that
// happy-dom does not implement, so component tests can mount without throwing.
import { vi } from 'vitest'

// Notification API (used by useNotifications and alert previews).
if (typeof globalThis.Notification === 'undefined') {
  // @ts-expect-error minimal stub
  globalThis.Notification = class {
    static permission = 'default'
    static requestPermission = vi.fn(async () => 'default')
    onclick: (() => void) | null = null
    close = vi.fn()
    constructor(_title: string, _opts?: unknown) {}
  }
}

// matchMedia (some Tabler/responsive helpers reference it).
if (typeof window !== 'undefined' && !window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

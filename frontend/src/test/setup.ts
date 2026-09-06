// Global test setup. Stubs browser APIs that components touch on mount but that
// happy-dom does not implement, so component tests can mount without throwing.
import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import { i18n } from '../i18n'
import en from '../locales/en'

// Every component under test may call useI18n(); installing the plugin once
// here (rather than per-spec `global: { plugins: [i18n] }`) keeps existing
// mount() calls working unchanged. Tests that care about a specific locale
// should call setLocale(...) explicitly — this does not reset it between
// tests, matching the singleton's real runtime behavior.
config.global.plugins.push(i18n)

// Only the fallback locale ships eagerly at runtime (see locales/index.ts).
// Specs call setLocale() synchronously, so every locale is registered up front
// here instead — a test asserting English output must not have to await a chunk.
i18n.global.setLocaleMessage('en', en as never)

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

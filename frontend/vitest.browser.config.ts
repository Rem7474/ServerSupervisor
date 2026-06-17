import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { playwright } from '@vitest/browser-playwright'

// Real-browser test config (Playwright/Chromium). Used for components whose
// behaviour depends on actual rendering — canvas charts (Chart.js) and the
// D3/SVG world map — which happy-dom cannot execute. Run via `npm run test:browser`.
// Files: src/**/*.browser.test.ts
//
// KNOWN ISSUE (CI job is non-blocking): on Vite 8 (rolldown dep-optimizer) +
// Vitest 4 browser mode, @vue/test-utils is force-bundled into .vite/deps while
// the compiled SFCs import the raw esm-bundler Vue → two Vue instances (template
// refs never bind, "Missing ref owner context" / "resolveComponent can only be
// used in render()" warnings). Bundling test-utils *with* vue instead trips a
// rolldown chunk-split bug ("init_shared_esm_bundler is not defined"). Re-enable
// blocking once the upstream optimizer bug is fixed (or Vite pins back to esbuild).
export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify('test'),
  },
  test: {
    globals: true,
    include: ['src/**/*.browser.test.ts'],
    browser: {
      enabled: true,
      provider: playwright(),
      headless: true,
      instances: [{ browser: 'chromium' }],
    },
  },
})
